package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/gallery"
)

// makeAlbumHandler builds a handler with a fakeReader pre-loaded with the given
// number of photos (IDs 1..n) in a root album.
func makeAlbumHandler(t *testing.T, photoCount int) (*handler, *fakeReader) {
	t.Helper()
	reader := &fakeReader{}
	reader.album = gallery.NewAlbum(reader, gallery.AlbumFields{ID: 7, Title: "Root"})

	photos := make([]gallery.Photo, photoCount)
	for i := range photos {
		photos[i] = *gallery.NewPhoto(reader, gallery.PhotoFields{
			ID:            i + 1,
			ParentID:      7,
			PathComponent: fmt.Sprintf("photo%d.jpg", i+1),
			MimeType:      "image/jpeg",
		})
	}
	reader.photos = photos

	dataDir := t.TempDir()
	h := &handler{
		store:      reader,
		config:     config.Config{Server: config.ServerConfig{DataDir: dataDir}},
		absDataDir: dataDir,
	}
	return h, reader
}

func TestAlbumPagination(t *testing.T) {
	tests := []struct {
		name         string
		photoCount   int
		query        string
		wantPage     string // substring expected in body
		wantNoPrev   bool
		wantNoNext   bool
		wantNoPagNav bool // true when only one page exists
	}{
		{
			name:         "zero photos — no pagination nav",
			photoCount:   0,
			query:        "",
			wantNoPagNav: true,
		},
		{
			name:         "fewer than page size — no pagination nav",
			photoCount:   photosPerPage - 1,
			query:        "",
			wantNoPagNav: true,
		},
		{
			name:         "exactly one page — no pagination nav",
			photoCount:   photosPerPage,
			query:        "",
			wantNoPagNav: true,
		},
		{
			name:       "two pages, first page — no prev, has next",
			photoCount: photosPerPage + 1,
			query:      "",
			wantPage:   "Page 1 of 2",
			wantNoPrev: true,
		},
		{
			name:       "two pages, second page — has prev, no next",
			photoCount: photosPerPage + 1,
			query:      "?page=2",
			wantPage:   "Page 2 of 2",
			wantNoNext: true,
		},
		{
			name:       "page beyond range clamps to 1",
			photoCount: photosPerPage + 1,
			query:      "?page=999",
			wantPage:   "Page 1 of 2",
			wantNoPrev: true,
		},
		{
			name:       "non-numeric page clamps to 1",
			photoCount: photosPerPage + 1,
			query:      "?page=abc",
			wantPage:   "Page 1 of 2",
			wantNoPrev: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := makeAlbumHandler(t, tc.photoCount)

			r := chi.NewRouter()
			r.Get("/album/{id}", h.album)

			req := httptest.NewRequest(http.MethodGet, "/album/7"+tc.query, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("GET /album/7%s = %d, want 200", tc.query, rec.Code)
			}

			body := rec.Body.String()

			if tc.wantNoPagNav {
				if strings.Contains(body, `<nav class="pagination">`) {
					t.Errorf("expected no pagination nav element, but found one; body:\n%s", body)
				}
				return
			}

			if tc.wantPage != "" && !strings.Contains(body, tc.wantPage) {
				t.Errorf("expected %q in body; body:\n%s", tc.wantPage, body)
			}
			// The template renders an <a> for active nav and a <span> for inactive.
			// Assert that inactive directions have no anchor element.
			if tc.wantNoPrev && strings.Contains(body, `>&larr; Prev</a>`) {
				t.Errorf("found unexpected prev anchor; body:\n%s", body)
			}
			if tc.wantNoNext && strings.Contains(body, `>Next &rarr;</a>`) {
				t.Errorf("found unexpected next anchor; body:\n%s", body)
			}
		})
	}
}
