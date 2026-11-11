package main

import (
	"example.local/architecture-demo/internal/reviews"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	path := os.Getenv("REVIEWS_DB")
	if path == "" {
		path = "reviews.db"
	}
	addr := os.Getenv("REVIEWS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8082"
	}
	catalog := os.Getenv("CATALOG_URL")
	if catalog == "" {
		catalog = "http://127.0.0.1:8081"
	}
	r, e := reviews.Open(path)
	if e != nil {
		log.Fatal(e)
	}
	defer r.Close()
	s := &http.Server{Addr: addr, Handler: reviews.Handler(r, reviews.HTTPVehicleChecker{BaseURL: catalog, Client: &http.Client{Timeout: 3 * time.Second}}), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("reviews %s", addr)
	log.Fatal(s.ListenAndServe())
}
