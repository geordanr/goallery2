package main

import (
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/db"
	"github.com/geordanr/goallery2/internal/web"
)

func main() {
	fs := flag.CommandLine
	configPath := fs.String("config", "", "path to TOML config file")
	debug := fs.Bool("debug", false, "enable debug logging")
	overrides := config.RegisterFlags(fs)
	flag.Parse()

	logLevel := new(slog.LevelVar) // defaults to INFO
	if *debug {
		logLevel.Set(slog.LevelDebug)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	if *debug {
		slog.Info("debug logging enabled")
		// Route chi's access log through slog so all output is on the same
		// stream (stderr) in the same format.
		middleware.DefaultLogger = middleware.RequestLogger(&middleware.DefaultLogFormatter{
			Logger:  slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo),
			NoColor: true,
		})
	}

	cfg, err := config.Load(*configPath, *overrides)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	database, err := db.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	web.RegisterRoutes(r, database, cfg)

	slog.Info("listening", "addr", cfg.Server.Addr)
	if err := http.ListenAndServe(cfg.Server.Addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
