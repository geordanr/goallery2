package gallery

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const rootAlbumID = 7

// Store provides access to Gallery 2 data. It is the entry point for loading
// albums, photos, and movies; the returned values carry a reference back to the
// Store so their methods can lazily load related entities.
type Store struct {
	db *sqlx.DB
}

// NewStore creates a Store backed by the given database connection.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// GetRootAlbum returns the top-level gallery album.
func (s *Store) GetRootAlbum() (*Album, error) {
	return s.GetAlbum(rootAlbumID)
}

// GetAlbum returns the album with the given ID.
func (s *Store) GetAlbum(id int) (*Album, error) {
	var row albumRow
	err := s.db.Get(&row, albumSelect+` WHERE e.g_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("GetAlbum %d: %w", id, err)
	}
	a := row.toAlbum(s)
	return &a, nil
}

// GetPhoto returns the photo with the given ID.
func (s *Store) GetPhoto(id int) (*Photo, error) {
	var row photoRow
	err := s.db.Get(&row, photoSelect+` WHERE e.g_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("GetPhoto %d: %w", id, err)
	}
	p := row.toPhoto(s)
	return &p, nil
}

// GetMovie returns the movie with the given ID.
func (s *Store) GetMovie(id int) (*Movie, error) {
	var row movieRow
	err := s.db.Get(&row, movieSelect+` WHERE e.g_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("GetMovie %d: %w", id, err)
	}
	m := row.toMovie(s)
	return &m, nil
}

// ── internal query helpers ────────────────────────────────────────────────────

func (s *Store) childAlbums(parentID int) ([]Album, error) {
	var rows []albumRow
	err := s.db.Select(&rows, albumSelect+` WHERE ce.g_parentId = ? ORDER BY i.g_title`, parentID)
	if err != nil {
		return nil, fmt.Errorf("childAlbums %d: %w", parentID, err)
	}
	albums := make([]Album, len(rows))
	for i, r := range rows {
		albums[i] = r.toAlbum(s)
	}
	return albums, nil
}

func (s *Store) albumPhotos(albumID int) ([]Photo, error) {
	var rows []photoRow
	err := s.db.Select(&rows, photoSelect+` WHERE ce.g_parentId = ? ORDER BY i.g_originationTimestamp, i.g_title`, albumID)
	if err != nil {
		return nil, fmt.Errorf("albumPhotos %d: %w", albumID, err)
	}
	photos := make([]Photo, len(rows))
	for i, r := range rows {
		photos[i] = r.toPhoto(s)
	}
	return photos, nil
}

func (s *Store) albumMovies(albumID int) ([]Movie, error) {
	var rows []movieRow
	err := s.db.Select(&rows, movieSelect+` WHERE ce.g_parentId = ? ORDER BY i.g_originationTimestamp, i.g_title`, albumID)
	if err != nil {
		return nil, fmt.Errorf("albumMovies %d: %w", albumID, err)
	}
	movies := make([]Movie, len(rows))
	for i, r := range rows {
		movies[i] = r.toMovie(s)
	}
	return movies, nil
}

func (s *Store) derivatives(sourceID int) ([]Derivative, error) {
	var rows []derivativeRow
	err := s.db.Select(&rows, derivativeSelect+` WHERE d.g_derivativeSourceId = ? ORDER BY d.g_derivativeOrder`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("derivatives %d: %w", sourceID, err)
	}
	derivs := make([]Derivative, len(rows))
	for i, r := range rows {
		derivs[i] = r.toDerivative()
	}
	return derivs, nil
}

func (s *Store) thumbnail(sourceID int) (*Derivative, error) {
	var row derivativeRow
	err := s.db.Get(&row, derivativeSelect+` WHERE d.g_derivativeSourceId = ? AND d.g_derivativeType = ?`,
		sourceID, DerivativeThumbnail)
	if err != nil {
		return nil, fmt.Errorf("thumbnail %d: %w", sourceID, err)
	}
	d := row.toDerivative()
	return &d, nil
}

// itemPath walks g2_ChildEntity upward from id to the root, collecting
// g_pathComponent values, and joins them into a relative disk path.
func (s *Store) itemPath(id int) (string, error) {
	type node struct {
		ParentID      int    `db:"parent_id"`
		PathComponent string `db:"path_component"`
	}

	var components []string
	current := id

	for current != 0 {
		var n node
		err := s.db.Get(&n, `
			SELECT ce.g_parentId                       AS parent_id,
			       COALESCE(fse.g_pathComponent, '')   AS path_component
			FROM g2_ChildEntity ce
			JOIN g2_FileSystemEntity fse ON fse.g_id = ce.g_id
			WHERE ce.g_id = ?`, current)
		if err != nil {
			return "", fmt.Errorf("itemPath %d (at node %d): %w", id, current, err)
		}
		if n.PathComponent != "" {
			components = append(components, n.PathComponent)
		}
		current = n.ParentID
	}

	// Reverse: collected leaf→root, want root→leaf.
	slices.Reverse(components)

	return joinPathComponents(components), nil
}

// joinPathComponents joins path components with forward slashes regardless of
// OS. Paths are used in URLs and as arguments to filepath.FromSlash, so they
// must never contain OS-native separators.
func joinPathComponents(components []string) string {
	return strings.Join(components, "/")
}

// ── scan row types ────────────────────────────────────────────────────────────

const albumSelect = `
SELECT
    e.g_id                            AS id,
    ce.g_parentId                     AS parent_id,
    COALESCE(i.g_title, '')           AS title,
    COALESCE(i.g_description, '')     AS description,
    COALESCE(i.g_keywords, '')        AS keywords,
    COALESCE(i.g_summary, '')         AS summary,
    COALESCE(fse.g_pathComponent, '') AS path_component,
    COALESCE(ai.g_orderBy, '')        AS order_by,
    COALESCE(ai.g_orderDirection, '') AS order_direction,
    e.g_creationTimestamp             AS created_at,
    e.g_modificationTimestamp         AS modified_at,
    i.g_originationTimestamp          AS originated_at
FROM g2_Entity e
JOIN g2_Item i               ON i.g_id    = e.g_id
JOIN g2_FileSystemEntity fse ON fse.g_id  = e.g_id
JOIN g2_AlbumItem ai         ON ai.g_id   = e.g_id
JOIN g2_ChildEntity ce       ON ce.g_id   = e.g_id`

type albumRow struct {
	ID             int    `db:"id"`
	ParentID       int    `db:"parent_id"`
	Title          string `db:"title"`
	Description    string `db:"description"`
	Keywords       string `db:"keywords"`
	Summary        string `db:"summary"`
	PathComponent  string `db:"path_component"`
	OrderBy        string `db:"order_by"`
	OrderDirection string `db:"order_direction"`
	CreatedAt      int64  `db:"created_at"`
	ModifiedAt     int64  `db:"modified_at"`
	OriginatedAt   int64  `db:"originated_at"`
}

func (r albumRow) toAlbum(s *Store) Album {
	return Album{
		store:          s,
		ID:             r.ID,
		ParentID:       r.ParentID,
		Title:          r.Title,
		Description:    r.Description,
		Keywords:       r.Keywords,
		Summary:        r.Summary,
		PathComponent:  r.PathComponent,
		OrderBy:        r.OrderBy,
		OrderDirection: r.OrderDirection,
		CreatedAt:      time.Unix(r.CreatedAt, 0),
		ModifiedAt:     time.Unix(r.ModifiedAt, 0),
		OriginatedAt:   time.Unix(r.OriginatedAt, 0),
	}
}

const photoSelect = `
SELECT
    e.g_id                            AS id,
    ce.g_parentId                     AS parent_id,
    COALESCE(i.g_title, '')           AS title,
    COALESCE(i.g_description, '')     AS description,
    COALESCE(i.g_keywords, '')        AS keywords,
    COALESCE(i.g_summary, '')         AS summary,
    COALESCE(fse.g_pathComponent, '') AS path_component,
    COALESCE(di.g_mimeType, '')       AS mime_type,
    COALESCE(di.g_size, 0)            AS file_size,
    COALESCE(pi.g_width, 0)           AS width,
    COALESCE(pi.g_height, 0)          AS height,
    e.g_creationTimestamp             AS created_at,
    e.g_modificationTimestamp         AS modified_at,
    i.g_originationTimestamp          AS originated_at
FROM g2_Entity e
JOIN g2_Item i               ON i.g_id    = e.g_id
JOIN g2_FileSystemEntity fse ON fse.g_id  = e.g_id
JOIN g2_DataItem di          ON di.g_id   = e.g_id
JOIN g2_PhotoItem pi         ON pi.g_id   = e.g_id
JOIN g2_ChildEntity ce       ON ce.g_id   = e.g_id`

type photoRow struct {
	ID            int    `db:"id"`
	ParentID      int    `db:"parent_id"`
	Title         string `db:"title"`
	Description   string `db:"description"`
	Keywords      string `db:"keywords"`
	Summary       string `db:"summary"`
	PathComponent string `db:"path_component"`
	MimeType      string `db:"mime_type"`
	FileSize      int    `db:"file_size"`
	Width         int    `db:"width"`
	Height        int    `db:"height"`
	CreatedAt     int64  `db:"created_at"`
	ModifiedAt    int64  `db:"modified_at"`
	OriginatedAt  int64  `db:"originated_at"`
}

func (r photoRow) toPhoto(s *Store) Photo {
	return Photo{
		store:         s,
		ID:            r.ID,
		ParentID:      r.ParentID,
		Title:         r.Title,
		Description:   r.Description,
		Keywords:      r.Keywords,
		Summary:       r.Summary,
		PathComponent: r.PathComponent,
		MimeType:      r.MimeType,
		FileSize:      r.FileSize,
		Width:         r.Width,
		Height:        r.Height,
		CreatedAt:     time.Unix(r.CreatedAt, 0),
		ModifiedAt:    time.Unix(r.ModifiedAt, 0),
		OriginatedAt:  time.Unix(r.OriginatedAt, 0),
	}
}

const movieSelect = `
SELECT
    e.g_id                            AS id,
    ce.g_parentId                     AS parent_id,
    COALESCE(i.g_title, '')           AS title,
    COALESCE(i.g_description, '')     AS description,
    COALESCE(i.g_keywords, '')        AS keywords,
    COALESCE(i.g_summary, '')         AS summary,
    COALESCE(fse.g_pathComponent, '') AS path_component,
    COALESCE(di.g_mimeType, '')       AS mime_type,
    COALESCE(di.g_size, 0)            AS file_size,
    COALESCE(mi.g_width, 0)           AS width,
    COALESCE(mi.g_height, 0)          AS height,
    COALESCE(mi.g_duration, 0)        AS duration,
    e.g_creationTimestamp             AS created_at,
    e.g_modificationTimestamp         AS modified_at,
    i.g_originationTimestamp          AS originated_at
FROM g2_Entity e
JOIN g2_Item i               ON i.g_id    = e.g_id
JOIN g2_FileSystemEntity fse ON fse.g_id  = e.g_id
JOIN g2_DataItem di          ON di.g_id   = e.g_id
JOIN g2_MovieItem mi         ON mi.g_id   = e.g_id
JOIN g2_ChildEntity ce       ON ce.g_id   = e.g_id`

type movieRow struct {
	ID            int    `db:"id"`
	ParentID      int    `db:"parent_id"`
	Title         string `db:"title"`
	Description   string `db:"description"`
	Keywords      string `db:"keywords"`
	Summary       string `db:"summary"`
	PathComponent string `db:"path_component"`
	MimeType      string `db:"mime_type"`
	FileSize      int    `db:"file_size"`
	Width         int    `db:"width"`
	Height        int    `db:"height"`
	Duration      int    `db:"duration"`
	CreatedAt     int64  `db:"created_at"`
	ModifiedAt    int64  `db:"modified_at"`
	OriginatedAt  int64  `db:"originated_at"`
}

func (r movieRow) toMovie(s *Store) Movie {
	return Movie{
		store:         s,
		ID:            r.ID,
		ParentID:      r.ParentID,
		Title:         r.Title,
		Description:   r.Description,
		Keywords:      r.Keywords,
		Summary:       r.Summary,
		PathComponent: r.PathComponent,
		MimeType:      r.MimeType,
		FileSize:      r.FileSize,
		Width:         r.Width,
		Height:        r.Height,
		Duration:      r.Duration,
		CreatedAt:     time.Unix(r.CreatedAt, 0),
		ModifiedAt:    time.Unix(r.ModifiedAt, 0),
		OriginatedAt:  time.Unix(r.OriginatedAt, 0),
	}
}

const derivativeSelect = `
SELECT
    e.g_id                                 AS id,
    d.g_derivativeSourceId                 AS source_id,
    COALESCE(d.g_derivativeOperations, '') AS operations,
    d.g_derivativeOrder                    AS order_num,
    COALESCE(d.g_derivativeSize, 0)        AS file_size,
    d.g_derivativeType                     AS type,
    d.g_mimeType                           AS mime_type,
    COALESCE(di.g_width, 0)               AS width,
    COALESCE(di.g_height, 0)              AS height
FROM g2_Entity e
JOIN g2_Derivative d       ON d.g_id  = e.g_id
JOIN g2_DerivativeImage di ON di.g_id = e.g_id`

type derivativeRow struct {
	ID         int    `db:"id"`
	SourceID   int    `db:"source_id"`
	Operations string `db:"operations"`
	Order      int    `db:"order_num"`
	FileSize   int    `db:"file_size"`
	Type       int    `db:"type"`
	MimeType   string `db:"mime_type"`
	Width      int    `db:"width"`
	Height     int    `db:"height"`
}

func (r derivativeRow) toDerivative() Derivative {
	return Derivative{
		ID:         r.ID,
		SourceID:   r.SourceID,
		Operations: r.Operations,
		Order:      r.Order,
		FileSize:   r.FileSize,
		Type:       DerivativeType(r.Type),
		MimeType:   r.MimeType,
		Width:      r.Width,
		Height:     r.Height,
	}
}
