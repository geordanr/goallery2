package web

import (
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(r *chi.Mux, db *sqlx.DB) {
	h := &handler{db: db}

	r.Get("/", h.index)
}
