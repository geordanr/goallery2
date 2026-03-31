package gallery

import "fmt"

// CachePath returns the relative path of this derivative's cache file under
// the Gallery 2 data directory.
//
// Gallery 2 shards cache files by the first two digits of the derivative ID:
//
//	cache/derivative/{id[0]}/{id[1]}/{id}.dat
//
// For example, derivative 10047 → cache/derivative/1/0/10047.dat.
// The files are JPEG images regardless of the .dat extension; callers should
// use MimeType when setting Content-Type.
func (d Derivative) CachePath() string {
	s := fmt.Sprintf("%02d", d.ID)
	return fmt.Sprintf("cache/derivative/%c/%c/%d.dat", s[0], s[1], d.ID)
}
