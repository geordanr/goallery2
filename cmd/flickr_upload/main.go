package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"

	"github.com/geordanr/goallery2/internal/config"
	"github.com/geordanr/goallery2/internal/db"
	"github.com/geordanr/goallery2/internal/flickr"
	"github.com/geordanr/goallery2/internal/gallery"
	"github.com/geordanr/goallery2/internal/uploader"
)

func main() {
	fs := flag.CommandLine
	configPath := fs.String("config", "", "path to TOML config file")
	flickrKey := fs.String("flickr-key", "", "Flickr API key")
	flickrSecret := fs.String("flickr-secret", "", "Flickr API secret")
	flickrToken := fs.String("flickr-token", "", "Flickr OAuth access token")
	flickrTokenSecret := fs.String("flickr-token-secret", "", "Flickr OAuth token secret")
	statePath := fs.String("state", "flickr_upload.json", "path to upload progress file")
	dryRun := fs.Bool("dryrun", false, "print planned actions without uploading anything")
	overrides := config.RegisterFlags(fs)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})))

	albumIDs, err := parseAlbumIDs(flag.Args())
	if err != nil {
		log.Fatalf("invalid album IDs: %v", err)
	}
	if len(albumIDs) == 0 {
		log.Fatal("usage: flickr_upload [flags] <album_id> [album_id ...]")
	}

	cfg, err := config.Load(*configPath, *overrides)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	if cfg.Server.DataDir == "" {
		log.Fatal("data_dir is not set; specify it in the config file under [server] or with the -data-dir flag")
	}

	if !*dryRun {
		if *flickrKey == "" || *flickrSecret == "" || *flickrToken == "" || *flickrTokenSecret == "" {
			log.Fatal("all four Flickr credential flags are required: -flickr-key, -flickr-secret, -flickr-token, -flickr-token-secret")
		}
	}

	database, err := db.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	store := gallery.NewStore(database)

	state, err := uploader.LoadState(*statePath)
	if err != nil {
		log.Fatalf("failed to load state file %q: %v", *statePath, err)
	}

	uploads, err := uploader.Walk(store, albumIDs)
	if err != nil {
		log.Fatalf("failed to walk albums: %v", err)
	}

	slog.Info("upload plan",
		"albums", len(uploads),
		"dry_run", *dryRun,
	)

	var client uploader.FlickrClient
	if !*dryRun {
		client = flickr.NewClient(flickr.Credentials{
			APIKey:      *flickrKey,
			APISecret:   *flickrSecret,
			Token:       *flickrToken,
			TokenSecret: *flickrTokenSecret,
		})
	}

	u := uploader.New(client, state, *statePath, cfg.Server.DataDir, *dryRun)
	if err := u.Run(uploads); err != nil {
		log.Fatalf("upload failed: %v", err)
	}

	slog.Info("done")
}

func parseAlbumIDs(args []string) ([]int, error) {
	ids := make([]int, 0, len(args))
	for _, s := range args {
		id, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("parsing album ID %q: %w", s, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
