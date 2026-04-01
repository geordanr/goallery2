package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestFileRouteMatches verifies that multi-segment paths under /files/ are
// matched by the route and reach the handler (not chi's default 404), and
// that the wildcard param contains the full sub-path.
func TestFileRouteMatches(t *testing.T) {
	tests := []struct {
		url       string
		wantParam string
	}{
		{"/files/foo.jpg", "foo.jpg"},
		{"/files/a/b.jpg", "a/b.jpg"},
		{"/files/albums/geordan/ax2004/124_2447_IMG.jpg", "albums/geordan/ax2004/124_2447_IMG.jpg"},
	}

	r := chi.NewRouter()
	var gotParam string
	r.Get("/files/*", func(w http.ResponseWriter, req *http.Request) {
		gotParam = chi.URLParam(req, "*")
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range tests {
		gotParam = ""
		req := httptest.NewRequest(http.MethodGet, tc.url, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s → %d, want 200", tc.url, rec.Code)
			continue
		}
		if gotParam != tc.wantParam {
			t.Errorf("GET %s: wildcard param = %q, want %q", tc.url, gotParam, tc.wantParam)
		}
	}
}
