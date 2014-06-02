package resized

import (
  "net/http"
  "net/url"
  "github.com/gographics/imagick/imagick"
  "log"
  "strings"
  "strconv"
  "mime"
  "io/ioutil"
)

const (
  Assets = "/tmp"
  Base = "/images/catalog/"
)

const (
  PathComponentsProductMax = 4
  PathComponentsCategoryMax = 2
  QualityIndex = 5
  ResolutionIndex = 4
)

type HandlerFunc func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc)

func getFilePathResQuality(url string) (path string, width, height, quality int) {
  var res []string
  var resq []string
  path = strings.TrimPrefix(url,Base)
  fields := strings.Split(path,"/")
  length := len(fields)

  if fields[0] == "category"  {
    path = Base +  strings.Join(fields[:PathComponentsCategoryMax],"/") + "/" + fields[length-1];
    resq = fields[PathComponentsCategoryMax:length-1]
  } else {
    path = Base + strings.Join(fields[:PathComponentsProductMax],"/") + "/" + fields[length-1]
    resq = fields[PathComponentsProductMax:length-1]
  }

  // defaults
  quality = 70
  width = 0
  height = 0

  switch (len(resq)) {
    case 2:
      quality,_ = strconv.Atoi(resq[1])
      res = strings.Split(resq[0],"x")
    case 1:
      res = strings.Split(resq[0],"x")
    default:
  }

  if (res != nil) {
    width,_ = strconv.Atoi(res[0])
    height,_ = strconv.Atoi(res[1])
  }
  return
}

func downstreamHandler(ds Downstream,ch chan DSData) {
  log.Println("Initializing downstream handler")
  ds.Init()
  for {
    data, ok := <-ch
    if ok == false {
      break
    }
    log.Println("received request for " + data.path)
    ds.Put(data)
  }
}

func Resizer(cacheDir string,ups string) (HandlerFunc) {

  var server Upstream
  var ds Downstream

  chD := make(chan DSData)

  imagick.Initialize()

  url,err := url.Parse(ups)
  if err != nil {
    log.Panic("Bad URL scheme")
  }

  if cacheDir != "" {
    ds = &S3Downstream{ downstreamURI: cacheDir }
    go downstreamHandler(ds,chD)
  }

  switch url.Scheme {
    case "http":
      server = &HTTPUpstream{ upstreamURI: ups}
      log.Println("Serving using " + ups)
    case "file":
      server = &FileUpstream{ upstreamURI: url.Path }
      log.Println("Serving using " + url.Path)
    default:
      log.Panic("Unsupported url scheme " + url.Scheme)
  }

  return func(w http.ResponseWriter, r* http.Request, next http.HandlerFunc) {

    if (strings.HasPrefix(r.URL.Path,"/images/catalog/") == false) {
      log.Println("skipping ",r.URL.Path)
      next(w,r);
      return
    }

    filePath,width,height,quality := getFilePathResQuality(r.URL.Path)

    if (width == 0 && height == 0) {
      log.Println("skipping resize for ",filePath)
      server.ServeOriginal(w,r,filePath)
      return
    }


    file,err := server.Get(w,r,filePath)

    if file != nil {
      defer file.Close() // in case of 404, file still needs to be closed.
    }

    if err != nil {
      log.Println("upstream error with ",r.URL.Path)
      http.Error(w, "File not found", http.StatusNotFound)
      return
    }

    body, err := ioutil.ReadAll(file)

    if err != nil {
      log.Println("Failed to read image ", r.URL.Path)
      http.Error(w, err.Error(), http.StatusNotFound)
      return
    }

    bytes,err := Resize(uint(width), uint(height), uint(quality), body)
    if err != nil {
      log.Println("Failed to resize image ", r.URL.Path)
      http.Error(w, err.Error(), http.StatusNotFound)
      return
    }

    mimeType := mime.TypeByExtension(filePath[strings.LastIndex(filePath,"."):])

    w.Header().Set("Content-Type", mimeType)
    w.Header().Set("Content-Length", strconv.FormatUint(uint64(len(bytes)), 10))
    w.WriteHeader(http.StatusOK)

    if r.Method != "HEAD" {
      w.Write(bytes)
    }

    // cache the result
    if (cacheDir != "") {
      log.Println("sending request to downstream for caching " + r.URL.Path)
      chD <- DSData{data: &bytes, path: r.URL.Path, mimeType: mimeType}
    }
  }
}