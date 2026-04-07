package uploader

import (
	"errors"
	"testing"

	"github.com/geordanr/goallery2/internal/gallery"
)

// walkReader is a fake gallery.Reader that serves a fixed album tree.
// Only the methods called by Walk are wired up.
type walkReader struct {
	albums   map[int]gallery.Album
	children map[int][]gallery.Album
	photos   map[int][]gallery.Photo
}

func (r *walkReader) GetRootAlbum() (*gallery.Album, error)  { panic("not implemented") }
func (r *walkReader) GetMovie(_ int) (*gallery.Movie, error) { panic("not implemented") }
func (r *walkReader) AlbumMovies(_ int, _ gallery.SortOrder) ([]gallery.Movie, error) {
	panic("not implemented")
}
func (r *walkReader) Derivatives(_ int) ([]gallery.Derivative, error) { panic("not implemented") }
func (r *walkReader) Thumbnail(_ int) (*gallery.Derivative, error)    { panic("not implemented") }
func (r *walkReader) ItemPath(_ int) (string, error)                  { panic("not implemented") }
func (r *walkReader) GetPhoto(_ int) (*gallery.Photo, error)          { panic("not implemented") }

func (r *walkReader) GetAlbum(id int) (*gallery.Album, error) {
	a, ok := r.albums[id]
	if !ok {
		return nil, errors.New("album not found")
	}
	return &a, nil
}

func (r *walkReader) ChildAlbums(parentID int, _ gallery.SortOrder) ([]gallery.Album, error) {
	return r.children[parentID], nil
}

func (r *walkReader) AlbumPhotos(albumID int, _ gallery.SortOrder) ([]gallery.Photo, error) {
	return r.photos[albumID], nil
}

// newWalkReader builds a fake reader from a simple description.
// albums is a map of id → title; children is a map of parentID → []childID;
// photoCount is a map of albumID → number of fake photos.
func newWalkReader(
	titles map[int]string,
	children map[int][]int,
	photoCount map[int]int,
) *walkReader {
	r := &walkReader{
		albums:   make(map[int]gallery.Album),
		children: make(map[int][]gallery.Album),
		photos:   make(map[int][]gallery.Photo),
	}
	for id, title := range titles {
		r.albums[id] = *gallery.NewAlbum(r, gallery.AlbumFields{ID: id, Title: title})
	}
	for parentID, childIDs := range children {
		for _, cid := range childIDs {
			r.children[parentID] = append(r.children[parentID], r.albums[cid])
		}
	}
	for albumID, n := range photoCount {
		photos := make([]gallery.Photo, n)
		for i := range photos {
			photos[i] = *gallery.NewPhoto(r, gallery.PhotoFields{ID: albumID*1000 + i, ParentID: albumID})
		}
		r.photos[albumID] = photos
	}
	return r
}

func TestWalk_SingleAlbum(t *testing.T) {
	r := newWalkReader(
		map[int]string{1: "Vacation"},
		nil,
		map[int]int{1: 3},
	)
	uploads, err := Walk(r, []int{1})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(uploads) != 1 {
		t.Fatalf("got %d uploads, want 1", len(uploads))
	}
	if uploads[0].FlickrTitle != "Vacation" {
		t.Errorf("FlickrTitle = %q, want %q", uploads[0].FlickrTitle, "Vacation")
	}
	if len(uploads[0].Photos) != 3 {
		t.Errorf("photo count = %d, want 3", len(uploads[0].Photos))
	}
}

func TestWalk_ChildTitlePrefixed(t *testing.T) {
	r := newWalkReader(
		map[int]string{1: "Trip", 2: "Day 1", 3: "Morning"},
		map[int][]int{1: {2}, 2: {3}},
		nil,
	)
	uploads, err := Walk(r, []int{1})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(uploads) != 3 {
		t.Fatalf("got %d uploads, want 3", len(uploads))
	}

	want := []string{"Trip", "Trip - Day 1", "Trip - Day 1 - Morning"}
	for i, u := range uploads {
		if u.FlickrTitle != want[i] {
			t.Errorf("uploads[%d].FlickrTitle = %q, want %q", i, u.FlickrTitle, want[i])
		}
	}
}

func TestWalk_MultipleRoots(t *testing.T) {
	r := newWalkReader(
		map[int]string{10: "Alpha", 20: "Beta"},
		nil,
		nil,
	)
	uploads, err := Walk(r, []int{10, 20})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(uploads) != 2 {
		t.Fatalf("got %d uploads, want 2", len(uploads))
	}
	if uploads[0].FlickrTitle != "Alpha" || uploads[1].FlickrTitle != "Beta" {
		t.Errorf("unexpected titles: %q, %q", uploads[0].FlickrTitle, uploads[1].FlickrTitle)
	}
}

func TestWalk_EmptyAlbumIncluded(t *testing.T) {
	r := newWalkReader(
		map[int]string{1: "Empty"},
		nil,
		nil, // no photos
	)
	uploads, err := Walk(r, []int{1})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(uploads) != 1 {
		t.Fatalf("got %d uploads, want 1 (empty albums must be included)", len(uploads))
	}
	if len(uploads[0].Photos) != 0 {
		t.Errorf("expected 0 photos, got %d", len(uploads[0].Photos))
	}
}

func TestFlickrTitle(t *testing.T) {
	cases := []struct {
		ancestors []string
		title     string
		want      string
	}{
		{nil, "Vacation", "Vacation"},
		{[]string{"Trip"}, "Day 1", "Trip - Day 1"},
		{[]string{"A", "B"}, "C", "A - B - C"},
	}
	for _, tc := range cases {
		got := FlickrTitle(tc.ancestors, tc.title)
		if got != tc.want {
			t.Errorf("FlickrTitle(%v, %q) = %q, want %q", tc.ancestors, tc.title, got, tc.want)
		}
	}
}
