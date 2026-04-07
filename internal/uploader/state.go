package uploader

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// State tracks which Gallery 2 items have already been uploaded to Flickr,
// so re-running the uploader skips completed work.
type State struct {
	// Photos maps Gallery 2 photo ID → Flickr photo ID.
	Photos map[int]string `json:"photos"`
	// Photosets maps Gallery 2 album ID → Flickr photoset ID.
	Photosets map[int]string `json:"photosets"`
}

func newState() State {
	return State{
		Photos:    make(map[int]string),
		Photosets: make(map[int]string),
	}
}

// LoadState reads state from path. If the file does not exist an empty State
// is returned without error, so the first run works without pre-creating the file.
func LoadState(path string) (State, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return newState(), nil
	}
	if err != nil {
		return State{}, fmt.Errorf("opening state file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var s State
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return State{}, fmt.Errorf("parsing state file %q: %w", path, err)
	}
	// Ensure maps are non-nil even if the JSON had null values.
	if s.Photos == nil {
		s.Photos = make(map[int]string)
	}
	if s.Photosets == nil {
		s.Photosets = make(map[int]string)
	}
	return s, nil
}

// SaveState writes state to path atomically (write to a temp file, then rename).
func SaveState(path string, s State) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("writing state file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("closing temp state file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming state file: %w", err)
	}
	return nil
}
