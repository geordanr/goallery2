package uploader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geordanr/goallery2/internal/gallery"
)

// fakeFlickr records calls made by Uploader and returns canned responses.
type fakeFlickr struct {
	uploadedPaths []string
	createdSets   []string // titles
	addedPhotos   []string // "photosetID:photoID"
	uploadIDSeq   int
	photosetIDSeq int
	uploadErr     error
	createSetErr  error
	addPhotoErr   error
}

func (f *fakeFlickr) UploadPhoto(diskPath, _, _, _ string) (string, error) {
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	f.uploadedPaths = append(f.uploadedPaths, diskPath)
	f.uploadIDSeq++
	return fmt.Sprintf("fid%d", f.uploadIDSeq), nil
}

func (f *fakeFlickr) SetDateTaken(_ string, _ time.Time) error { return nil }

func (f *fakeFlickr) CreatePhotoset(title, _ string) (string, error) {
	if f.createSetErr != nil {
		return "", f.createSetErr
	}
	f.createdSets = append(f.createdSets, title)
	f.photosetIDSeq++
	return fmt.Sprintf("ps%d", f.photosetIDSeq), nil
}

func (f *fakeFlickr) AddPhotoToPhotoset(photosetID, photoID string) error {
	if f.addPhotoErr != nil {
		return f.addPhotoErr
	}
	f.addedPhotos = append(f.addedPhotos, photosetID+":"+photoID)
	return nil
}

