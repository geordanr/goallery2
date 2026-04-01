package web

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request) {
	// Resolve the data directory to an absolute path so that symlinks and
	// relative paths in the config cannot bypass the traversal check below.
	absDataDir, err := filepath.Abs(h.config.Server.DataDir)
	if err != nil {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		return
	}

	// {path...} captures everything after /files/ as a single value, including
	// slashes — e.g. /files/foo/bar/image.jpg → "foo/bar/image.jpg".
	rawPath := chi.URLParam(r, "path")

	// Join and clean to collapse any ".." components, then confirm the result
	// still lives under absDataDir. This prevents path traversal attacks.
	target := filepath.Clean(filepath.Join(absDataDir, filepath.FromSlash(rawPath)))

	slog.Debug("serveFile", "raw_path", rawPath, "target", target)

	if !strings.HasPrefix(target, absDataDir+string(filepath.Separator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Stat before serving so we can return a clean 404 for missing files
	// without leaking filesystem details in the error message.
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("serveFile: not found", "target", target)
			http.Error(w, "not found", http.StatusNotFound)
		} else {
			slog.Debug("serveFile: stat error", "target", target, "err", err)
			http.Error(w, "error accessing file", http.StatusInternalServerError)
		}
		return
	}
	if info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, target)
}
