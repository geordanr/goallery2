package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load("", Overrides{})
	if err != nil {
		t.Fatalf("Load with empty path: %v", err)
	}

	if cfg.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, ":8080")
	}
	if cfg.Server.DataDir != "" {
		t.Errorf("Server.DataDir = %q, want empty", cfg.Server.DataDir)
	}
	if cfg.DB.User != "root" {
		t.Errorf("DB.User = %q, want %q", cfg.DB.User, "root")
	}
	if cfg.DB.Host != "127.0.0.1" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "127.0.0.1")
	}
	if cfg.DB.Port != 3306 {
		t.Errorf("DB.Port = %d, want 3306", cfg.DB.Port)
	}
	if cfg.DB.Name != "gallery2" {
		t.Errorf("DB.Name = %q, want %q", cfg.DB.Name, "gallery2")
	}
}

func TestTOMLFile(t *testing.T) {
	content := `
[server]
addr     = ":9000"
data_dir = "/srv/gallery"

[db]
user     = "guser"
password = "secret"
host     = "db.example.com"
port     = 3307
name     = "mydb"
`
	path := filepath.Join(t.TempDir(), "goallery2.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path, Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Addr != ":9000" {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, ":9000")
	}
	if cfg.Server.DataDir != "/srv/gallery" {
		t.Errorf("Server.DataDir = %q, want %q", cfg.Server.DataDir, "/srv/gallery")
	}
	if cfg.DB.User != "guser" {
		t.Errorf("DB.User = %q, want %q", cfg.DB.User, "guser")
	}
	if cfg.DB.Password != "secret" {
		t.Errorf("DB.Password = %q, want %q", cfg.DB.Password, "secret")
	}
	if cfg.DB.Host != "db.example.com" {
		t.Errorf("DB.Host = %q, want %q", cfg.DB.Host, "db.example.com")
	}
	if cfg.DB.Port != 3307 {
		t.Errorf("DB.Port = %d, want 3307", cfg.DB.Port)
	}
	if cfg.DB.Name != "mydb" {
		t.Errorf("DB.Name = %q, want %q", cfg.DB.Name, "mydb")
	}
}

func TestFlagOverrides(t *testing.T) {
	content := `
[server]
addr = ":9000"

[db]
host = "db.example.com"
port = 3307
`
	path := filepath.Join(t.TempDir(), "goallery2.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	overrides := Overrides{
		Addr:   ":7777", // override file value
		DBPort: 0,       // zero — must NOT clobber file value
	}
	cfg, err := Load(path, overrides)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Addr != ":7777" {
		t.Errorf("Server.Addr = %q, want %q (override not applied)", cfg.Server.Addr, ":7777")
	}
	if cfg.DB.Host != "db.example.com" {
		t.Errorf("DB.Host = %q, want %q (file value overwritten)", cfg.DB.Host, "db.example.com")
	}
	if cfg.DB.Port != 3307 {
		t.Errorf("DB.Port = %d, want 3307 (zero override clobbered file value)", cfg.DB.Port)
	}
}

func TestMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"), Overrides{})
	if err == nil {
		t.Fatal("Load with nonexistent path: want error, got nil")
	}
}
