package gallery

import "slices"

// GetDerivatives returns all derivatives (thumbnails, resized copies) of this movie.
func (m *Movie) GetDerivatives() ([]Derivative, error) {
	return m.store.derivatives(m.ID)
}

// GetThumbnail returns the thumbnail derivative for this movie.
func (m *Movie) GetThumbnail() (*Derivative, error) {
	return m.store.thumbnail(m.ID)
}

// GetResized returns all non-thumbnail derivatives for this movie.
func (m *Movie) GetResized() ([]Derivative, error) {
	all, err := m.store.derivatives(m.ID)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(d Derivative) bool {
		return d.Type == DerivativeThumbnail
	}), nil
}

// Path returns the relative disk path of this movie.
func (m *Movie) Path() (string, error) {
	return m.store.itemPath(m.ID)
}
