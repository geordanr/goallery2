package web

import (
	"fmt"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/gallery"
)

func RegisterRoutes(r *chi.Mux, db *sqlx.DB, cfg config.Config) error {
	absDataDir, err := filepath.Abs(cfg.Server.DataDir)
	if err != nil {
		return fmt.Errorf("could not resolve data directory %q: %w", cfg.Server.DataDir, err)
	}

	h := &handler{
		store:      gallery.NewStore(db),
		config:     cfg,
		absDataDir: absDataDir,
	}

	r.Get("/", h.index)
	r.Get("/album/{id}", h.album)
	r.Get("/photo/{id}", h.photo)
	r.Get("/thumbnail/{id}", h.thumbnail)
	r.Get("/photo/{id}/thumbnail", h.photoThumbnail)
	r.Get("/movie/{id}", h.movie)
	r.Get("/movie/{id}/thumbnail", h.movieThumbnail)
	r.Get("/files/*", h.serveFile)
	r.Get("/flickr/verify", h.flickrVerify)
	r.Get("/flickr/upload/{id}", h.flickrUpload)
	r.Post("/flickr/upload/{id}", h.flickrUpload)
	r.Get("/oauth/start/{provider}", h.oauth)
	r.Get("/oauth/callback/v1/{provider}", h.oauthV1Callback)
	return nil
}
