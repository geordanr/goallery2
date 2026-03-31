package gallery

// GetChildAlbums returns all direct child albums of this album.
func (a *Album) GetChildAlbums() ([]Album, error) {
	return a.store.childAlbums(a.ID)
}

// GetPhotos returns all photos in this album.
func (a *Album) GetPhotos() ([]Photo, error) {
	return a.store.albumPhotos(a.ID)
}

// GetMovies returns all movies in this album.
func (a *Album) GetMovies() ([]Movie, error) {
	return a.store.albumMovies(a.ID)
}

// Path returns the relative disk path of this album.
func (a *Album) Path() (string, error) {
	return a.store.itemPath(a.ID)
}
