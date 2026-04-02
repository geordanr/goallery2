package gallery

import "time"

// SortOrder controls the ORDER BY clause used when listing photos, movies, and albums.
type SortOrder int

const (
	// SortByDate orders by origination timestamp then title. This is the default.
	SortByDate SortOrder = iota
	// SortByTitle orders alphabetically by title.
	SortByTitle
	// SortByModified orders by last-modified timestamp then title.
	SortByModified
)

// String returns the canonical URL query value for this sort order.
func (s SortOrder) String() string {
	switch s {
	case SortByTitle:
		return "title"
	case SortByModified:
		return "modified"
	default:
		return "date"
	}
}

// orderByClause returns a safe SQL ORDER BY expression for this sort order.
// The returned string contains only hardcoded column references — never user input.
func (s SortOrder) orderByClause() string {
	switch s {
	case SortByTitle:
		return "i.g_title"
	case SortByModified:
		return "e.g_modificationTimestamp, i.g_title"
	default: // SortByDate
		return "i.g_originationTimestamp, i.g_title"
	}
}

// DerivativeType corresponds to g2_Derivative.g_derivativeType.
type DerivativeType int

const (
	DerivativeThumbnail DerivativeType = 1
	DerivativeResized   DerivativeType = 2
)

// Album represents a GalleryAlbumItem, joining:
//
//	g2_Entity + g2_Item + g2_FileSystemEntity + g2_AlbumItem + g2_ChildEntity
type Album struct {
	reader         Reader
	ID             int
	ParentID       int // 0 = root
	Title          string
	Description    string
	Keywords       string
	Summary        string
	PathComponent  string // directory name on disk; NULL for root (ID 7)
	CoverPhotoID   int    // ID of the first photo in this album; 0 if album has no photos
	OrderBy        string
	OrderDirection string
	CreatedAt      time.Time
	ModifiedAt     time.Time
	OriginatedAt   time.Time
}

// Photo represents a GalleryPhotoItem, joining:
//
//	g2_Entity + g2_Item + g2_FileSystemEntity + g2_DataItem + g2_PhotoItem + g2_ChildEntity
type Photo struct {
	reader        Reader
	ID            int
	ParentID      int // album ID
	Title         string
	Description   string
	Keywords      string
	Summary       string
	PathComponent string // filename on disk
	MimeType      string
	FileSize      int
	Width         int
	Height        int
	CreatedAt     time.Time
	ModifiedAt    time.Time
	OriginatedAt  time.Time
}

// Movie represents a GalleryMovieItem, joining:
//
//	g2_Entity + g2_Item + g2_FileSystemEntity + g2_DataItem + g2_MovieItem + g2_ChildEntity
type Movie struct {
	reader        Reader
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
	Duration      int // seconds
	CreatedAt     time.Time
	ModifiedAt    time.Time
	OriginatedAt  time.Time
}

// Derivative represents a GalleryDerivativeImage (thumbnail or resized copy), joining:
//
//	g2_Entity + g2_Derivative + g2_DerivativeImage
//
// Derivatives are NOT in g2_FileSystemEntity. Their on-disk paths are determined
// by Gallery 2's cache layout (to be confirmed once image files are available).
type Derivative struct {
	ID         int
	SourceID   int    // ID of the Photo or Movie this was derived from
	Operations string // e.g. "thumbnail|150"
	Order      int    // ordering among derivatives of the same source
	FileSize   int
	Type       DerivativeType // DerivativeThumbnail or DerivativeResized
	MimeType   string
	Width      int
	Height     int
}
