package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/gallery"
)

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

func (r movieRow) toMovie() gallery.Movie {
	return gallery.Movie{
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

// GetMovie returns the movie with the given ID.
func GetMovie(db *sqlx.DB, id int) (*gallery.Movie, error) {
	var row movieRow
	err := db.Get(&row, movieSelect+` WHERE e.g_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("GetMovie %d: %w", id, err)
	}
	m := row.toMovie()
	return &m, nil
}

// GetAlbumMovies returns all movies in the given album.
func GetAlbumMovies(db *sqlx.DB, albumID int) ([]gallery.Movie, error) {
	var rows []movieRow
	err := db.Select(&rows, movieSelect+` WHERE ce.g_parentId = ? ORDER BY i.g_originationTimestamp, i.g_title`, albumID)
	if err != nil {
		return nil, fmt.Errorf("GetAlbumMovies %d: %w", albumID, err)
	}
	movies := make([]gallery.Movie, len(rows))
	for i, r := range rows {
		movies[i] = r.toMovie()
	}
	return movies, nil
}
