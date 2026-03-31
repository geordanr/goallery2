package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/gallery"
)

type photoData struct {
	Photo       *gallery.Photo
	DiskPath    string
	Breadcrumbs []breadcrumb
}

func (h *handler) photo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid photo id", http.StatusBadRequest)
		return
	}

	photo, err := h.store.GetPhoto(id)
	if err != nil {
		http.Error(w, "photo not found", http.StatusNotFound)
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

	data := photoData{
		Photo:       photo,
		DiskPath:    diskPath,
		Breadcrumbs: breadcrumbs,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := photoTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
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
	title := photo.Title
	if title == "" {
		title = photo.PathComponent
	}
	return append(crumbs, breadcrumb{Title: title}), nil
}
