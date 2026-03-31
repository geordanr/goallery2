package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	albumTmpl = template.Must(template.ParseFS(templateFS, "templates/album.html"))
)
