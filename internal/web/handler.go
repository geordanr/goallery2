package web

import (
	"net/http"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/gallery"
)

type handler struct {
	store      gallery.Reader
	config     config.Config
	absDataDir string // filepath.Abs(config.Server.DataDir), resolved once at construction
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/album/7", http.StatusFound)
}
