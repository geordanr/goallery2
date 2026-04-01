package gallery

// GetChildAlbums returns all direct child albums of this album.
func (a *Album) GetChildAlbums() ([]Album, error) {
	return a.reader.ChildAlbums(a.ID)
}

// GetPhotos returns all photos in this album.
func (a *Album) GetPhotos() ([]Photo, error) {
	return a.reader.AlbumPhotos(a.ID)
}

// GetMovies returns all movies in this album.
func (a *Album) GetMovies() ([]Movie, error) {
	return a.reader.AlbumMovies(a.ID)
}

// Path returns the relative disk path of this album.
func (a *Album) Path() (string, error) {
	return a.reader.ItemPath(a.ID)
}
