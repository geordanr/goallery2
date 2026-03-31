package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/gallery"
)

func (h *handler) photoThumbnail(w http.ResponseWriter, r *http.Request) {
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
	thumb, err := photo.GetThumbnail()
	if err != nil {
		http.Error(w, "thumbnail not found", http.StatusNotFound)
		return
	}
	h.serveDerivative(w, r, thumb)
}

func (h *handler) movieThumbnail(w http.ResponseWriter, r *http.Request) {
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
	thumb, err := movie.GetThumbnail()
	if err != nil {
		http.Error(w, "thumbnail not found", http.StatusNotFound)
		return
	}
	h.serveDerivative(w, r, thumb)
}

// serveDerivative resolves the derivative's cache path under DataDir and
// streams it with the MIME type from the DB. The .dat extension on cache files
// is meaningless, so we set Content-Type explicitly rather than letting
// http.ServeFile infer it from the filename.
func (h *handler) serveDerivative(w http.ResponseWriter, r *http.Request, d *gallery.Derivative) {
	absDataDir, err := filepath.Abs(h.config.Server.DataDir)
	if err != nil {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		return
	}

	target := filepath.Clean(filepath.Join(absDataDir, filepath.FromSlash(d.CachePath())))

	if !strings.HasPrefix(target, absDataDir+string(filepath.Separator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	f, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
		} else {
			http.Error(w, "error accessing file", http.StatusInternalServerError)
		}
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "error accessing file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", d.MimeType)
	http.ServeContent(w, r, "", stat.ModTime(), f)
}

