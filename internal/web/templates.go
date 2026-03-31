package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	albumTmpl = template.Must(template.ParseFS(templateFS, "templates/album.html"))
	photoTmpl = template.Must(template.ParseFS(templateFS, "templates/photo.html"))
	movieTmpl = template.Must(template.ParseFS(templateFS, "templates/movie.html"))
)
