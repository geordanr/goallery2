package web

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/gallery"
)

func RegisterRoutes(r *chi.Mux, db *sqlx.DB, cfg config.Config) {
	h := &handler{
		store:  gallery.NewStore(db),
		config: cfg,
	}

	r.Get("/", h.index)
	r.Get("/album/{id}", h.album)
	r.Get("/photo/{id}", h.photo)
	r.Get("/movie/{id}", h.movie)
	r.Get("/files/{path...}", h.serveFile)
}
