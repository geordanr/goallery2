package gallery

import (
	"cmp"
	"slices"
)

// GetDerivatives returns all derivatives (thumbnails, resized copies) of this photo.
func (p *Photo) GetDerivatives() ([]Derivative, error) {
	return p.reader.Derivatives(p.ID)
}

// GetThumbnail returns the thumbnail derivative for this photo.
func (p *Photo) GetThumbnail() (*Derivative, error) {
	return p.reader.Thumbnail(p.ID)
}

// GetResized returns all non-thumbnail derivatives for this photo (e.g. resized copies),
// ordered by Derivative.Order ascending. Store.Derivatives already orders by
// g_derivativeOrder in SQL; the sort here makes the guarantee explicit for any
// Reader implementation that does not.
func (p *Photo) GetResized() ([]Derivative, error) {
	all, err := p.reader.Derivatives(p.ID)
	if err != nil {
		return nil, err
	}
	// slices.DeleteFunc modifies all in place; result shares all's backing
	// array. This is safe because all is not retained after this function returns.
	result := slices.DeleteFunc(all, func(d Derivative) bool {
		return d.Type == DerivativeThumbnail
	})
	slices.SortFunc(result, func(a, b Derivative) int {
		return cmp.Compare(a.Order, b.Order)
	})
	return result, nil
}

// DisplayTitle returns the photo's title if set, otherwise its filename.
func (p *Photo) DisplayTitle() string {
	if p.Title != "" {
		return p.Title
	}
	return p.PathComponent
}

// Path returns the relative disk path of this photo.
func (p *Photo) Path() (string, error) {
	return p.reader.ItemPath(p.ID)
}
