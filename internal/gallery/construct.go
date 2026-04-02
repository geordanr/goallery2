package gallery

import "time"

// This file provides constructors for domain types. Because Album, Photo, and
// Movie carry an unexported reader field, callers outside this package cannot
// construct them with struct literals. These functions are used primarily in
// tests that need to inject a fake Reader.

// PhotoFields holds all public fields for constructing a Photo.
type PhotoFields struct {
	ID            int
	ParentID      int
	Title         string
	Description   string
	Keywords      string
	Summary       string
	PathComponent string
	MimeType      string
	FileSize      int
	Width         int
	Height        int
	CreatedAt     time.Time
	ModifiedAt    time.Time
	OriginatedAt  time.Time
}

// NewPhoto constructs a Photo bound to r with the provided field values.
func NewPhoto(r Reader, f PhotoFields) *Photo {
	return &Photo{
		reader:        r,
		ID:            f.ID,
		ParentID:      f.ParentID,
		Title:         f.Title,
		Description:   f.Description,
		Keywords:      f.Keywords,
		Summary:       f.Summary,
		PathComponent: f.PathComponent,
		MimeType:      f.MimeType,
		FileSize:      f.FileSize,
		Width:         f.Width,
		Height:        f.Height,
		CreatedAt:     f.CreatedAt,
		ModifiedAt:    f.ModifiedAt,
		OriginatedAt:  f.OriginatedAt,
	}
}

// AlbumFields holds all public fields for constructing an Album.
type AlbumFields struct {
	ID             int
	ParentID       int
	Title          string
	Description    string
	Keywords       string
	Summary        string
	PathComponent  string
	OrderBy        string
	OrderDirection string
	CreatedAt      time.Time
	ModifiedAt     time.Time
	OriginatedAt   time.Time
}

// NewAlbum constructs an Album bound to r with the provided field values.
func NewAlbum(r Reader, f AlbumFields) *Album {
	return &Album{
		reader:         r,
		ID:             f.ID,
		ParentID:       f.ParentID,
		Title:          f.Title,
		Description:    f.Description,
		Keywords:       f.Keywords,
		Summary:        f.Summary,
		PathComponent:  f.PathComponent,
		OrderBy:        f.OrderBy,
		OrderDirection: f.OrderDirection,
		CreatedAt:      f.CreatedAt,
		ModifiedAt:     f.ModifiedAt,
		OriginatedAt:   f.OriginatedAt,
	}
}
