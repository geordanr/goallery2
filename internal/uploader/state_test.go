package uploader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadState_Missing(t *testing.T) {
	s, err := LoadState(filepath.Join(t.TempDir(), "does_not_exist.json"))
	if err != nil {
		t.Fatalf("LoadState on missing file: %v", err)
	}
	if s.Photos == nil || s.Photosets == nil {
		t.Error("maps should be non-nil even when file is missing")
	}
}

func TestSaveAndLoadState_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := State{
		Photos:    map[int]string{1: "f1", 2: "f2"},
		Photosets: map[int]string{10: "ps10"},
	}
	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.Photos[1] != "f1" || loaded.Photos[2] != "f2" {
		t.Errorf("Photos mismatch: %v", loaded.Photos)
	}
	if loaded.Photosets[10] != "ps10" {
		t.Errorf("Photosets mismatch: %v", loaded.Photosets)
	}
}

func TestLoadState_NullMaps(t *testing.T) {
	// A state file with JSON null for both maps should produce non-nil maps.
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"photos":null,"photosets":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if s.Photos == nil {
		t.Error("Photos should be non-nil after loading null JSON")
	}
	if s.Photosets == nil {
		t.Error("Photosets should be non-nil after loading null JSON")
	}
}
