package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/gallery"
)

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

func (r photoRow) toPhoto() gallery.Photo {
	return gallery.Photo{
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

// GetPhoto returns the photo with the given ID.
func GetPhoto(db *sqlx.DB, id int) (*gallery.Photo, error) {
	var row photoRow
	err := db.Get(&row, photoSelect+` WHERE e.g_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("GetPhoto %d: %w", id, err)
	}
	p := row.toPhoto()
	return &p, nil
}

// GetAlbumPhotos returns all photos in the given album.
func GetAlbumPhotos(db *sqlx.DB, albumID int) ([]gallery.Photo, error) {
	var rows []photoRow
	err := db.Select(&rows, photoSelect+` WHERE ce.g_parentId = ? ORDER BY i.g_originationTimestamp, i.g_title`, albumID)
	if err != nil {
		return nil, fmt.Errorf("GetAlbumPhotos %d: %w", albumID, err)
	}
	photos := make([]gallery.Photo, len(rows))
	for i, r := range rows {
		photos[i] = r.toPhoto()
	}
	return photos, nil
}
