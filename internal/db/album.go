package db

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/gallery"
)

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

func (r albumRow) toAlbum() gallery.Album {
	return gallery.Album{
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

// GetAlbum returns the album with the given ID.
func GetAlbum(db *sqlx.DB, id int) (*gallery.Album, error) {
	var row albumRow
	err := db.Get(&row, albumSelect+` WHERE e.g_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("GetAlbum %d: %w", id, err)
	}
	a := row.toAlbum()
	return &a, nil
}

// GetChildAlbums returns all direct child albums of parentID, in their stored order.
func GetChildAlbums(db *sqlx.DB, parentID int) ([]gallery.Album, error) {
	var rows []albumRow
	err := db.Select(&rows, albumSelect+` WHERE ce.g_parentId = ? ORDER BY i.g_title`, parentID)
	if err != nil {
		return nil, fmt.Errorf("GetChildAlbums %d: %w", parentID, err)
	}
	albums := make([]gallery.Album, len(rows))
	for i, r := range rows {
		albums[i] = r.toAlbum()
	}
	return albums, nil
}
