package web

import (
	"database/sql"
	"fmt"
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

// errReader is a fakeReader that returns sql.ErrNoRows for GetAlbum and GetPhoto
// so that 404 paths can be tested.
type errReader struct {
	fakeReader
	albumErr error
	photoErr error
}

func (e *errReader) GetAlbum(_ int) (*gallery.Album, error) { return nil, e.albumErr }
func (e *errReader) GetPhoto(_ int) (*gallery.Photo, error) { return nil, e.photoErr }

// ── index ─────────────────────────────────────────────────────────────────────

func TestIndexRedirect(t *testing.T) {
	h := &handler{
		store:      &fakeReader{},
		config:     config.Config{},
		absDataDir: t.TempDir(),
	}

	r := chi.NewRouter()
	r.Get("/", h.index)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("GET / = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/album/7" {
		t.Errorf("Location = %q, want %q", loc, "/album/7")
	}
}

// ── album handler ─────────────────────────────────────────────────────────────

func TestAlbumHandler(t *testing.T) {
	reader := &fakeReader{}
	reader.album = gallery.NewAlbum(reader, gallery.AlbumFields{ID: 7, Title: "My Gallery"})

	h := &handler{
		store:      reader,
		config:     config.Config{},
		absDataDir: t.TempDir(),
	}

	r := chi.NewRouter()
	r.Get("/album/{id}", h.album)

	req := httptest.NewRequest(http.MethodGet, "/album/7", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /album/7 = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "My Gallery") {
		t.Errorf("response does not contain album title %q", "My Gallery")
	}
}

func TestAlbumNotFound(t *testing.T) {
	// The album handler returns 404 for any GetAlbum error (not just ErrNoRows).
	er := &errReader{albumErr: fmt.Errorf("album does not exist")}

	h := &handler{
		store:      er,
		config:     config.Config{},
		absDataDir: t.TempDir(),
	}

	r := chi.NewRouter()
	r.Get("/album/{id}", h.album)

	req := httptest.NewRequest(http.MethodGet, "/album/9999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /album/9999 = %d, want 404", rec.Code)
	}
}

// ── photo handler ─────────────────────────────────────────────────────────────

func TestPhotoNotFound(t *testing.T) {
	er := &errReader{photoErr: fmt.Errorf("wrapped: %w", sql.ErrNoRows)}
	// er.album must be set: the photo handler calls GetAlbum (via photoBreadcrumbs)
	// after GetPhoto fails only when the error is not ErrNoRows, so this path
	// returns 404 before breadcrumbs are built. But GetAlbum is still called by
	// fakeReader.GetAlbum which returns er.album — set it to a non-nil value so
	// any accidental call doesn't panic.
	er.album = gallery.NewAlbum(&er.fakeReader, gallery.AlbumFields{ID: 7})

	h := &handler{
		store:      er,
		config:     config.Config{},
		absDataDir: t.TempDir(),
	}

	r := chi.NewRouter()
	r.Get("/photo/{id}", h.photo)

	req := httptest.NewRequest(http.MethodGet, "/photo/9999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /photo/9999 = %d, want 404", rec.Code)
	}
}

// ── file serving ──────────────────────────────────────────────────────────────

func newFileHandler(t *testing.T) (*handler, string) {
	t.Helper()
	dataDir := t.TempDir()
	h := &handler{
		store:      &fakeReader{},
		config:     config.Config{Server: config.ServerConfig{DataDir: dataDir}},
		absDataDir: dataDir,
	}
	return h, dataDir
}

func TestServeFile_OK(t *testing.T) {
	h, dataDir := newFileHandler(t)

	if err := os.MkdirAll(filepath.Join(dataDir, "albums", "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "albums", "test", "photo.jpg"), []byte("jpeg data"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Get("/files/*", h.serveFile)

	req := httptest.NewRequest(http.MethodGet, "/files/albums/test/photo.jpg", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /files/albums/test/photo.jpg = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "jpeg data" {
		t.Errorf("body = %q, want %q", body, "jpeg data")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/jpeg") {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
}

func TestServeFile_Traversal(t *testing.T) {
	h, _ := newFileHandler(t)

	// Write a file in a sibling temp dir (entirely under test control) to prove
	// it cannot be reached via path traversal from the data dir.
	siblingDir := t.TempDir()
	secret := filepath.Join(siblingDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Get("/files/*", h.serveFile)

	req := httptest.NewRequest(http.MethodGet, "/files/../secret.txt", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /files/../secret.txt = %d, want 403", rec.Code)
	}
}

func TestServeFile_NotFound(t *testing.T) {
	h, _ := newFileHandler(t)

	r := chi.NewRouter()
	r.Get("/files/*", h.serveFile)

	req := httptest.NewRequest(http.MethodGet, "/files/albums/nonexistent.jpg", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /files/albums/nonexistent.jpg = %d, want 404", rec.Code)
	}
}

func TestServeFile_Directory(t *testing.T) {
	h, dataDir := newFileHandler(t)

	if err := os.MkdirAll(filepath.Join(dataDir, "albums", "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Get("/files/*", h.serveFile)

	req := httptest.NewRequest(http.MethodGet, "/files/albums/subdir", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /files/albums/subdir = %d, want 404", rec.Code)
	}
}
