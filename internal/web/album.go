package web

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/gallery"
)

const photosPerPage = 50

// parseSortOrder maps a URL query value ("title", "modified") to a SortOrder.
// Unknown or missing values default to SortByDate.
func parseSortOrder(s string) gallery.SortOrder {
	switch s {
	case "title":
		return gallery.SortByTitle
	case "modified":
		return gallery.SortByModified
	default:
		return gallery.SortByDate
	}
}

type breadcrumb struct {
	ID    int
	Title string
}

type albumData struct {
	Album       *gallery.Album
	ChildAlbums []gallery.Album
	Photos      []gallery.Photo
	Movies      []gallery.Movie
	Breadcrumbs []breadcrumb
	SortKey     string // current sort query param value: "date" | "title" | "modified"
	Page        int    // current 1-indexed page number
	TotalPages  int    // total number of photo pages
	PrevPage    int    // previous page number; 0 if on the first page
	NextPage    int    // next page number; 0 if on the last page
}

func (h *handler) album(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid album id", http.StatusBadRequest)
		return
	}

	album, err := h.store.GetAlbum(id)
	if err != nil {
		http.Error(w, "album not found", http.StatusNotFound)
		return
	}

	sortOrder := parseSortOrder(r.URL.Query().Get("sort"))

	childAlbums, err := album.GetChildAlbums(sortOrder)
	if err != nil {
		http.Error(w, "error loading albums", http.StatusInternalServerError)
		return
	}

	allPhotos, err := album.GetPhotos(sortOrder)
	if err != nil {
		http.Error(w, "error loading photos", http.StatusInternalServerError)
		return
	}

	movies, err := album.GetMovies(sortOrder)
	if err != nil {
		http.Error(w, "error loading movies", http.StatusInternalServerError)
		return
	}

	breadcrumbs, err := h.albumBreadcrumbs(album)
	if err != nil {
		http.Error(w, "error building breadcrumbs", http.StatusInternalServerError)
		return
	}

	// Paginate photos. Movies and child albums are small enough to show all at once.
	totalPages := max(1, (len(allPhotos)+photosPerPage-1)/photosPerPage)
	page, _ := strconv.Atoi(r.URL.Query().Get("page")) // invalid/missing → 0, clamped to 1 below
	if page < 1 || page > totalPages {
		page = 1
	}
	start := (page - 1) * photosPerPage
	end := min(start+photosPerPage, len(allPhotos))
	photos := allPhotos[start:end]

	prevPage, nextPage := page-1, page+1
	if page == 1 {
		prevPage = 0
	}
	if page == totalPages {
		nextPage = 0
	}

	data := albumData{
		Album:       album,
		ChildAlbums: childAlbums,
		Photos:      photos,
		Movies:      movies,
		Breadcrumbs: breadcrumbs,
		SortKey:     sortOrder.String(),
		Page:        page,
		TotalPages:  totalPages,
		PrevPage:    prevPage,
		NextPage:    nextPage,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := albumTmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// albumBreadcrumbs walks from the given album up to the root, returning the
// path from root to album as a slice of breadcrumbs. The current album is
// included without a link (ID = 0).
func (h *handler) albumBreadcrumbs(album *gallery.Album) ([]breadcrumb, error) {
	var crumbs []breadcrumb
	current := album

	for {
		isLeaf := current.ID == album.ID
		crumb := breadcrumb{Title: current.Title}
		if !isLeaf {
			crumb.ID = current.ID
		}
		crumbs = append(crumbs, crumb)

		if current.ParentID == 0 {
			break
		}

		parent, err := h.store.GetAlbum(current.ParentID)
		if err != nil {
			return nil, err
		}
		current = parent
	}

	// Reverse: collected leaf→root, want root→leaf.
	slices.Reverse(crumbs)
	return crumbs, nil
}
