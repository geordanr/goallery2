// Package uploader walks Gallery 2 albums and uploads photos to Flickr.
package uploader

import (
	"fmt"
	"strings"

	"github.com/geordanr/goallery2/internal/gallery"
)

// AlbumUpload describes one album and the photos within it that should be
// uploaded as a single Flickr photoset.
type AlbumUpload struct {
	// FlickrTitle is the title to use for the Flickr photoset. Child albums
	// have their ancestor titles prepended so the flat Flickr list is navigable.
	FlickrTitle string
	// SourceAlbum is the Gallery 2 album being uploaded.
	SourceAlbum gallery.Album
	// Photos is the ordered list of photos to upload from this album.
	Photos []gallery.Photo
}

// Walk collects all albums reachable from rootIDs (inclusive of each root and
// all descendants) and returns them as a flat slice of AlbumUpload, depth-first.
// Albums with no photos are still included (as empty photosets) so the Flickr
// listing mirrors the Gallery 2 structure.
//
// Child album titles are prefixed with their ancestor titles separated by " - "
// so that the hierarchy is visible in Flickr's flat photoset list.
func Walk(reader gallery.Reader, rootIDs []int) ([]AlbumUpload, error) {
	var uploads []AlbumUpload
	for _, id := range rootIDs {
		album, err := reader.GetAlbum(id)
		if err != nil {
			return nil, fmt.Errorf("loading root album %d: %w", id, err)
		}
		if err := walk(reader, *album, "", &uploads); err != nil {
			return nil, err
		}
	}
	return uploads, nil
}

func walk(reader gallery.Reader, album gallery.Album, titlePrefix string, out *[]AlbumUpload) error {
	flickrTitle := album.Title
	if flickrTitle == "" {
		flickrTitle = album.PathComponent
	}
	if titlePrefix != "" {
		flickrTitle = titlePrefix + " - " + flickrTitle
	}

	photos, err := reader.AlbumPhotos(album.ID, gallery.SortByDate)
	if err != nil {
		return fmt.Errorf("loading photos for album %d: %w", album.ID, err)
	}

	*out = append(*out, AlbumUpload{
		FlickrTitle: flickrTitle,
		SourceAlbum: album,
		Photos:      photos,
	})

	children, err := reader.ChildAlbums(album.ID, gallery.SortByDate)
	if err != nil {
		return fmt.Errorf("loading child albums of %d: %w", album.ID, err)
	}
	for _, child := range children {
		if err := walk(reader, child, flickrTitle, out); err != nil {
			return err
		}
	}
	return nil
}

// FlickrTitle returns the display title for an album given its ancestors'
// titles. albumTitle must be non-empty; walk always substitutes PathComponent
// for untitled albums before calling this function.
// Exported for use in tests and dry-run output.
func FlickrTitle(ancestors []string, albumTitle string) string {
	all := append(ancestors, albumTitle) //nolint:gocritic // intentional copy
	return strings.Join(all, " - ")
}