// makeDataDir creates fake photo files on disk so photo.Path() resolves.
func makeDataDir(t *testing.T, paths []string) string {
	t.Helper()
	dataDir := t.TempDir()
	for _, p := range paths {
		full := filepath.Join(dataDir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dataDir
}

// photoWithPath returns a Photo whose ItemPath resolves to relPath.
type pathReader struct{ path string }

func (p *pathReader) GetRootAlbum() (*gallery.Album, error)  { panic("not implemented") }
func (p *pathReader) GetAlbum(_ int) (*gallery.Album, error) { panic("not implemented") }
func (p *pathReader) GetPhoto(_ int) (*gallery.Photo, error) { panic("not implemented") }
func (p *pathReader) GetMovie(_ int) (*gallery.Movie, error) { panic("not implemented") }
func (p *pathReader) ChildAlbums(_ int, _ gallery.SortOrder) ([]gallery.Album, error) {
	panic("not implemented")
}
func (p *pathReader) AlbumPhotos(_ int, _ gallery.SortOrder) ([]gallery.Photo, error) {
	panic("not implemented")
}
func (p *pathReader) AlbumMovies(_ int, _ gallery.SortOrder) ([]gallery.Movie, error) {
	panic("not implemented")
}
func (p *pathReader) Derivatives(_ int) ([]gallery.Derivative, error) { panic("not implemented") }
func (p *pathReader) Thumbnail(_ int) (*gallery.Derivative, error)    { panic("not implemented") }
func (p *pathReader) ItemPath(_ int) (string, error)                  { return p.path, nil }

func makePhoto(id int, relPath string) gallery.Photo {
	r := &pathReader{path: relPath}
	return *gallery.NewPhoto(r, gallery.PhotoFields{ID: id, ParentID: 1, PathComponent: filepath.Base(relPath)})
}

func makeAlbumUpload(albumID int, title string, photos []gallery.Photo) AlbumUpload {
	return AlbumUpload{
		FlickrTitle: title,
		SourceAlbum: *gallery.NewAlbum(&pathReader{}, gallery.AlbumFields{ID: albumID, Title: title}),
		Photos:      photos,
	}
}

// ── dry-run ───────────────────────────────────────────────────────────────────

func TestRun_DryRun_NoUploads(t *testing.T) {
	fc := &fakeFlickr{}
	u := New(fc, newState(), filepath.Join(t.TempDir(), "state.json"), t.TempDir(), true)

	uploads := []AlbumUpload{
		makeAlbumUpload(1, "Vacation", []gallery.Photo{makePhoto(101, "albums/vac/a.jpg")}),
	}
	if err := u.Run(uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fc.uploadedPaths) != 0 {
		t.Errorf("dry-run should not upload anything; got %v", fc.uploadedPaths)
	}
	if len(fc.createdSets) != 0 {
		t.Errorf("dry-run should not create photosets; got %v", fc.createdSets)
	}
}

// ── happy path ────────────────────────────────────────────────────────────────

func TestRun_CreatesPhotosetOnFirstPhoto(t *testing.T) {
	dataDir := makeDataDir(t, []string{"albums/vac/a.jpg", "albums/vac/b.jpg"})
	fc := &fakeFlickr{}
	statePath := filepath.Join(t.TempDir(), "state.json")
	u := New(fc, newState(), statePath, dataDir, false)

	photos := []gallery.Photo{
		makePhoto(101, "albums/vac/a.jpg"),
		makePhoto(102, "albums/vac/b.jpg"),
	}
	uploads := []AlbumUpload{makeAlbumUpload(1, "Vacation", photos)}

	if err := u.Run(uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(fc.uploadedPaths) != 2 {
		t.Errorf("uploaded %d photos, want 2", len(fc.uploadedPaths))
	}
	if len(fc.createdSets) != 1 || fc.createdSets[0] != "Vacation" {
		t.Errorf("createdSets = %v, want [Vacation]", fc.createdSets)
	}
	// Second photo is added to the existing photoset, not via CreatePhotoset.
	if len(fc.addedPhotos) != 1 {
		t.Errorf("addedPhotos = %v, want 1 entry (second photo added to photoset)", fc.addedPhotos)
	}
}

func TestRun_SkipsAlreadyUploadedPhoto(t *testing.T) {
	dataDir := makeDataDir(t, []string{"albums/vac/b.jpg"})
	state := newState()
	state.Photos[101] = "existing-fid"
	state.Photosets[1] = "existing-ps"

	fc := &fakeFlickr{}
	u := New(fc, state, filepath.Join(t.TempDir(), "state.json"), dataDir, false)

	photos := []gallery.Photo{
		makePhoto(101, "albums/vac/a.jpg"), // already done
		makePhoto(102, "albums/vac/b.jpg"), // new
	}
	uploads := []AlbumUpload{makeAlbumUpload(1, "Vacation", photos)}

	if err := u.Run(uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(fc.uploadedPaths) != 1 {
		t.Errorf("uploaded %d photos, want 1 (skip already-uploaded)", len(fc.uploadedPaths))
	}
	if len(fc.createdSets) != 0 {
		t.Errorf("should not create photoset (already exists); got %v", fc.createdSets)
	}
	if len(fc.addedPhotos) != 1 {
		t.Errorf("addedPhotos = %v, want 1 (new photo added to existing photoset)", fc.addedPhotos)
	}
}

// TestRun_ResumePhotosUploadedNoPhotoset covers the case where a prior run
// uploaded all photos but crashed before creating the photoset. All photos
// must end up in the newly-created photoset.
func TestRun_ResumePhotosUploadedNoPhotoset(t *testing.T) {
	// Both photos already in state; no photoset yet.
	state := newState()
	state.Photos[101] = "fid101"
	state.Photos[102] = "fid102"

	fc := &fakeFlickr{}
	u := New(fc, state, filepath.Join(t.TempDir(), "state.json"), t.TempDir(), false)

	photos := []gallery.Photo{
		makePhoto(101, "albums/vac/a.jpg"),
		makePhoto(102, "albums/vac/b.jpg"),
	}
	uploads := []AlbumUpload{makeAlbumUpload(1, "Vacation", photos)}

	if err := u.Run(uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fc.uploadedPaths) != 0 {
		t.Errorf("no new uploads expected; got %v", fc.uploadedPaths)
	}
	if len(fc.createdSets) != 1 {
		t.Errorf("expected 1 photoset created; got %v", fc.createdSets)
	}
	// photo 101 creates the set; photo 102 must be explicitly added.
	if len(fc.addedPhotos) != 1 {
		t.Errorf("addedPhotos = %v, want 1 (fid102 added to new photoset)", fc.addedPhotos)
	}
}

func TestRun_EmptyAlbumNoPhotoset(t *testing.T) {
	fc := &fakeFlickr{}
	u := New(fc, newState(), filepath.Join(t.TempDir(), "state.json"), t.TempDir(), false)

	uploads := []AlbumUpload{makeAlbumUpload(1, "Empty", nil)}
	if err := u.Run(uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fc.createdSets) != 0 {
		t.Errorf("empty album should not create a photoset; got %v", fc.createdSets)
	}
}

// ── error propagation ─────────────────────────────────────────────────────────

func TestRun_UploadErrorPropagates(t *testing.T) {
	dataDir := makeDataDir(t, []string{"albums/vac/a.jpg"})
	fc := &fakeFlickr{uploadErr: errors.New("network error")}
	u := New(fc, newState(), filepath.Join(t.TempDir(), "state.json"), dataDir, false)

	uploads := []AlbumUpload{
		makeAlbumUpload(1, "Vacation", []gallery.Photo{makePhoto(101, "albums/vac/a.jpg")}),
	}
	if err := u.Run(uploads); err == nil {
		t.Fatal("expected error from upload failure, got nil")
	}
}

// ── state persistence ─────────────────────────────────────────────────────────

func TestRun_StateSavedAfterUpload(t *testing.T) {
	dataDir := makeDataDir(t, []string{"albums/vac/a.jpg"})
	fc := &fakeFlickr{}
	statePath := filepath.Join(t.TempDir(), "state.json")
	u := New(fc, newState(), statePath, dataDir, false)

	uploads := []AlbumUpload{
		makeAlbumUpload(1, "Vacation", []gallery.Photo{makePhoto(101, "albums/vac/a.jpg")}),
	}
	if err := u.Run(uploads); err != nil {
		t.Fatalf("Run: %v", err)
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(loaded.Photos) == 0 {
		t.Error("state should contain uploaded photo ID")
	}
	if len(loaded.Photosets) == 0 {
		t.Error("state should contain created photoset ID")
	}
}

// ── buildTags ─────────────────────────────────────────────────────────────────

func TestBuildTags(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"cat", "cat"},
		{"cat,dog", "cat dog"},
		{"cat, dog", "cat dog"},
		{"New York,cat", `"New York" cat`},
		{",,,", ""},
		{"  spaces  ,  trimmed  ", "spaces trimmed"},
	}
	for _, tc := range cases {
		got := buildTags(tc.input)
		if got != tc.want {
			t.Errorf("buildTags(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
