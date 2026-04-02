package gallery

// GetChildAlbums returns all direct child albums of this album in the given order.
func (a *Album) GetChildAlbums(sort SortOrder) ([]Album, error) {
	return a.reader.ChildAlbums(a.ID, sort)
}

// GetPhotos returns all photos in this album in the given order.
func (a *Album) GetPhotos(sort SortOrder) ([]Photo, error) {
	return a.reader.AlbumPhotos(a.ID, sort)
}

// GetMovies returns all movies in this album in the given order.
func (a *Album) GetMovies(sort SortOrder) ([]Movie, error) {
	return a.reader.AlbumMovies(a.ID, sort)
}

// Path returns the relative disk path of this album.
func (a *Album) Path() (string, error) {
	return a.reader.ItemPath(a.ID)
}
