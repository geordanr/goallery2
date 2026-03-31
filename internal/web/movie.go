package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/gallery"
)

type movieData struct {
	Movie       *gallery.Movie
	DiskPath    string
	Breadcrumbs []breadcrumb
}

func (h *handler) movie(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid movie id", http.StatusBadRequest)
		return
	}

	movie, err := h.store.GetMovie(id)
	if err != nil {
		http.Error(w, "movie not found", http.StatusNotFound)
		return
	}

	diskPath, err := movie.Path()
	if err != nil {
		http.Error(w, "error resolving movie path", http.StatusInternalServerError)
		return
	}

	breadcrumbs, err := h.movieBreadcrumbs(movie)
	if err != nil {
		http.Error(w, "error building breadcrumbs", http.StatusInternalServerError)
		return
	}

	data := movieData{
		Movie:       movie,
		DiskPath:    diskPath,
		Breadcrumbs: breadcrumbs,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := movieTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func (h *handler) movieBreadcrumbs(movie *gallery.Movie) ([]breadcrumb, error) {
	parent, err := h.store.GetAlbum(movie.ParentID)
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
	title := movie.Title
	if title == "" {
		title = movie.PathComponent
	}
	return append(crumbs, breadcrumb{Title: title}), nil
}
