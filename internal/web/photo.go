package web

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/gallery"
)

type photoData struct {
	Photo        *gallery.Photo
	DiskPath     string
	ResizedPath  string // non-empty when a resized derivative exists; use /files/ResizedPath
	DisplayTitle string // Photo.Title if set, otherwise Photo.PathComponent
	Breadcrumbs  []breadcrumb
}

func (h *handler) photo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid photo id", http.StatusBadRequest)
		return
	}

	photo, err := h.store.GetPhoto(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "photo not found", http.StatusNotFound)
		} else {
			http.Error(w, "error loading photo", http.StatusInternalServerError)
		}
		return
	}

	diskPath, err := photo.Path()
	if err != nil {
		http.Error(w, "error resolving photo path", http.StatusInternalServerError)
		return
	}

	breadcrumbs, err := h.photoBreadcrumbs(photo)
	if err != nil {
		http.Error(w, "error building breadcrumbs", http.StatusInternalServerError)
		return
	}

	// Derivative query failure degrades gracefully — the photo loaded
	// successfully, so show the full-size image and log the error.
	// GetResized returns results sorted by Order, so index 0 is the primary.
	resized, resizedErr := photo.GetResized()
	if resizedErr != nil {
		slog.Error("could not load resized derivatives; falling back to full-size", "photo_id", id, "err", resizedErr)
	}
	// Find the first resized derivative whose cache file actually exists on disk.
	// Gallery 2 sometimes writes a DB record without generating the cache file.
	var resizedPath string
	if resizedErr == nil {
		for _, d := range resized {
			candidate := filepath.Join(h.absDataDir, filepath.FromSlash(d.CachePath()))
			if _, err := os.Stat(candidate); err == nil {
				resizedPath = d.CachePath()
				break
			}
		}
	}

	data := photoData{
		Photo:        photo,
		DiskPath:     diskPath,
		ResizedPath:  resizedPath,
		DisplayTitle: photo.DisplayTitle(),
		Breadcrumbs:  breadcrumbs,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := photoTmpl.Execute(w, data); err != nil {
		// Headers already sent; http.Error would be ignored. Log instead.
		slog.Error("photo template render failed", "photo_id", id, "err", err)
	}
}

func (h *handler) photoBreadcrumbs(photo *gallery.Photo) ([]breadcrumb, error) {
	parent, err := h.store.GetAlbum(photo.ParentID)
	if err != nil {
		return nil, err
	}
	crumbs, err := h.albumBreadcrumbs(parent)
	if err != nil {
		return nil, err
	}
	// Restore the parent album link (albumBreadcrumbs leaves the leaf without an ID).
	if len(crumbs) > 0 {
		crumbs[len(crumbs)-1].ID = parent.ID
	}
	return append(crumbs, breadcrumb{Title: photo.DisplayTitle()}), nil
}
