package main

import (
  "github.com/codegangsta/negroni"
  "net/http"
  "fmt"
  "router"
)

const (
  CacheDir = "./public"
)

func main() {
  mux := http.NewServeMux()
  mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
    fmt.Fprintf(w, "Welcome to Paytm.")
  })

  n := negroni.Classic()
  n.Use(negroni.HandlerFunc(router.Resizer("")))
  n.UseHandler(mux)
  n.Run(":3000")
}
