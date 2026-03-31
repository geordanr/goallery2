package db

import (
	"fmt"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

// GetItemPath builds the full relative disk path for any item or album by
// walking g2_ChildEntity upward to the root, collecting g_pathComponent from
// g2_FileSystemEntity at each level.
//
// The root album (ID 7) has a NULL path component and is excluded, so the
// returned path is relative to the gallery root directory.
// Example: "Wang/Wang2006/Wang2006Sep/Wang2006SepItaly/photo.jpg"
func GetItemPath(db *sqlx.DB, id int) (string, error) {
	type node struct {
		ParentID      int    `db:"parent_id"`
		PathComponent string `db:"path_component"`
	}

	var components []string
	current := id

	for current != 0 {
		var n node
		err := db.Get(&n, `
			SELECT ce.g_parentId              AS parent_id,
			       COALESCE(fse.g_pathComponent, '') AS path_component
			FROM g2_ChildEntity ce
			JOIN g2_FileSystemEntity fse ON fse.g_id = ce.g_id
			WHERE ce.g_id = ?`, current)
		if err != nil {
			return "", fmt.Errorf("GetItemPath %d (at node %d): %w", id, current, err)
		}
		if n.PathComponent != "" {
			components = append(components, n.PathComponent)
		}
		current = n.ParentID
	}

	// Reverse: we collected leaf→root, want root→leaf.
	for i, j := 0, len(components)-1; i < j; i, j = i+1, j-1 {
		components[i], components[j] = components[j], components[i]
	}

	return filepath.Join(components...), nil
}
