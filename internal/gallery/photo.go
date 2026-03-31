package gallery

// GetDerivatives returns all derivatives (thumbnails, resized copies) of this photo.
func (p *Photo) GetDerivatives() ([]Derivative, error) {
	return p.store.derivatives(p.ID)
}

// GetThumbnail returns the thumbnail derivative for this photo.
func (p *Photo) GetThumbnail() (*Derivative, error) {
	return p.store.thumbnail(p.ID)
}

// GetResized returns all non-thumbnail derivatives for this photo (e.g. resized copies).
func (p *Photo) GetResized() ([]Derivative, error) {
	all, err := p.store.derivatives(p.ID)
	if err != nil {
		return nil, err
	}
	var resized []Derivative
	for _, d := range all {
		if d.Type != DerivativeThumbnail {
			resized = append(resized, d)
		}
	}
	return resized, nil
}

// Path returns the relative disk path of this photo.
func (p *Photo) Path() (string, error) {
	return p.store.itemPath(p.ID)
}
