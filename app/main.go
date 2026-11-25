package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

const headerReadTimeout = 10 * time.Second

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{Addr: addr, Handler: newServer(), ReadHeaderTimeout: headerReadTimeout}
	log.Printf("listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
