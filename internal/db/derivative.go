package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/gallery"
)

const derivativeSelect = `
SELECT
    e.g_id                              AS id,
    d.g_derivativeSourceId              AS source_id,
    COALESCE(d.g_derivativeOperations, '') AS operations,
    d.g_derivativeOrder                 AS order_num,
    COALESCE(d.g_derivativeSize, 0)     AS file_size,
    d.g_derivativeType                  AS type,
    d.g_mimeType                        AS mime_type,
    COALESCE(di.g_width, 0)             AS width,
    COALESCE(di.g_height, 0)            AS height
FROM g2_Entity e
JOIN g2_Derivative d      ON d.g_id = e.g_id
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

func (r derivativeRow) toDerivative() gallery.Derivative {
	return gallery.Derivative{
		ID:         r.ID,
		SourceID:   r.SourceID,
		Operations: r.Operations,
		Order:      r.Order,
		FileSize:   r.FileSize,
		Type:       gallery.DerivativeType(r.Type),
		MimeType:   r.MimeType,
		Width:      r.Width,
		Height:     r.Height,
	}
}

// GetDerivatives returns all derivatives for the given source item ID.
func GetDerivatives(db *sqlx.DB, sourceID int) ([]gallery.Derivative, error) {
	var rows []derivativeRow
	err := db.Select(&rows, derivativeSelect+` WHERE d.g_derivativeSourceId = ? ORDER BY d.g_derivativeOrder`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("GetDerivatives %d: %w", sourceID, err)
	}
	derivs := make([]gallery.Derivative, len(rows))
	for i, r := range rows {
		derivs[i] = r.toDerivative()
	}
	return derivs, nil
}

// GetThumbnail returns the thumbnail derivative for the given source item ID, or nil if none exists.
func GetThumbnail(db *sqlx.DB, sourceID int) (*gallery.Derivative, error) {
	var row derivativeRow
	err := db.Get(&row, derivativeSelect+` WHERE d.g_derivativeSourceId = ? AND d.g_derivativeType = ?`,
		sourceID, gallery.DerivativeThumbnail)
	if err != nil {
		return nil, fmt.Errorf("GetThumbnail %d: %w", sourceID, err)
	}
	d := row.toDerivative()
	return &d, nil
}
