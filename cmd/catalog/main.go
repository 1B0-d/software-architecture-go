package main

import (
	"example.local/architecture-demo/internal/catalog"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	dbpath := os.Getenv("CATALOG_DB")
	if dbpath == "" {
		dbpath = "catalog.db"
	}
	addr := os.Getenv("CATALOG_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	r, e := catalog.Open(dbpath)
	if e != nil {
		log.Fatal(e)
	}
	defer r.Close()
	s := &http.Server{Addr: addr, Handler: catalog.Handler(r), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("catalog %s", addr)
	log.Fatal(s.ListenAndServe())
}
