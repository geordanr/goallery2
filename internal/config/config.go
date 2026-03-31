package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds all runtime configuration for goallery2.
type Config struct {
	Server ServerConfig `toml:"server"`
	DB     DBConfig     `toml:"db"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr    string `toml:"addr"`
	DataDir string `toml:"data_dir"`
}

// DBConfig holds MySQL connection settings.
type DBConfig struct {
	User     string `toml:"user"`
	Password string `toml:"password"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
}

// defaults returns a Config populated with sensible local-dev defaults.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr:    ":8080",
			DataDir: "./data",
		},
		DB: DBConfig{
			User: "root",
			Host: "127.0.0.1",
			Port: 3306,
			Name: "gallery2",
		},
	}
}

// Load reads configuration from the TOML file at path (if non-empty), then
// applies any non-zero flag overrides.
func Load(path string, overrides Overrides) (Config, error) {
	cfg := defaults()

	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return Config{}, fmt.Errorf("open config file: %w", err)
		}
		defer f.Close()
		if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("parse config file: %w", err)
		}
	}

	overrides.apply(&cfg)
	return cfg, nil
}

// Overrides holds CLI flag values. Zero values mean "not set; keep file/default value."
type Overrides struct {
	Addr     string
	DataDir  string
	DBUser   string
	DBPass   string
	DBHost   string
	DBPort   int
	DBName   string
}

func (o Overrides) apply(cfg *Config) {
	if o.Addr != "" {
		cfg.Server.Addr = o.Addr
	}
	if o.DataDir != "" {
		cfg.Server.DataDir = o.DataDir
	}
	if o.DBUser != "" {
		cfg.DB.User = o.DBUser
	}
	if o.DBPass != "" {
		cfg.DB.Password = o.DBPass
	}
	if o.DBHost != "" {
		cfg.DB.Host = o.DBHost
	}
	if o.DBPort != 0 {
		cfg.DB.Port = o.DBPort
	}
	if o.DBName != "" {
		cfg.DB.Name = o.DBName
	}
}

// RegisterFlags registers CLI flags onto fs and returns an Overrides that will
// be populated when fs is parsed.
func RegisterFlags(fs *flag.FlagSet) *Overrides {
	o := &Overrides{}
	fs.StringVar(&o.Addr, "addr", "", "listen address (overrides config file)")
	fs.StringVar(&o.DataDir, "data-dir", "", "path to Gallery 2 album data (overrides config file)")
	fs.StringVar(&o.DBUser, "db-user", "", "MySQL user (overrides config file)")
	fs.StringVar(&o.DBPass, "db-password", "", "MySQL password (overrides config file)")
	fs.StringVar(&o.DBHost, "db-host", "", "MySQL host (overrides config file)")
	fs.IntVar(&o.DBPort, "db-port", 0, "MySQL port (overrides config file)")
	fs.StringVar(&o.DBName, "db-name", "", "MySQL database name (overrides config file)")
	return o
}
