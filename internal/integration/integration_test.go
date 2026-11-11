package integration

import (
	"bytes"
	"encoding/json"
	"example.local/architecture-demo/internal/catalog"
	"example.local/architecture-demo/internal/reviews"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestServicesPersistAndCommunicate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.db")
	cat, err := catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cs := httptest.NewServer(catalog.Handler(cat))
	defer cs.Close()
	res, err := http.Post(cs.URL+"/vehicles", "application/json", bytes.NewBufferString(`{"name":"Sample car"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 {
		t.Fatalf("create: %d", res.StatusCode)
	}
	var vehicle catalog.Vehicle
	if err = json.NewDecoder(res.Body).Decode(&vehicle); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	repo, err := reviews.Open(filepath.Join(dir, "reviews.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	rs := httptest.NewServer(reviews.Handler(repo, reviews.HTTPVehicleChecker{BaseURL: cs.URL, Client: cs.Client()}))
	defer rs.Close()
	res, err = http.Post(rs.URL+"/reviews", "application/json", bytes.NewBufferString(`{"vehicle_id":1,"text":"Good"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 {
		t.Fatalf("review: %d", res.StatusCode)
	}
	res.Body.Close()
	res, err = http.Post(rs.URL+"/reviews", "application/json", bytes.NewBufferString(`{"vehicle_id":9999,"text":"No car"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 404 {
		t.Fatalf("missing vehicle: %d", res.StatusCode)
	}
	res.Body.Close()
	res, err = http.Get(rs.URL + "/reviews?vehicle_id=1")
	if err != nil {
		t.Fatal(err)
	}
	var got []reviews.Review
	if err = json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(got) != 1 || got[0].Text != "Good" {
		t.Fatalf("reviews: %+v", got)
	}
	cs.Close()
	if err = cat.Close(); err != nil {
		t.Fatal(err)
	}
	cat, err = catalog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	v, err := cat.Get(t.Context(), vehicle.ID)
	if err != nil || v.Name != "Sample car" {
		t.Fatalf("reopen: %+v %v", v, err)
	}
}
func TestInvalidInputAndCatalogFailure(t *testing.T) {
	repo, err := reviews.Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	server := httptest.NewServer(reviews.Handler(repo, reviews.HTTPVehicleChecker{BaseURL: "http://127.0.0.1:1"}))
	defer server.Close()
	for _, tc := range []struct {
		body string
		want int
	}{{`{"vehicle_id":0,"text":"x"}`, 400}, {`{"vehicle_id":1,"text":"x"}`, 502}, {`not json`, 400}} {
		res, err := http.Post(server.URL+"/reviews", "application/json", bytes.NewBufferString(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.want {
			t.Fatalf("%s got %d want %d", tc.body, res.StatusCode, tc.want)
		}
	}
}
