package config

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/dghubble/oauth1"
)

// Config holds all runtime configuration for goallery2.
type Config struct {
	Server ServerConfig `toml:"server"`
	DB     DBConfig     `toml:"db"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr         string                         `toml:"addr"`
	DataDir      string                         `toml:"data_dir"`
	OAuthConfigs map[string]OAuthProviderConfig `toml:"oauth"`
}

// DBConfig holds MySQL connection settings.
type DBConfig struct {
	User     string `toml:"user"`
	Password string `toml:"password"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
}

// OAuthProviderConfig stores URLs and  our consumer key and secret for a given provider.
type OAuthProviderConfig struct {
	Key             string `toml:"key"`
	Secret          string `toml:"secret"`
	Callback        string `toml:"callback"`
	AccessTokenURL  string `toml:"access_token_url"`
	AuthorizeURL    string `toml:"authorize_url"`
	RequestTokenURL string `toml:"request_token_url"`
}

// defaults returns a Config populated with sensible local-dev defaults.
func defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr: ":8080",
			// DataDir has no default — it must be set explicitly via config file
			// or the -data-dir flag, since it is installation-specific.
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
			return Config{}, fmt.Errorf("could not open config file %q: %w\n  check that the path passed to -config exists and is readable", path, err)
		}
		defer func() { _ = f.Close() }()
		if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("could not parse config file %q: %w\n  verify the file is valid TOML and all keys are spelled correctly", path, err)
		}
	}

	overrides.apply(&cfg)
	return cfg, nil
}

// Overrides holds CLI flag values. Zero values mean "not set; keep file/default value."
type Overrides struct {
	Addr    string
	DataDir string
	DBUser  string
	DBPass  string
	DBHost  string
	DBPort  int
	DBName  string
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

// BaseURL returns the absolute HTTP base URL for this server derived from Addr.
// Unspecified or wildcard hosts (e.g. ":8080", "0.0.0.0:8080") resolve to localhost.
func (s ServerConfig) BaseURL() string {
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return "http://" + s.Addr
	}
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	if port == "" {
		return "http://" + host
	}
	return "http://" + host + ":" + port
}

// GetOAuthConfig creates an oauth1.Config for the given provider.
func (c *Config) GetOAuthConfig(provider string) (oauth1.Config, error) {
	p, ok := c.Server.OAuthConfigs[provider]
	if !ok {
		return oauth1.Config{}, fmt.Errorf("provider not found in config: %s", provider)
	}

	oauthConfig := oauth1.Config{
		ConsumerKey:    p.Key,
		ConsumerSecret: p.Secret,
		CallbackURL:    c.Server.BaseURL() + p.Callback,
		Endpoint: oauth1.Endpoint{
			AccessTokenURL:  p.AccessTokenURL,
			AuthorizeURL:    p.AuthorizeURL,
			RequestTokenURL: p.RequestTokenURL,
		},
	}

	return oauthConfig, nil
}
