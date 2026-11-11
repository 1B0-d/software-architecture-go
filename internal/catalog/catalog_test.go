package catalog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTMLViewEscapesVehicleNames(t *testing.T) {
	repo, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	_, err = repo.Create(t.Context(), "<script>alert(1)</script>")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handler(repo).ServeHTTP(w, r)
	b, _ := io.ReadAll(w.Result().Body)
	if w.Code != 200 || strings.Contains(string(b), "<script>") || !strings.Contains(string(b), "&lt;script&gt;") {
		t.Fatalf("view failed to escape output: status %d body %q", w.Code, b)
	}
}
