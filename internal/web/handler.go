package web

import (
	"net/http"

	"github.com/jmoiron/sqlx"
)

type handler struct {
	db *sqlx.DB
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("goallery2"))
}
