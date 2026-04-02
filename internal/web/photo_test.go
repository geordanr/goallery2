package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/gallery"
)

// fakeReader implements gallery.Reader for handler tests. Only methods exercised
// by the photo handler are wired up; all others panic to make accidental calls obvious.
type fakeReader struct {
	photo   *gallery.Photo
	album   *gallery.Album
	resized []gallery.Derivative
}

func (f *fakeReader) GetRootAlbum() (*gallery.Album, error)           { return f.album, nil }
func (f *fakeReader) GetAlbum(_ int) (*gallery.Album, error)          { return f.album, nil }
func (f *fakeReader) GetPhoto(_ int) (*gallery.Photo, error)          { return f.photo, nil }
func (f *fakeReader) GetMovie(_ int) (*gallery.Movie, error)          { panic("not implemented") }
func (f *fakeReader) ChildAlbums(_ int) ([]gallery.Album, error)      { panic("not implemented") }
func (f *fakeReader) AlbumPhotos(_ int) ([]gallery.Photo, error)      { panic("not implemented") }
func (f *fakeReader) AlbumMovies(_ int) ([]gallery.Movie, error)      { panic("not implemented") }
func (f *fakeReader) Derivatives(_ int) ([]gallery.Derivative, error) { return f.resized, nil }
func (f *fakeReader) Thumbnail(_ int) (*gallery.Derivative, error)    { panic("not implemented") }
func (f *fakeReader) ItemPath(_ int) (string, error)                  { return "albums/test/photo.jpg", nil }

// TestPhotoHandler_ResizedMissingFromDisk verifies that when a photo has a
// resized derivative in the DB but the corresponding cache file does not exist
// on disk, the photo page falls back to the full-size image rather than
// rendering a broken image link.
//
// This reproduces the production condition where Gallery 2 writes a derivative
// record to the DB but never generates the actual cache file (e.g. ID 26445,
// where 26444.dat and 26446.dat exist but 26445.dat does not).
func TestPhotoHandler_ResizedMissingFromDisk(t *testing.T) {
	// Set up a temp data dir. The resized derivative cache file is deliberately
	// NOT created — only the shard directories are.
	dataDir := t.TempDir()

	// Derivative ID 26445 → CachePath "cache/derivative/2/6/26445.dat".
	// Create the shard dirs but not the file.
	if err := os.MkdirAll(filepath.Join(dataDir, "cache", "derivative", "2", "6"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create the full-size photo file so ItemPath resolves to something real.
	if err := os.MkdirAll(filepath.Join(dataDir, "albums", "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "albums", "test", "photo.jpg"), []byte("fake jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	reader := &fakeReader{}
	reader.album = gallery.NewAlbum(reader, gallery.AlbumFields{
		ID: 7, Title: "Root",
	})
	reader.photo = gallery.NewPhoto(reader, gallery.PhotoFields{
		ID: 100, ParentID: 7, Title: "Test Photo", PathComponent: "photo.jpg", MimeType: "image/jpeg",
	})
	reader.resized = []gallery.Derivative{
		{ID: 26445, SourceID: 100, Type: gallery.DerivativeResized, MimeType: "image/jpeg"},
	}

	h := &handler{
		store:      reader,
		config:     config.Config{Server: config.ServerConfig{DataDir: dataDir}},
		absDataDir: dataDir,
	}

	r := chi.NewRouter()
	r.Get("/photo/{id}", h.photo)

	req := httptest.NewRequest(http.MethodGet, "/photo/100", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /photo/100 = %d, want 200", rec.Code)
	}

	body := rec.Body.String()

	// Must NOT link to the missing derivative file.
	if strings.Contains(body, "cache/derivative/2/6/26445.dat") {
		t.Errorf("response contains link to missing derivative file; expected fallback to full-size image\nbody:\n%s", body)
	}

	// Must show the full-size image instead.
	if !strings.Contains(body, "albums/test/photo.jpg") {
		t.Errorf("response does not contain full-size image path; body:\n%s", body)
	}
}
