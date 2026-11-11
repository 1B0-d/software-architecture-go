package shared

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func JSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Middleware logs method, path, and duration; it does not log request bodies.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() { log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start)) }()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
