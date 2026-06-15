# goallery2

A read-only Go web application that serves photo galleries originally created with [Gallery 2](https://sourceforge.net/projects/gallery/), a PHP-based gallery application that was widely used in the early 2000s.

## Why this exists

Gallery 2 is long unmaintained. If you ran it and still have the data — images on disk in Gallery 2's directory layout and metadata in a MySQL database — goallery2 lets you browse those galleries in a modern browser without running PHP or touching the original data. It also provides a browser-based Flickr export flow for migrating photos off the local server.

goallery2 is read-only: it never writes to the Gallery 2 database or modifies any image files.

## Prerequisites

- Go 1.21+
- MySQL with the original Gallery 2 database imported
- The Gallery 2 data directory (`g2data/`) accessible on disk

## Building

```sh
go build ./...
```

This produces two binaries:

| Binary | Purpose |
|---|---|
| `cmd/server` | HTTP server for browsing galleries |
| `cmd/flickr_upload` | CLI tool for uploading albums to Flickr |

## Configuration

The server reads a TOML config file. Minimal example:

```toml
[server]
addr     = ":8080"
data_dir = "/path/to/g2data"

[db]
user     = "root"
password = ""
host     = "127.0.0.1"
port     = 3306
name     = "gallery2"
```

Any value can be overridden with a CLI flag. Run `cmd/server -help` for the full list.

## Running the server

```sh
go run ./cmd/server -config /path/to/goallery2.toml
```

Browse to `http://localhost:8080`.

## Flickr export

### Setup

You must first register a Flickr app at [flickr.com/services/apps](https://www.flickr.com/services/apps) to obtain an API key and secret. Add the following to your config file:

```toml
[server.oauth.flickr]
key               = "<api key>"
secret            = "<api secret>"
access_token_url  = "https://www.flickr.com/services/oauth/access_token"
authorize_url     = "https://www.flickr.com/services/oauth/authorize"
request_token_url = "https://www.flickr.com/services/oauth/request_token"
callback          = "/oauth/callback/v1/flickr"
```

### Browser-based (recommended)

With Flickr OAuth configured, an **Upload to Flickr** link appears at the bottom of each album page. Clicking it walks you through OAuth (once) and then shows a confirmation page before starting the upload. Progress streams to the browser as each photo is processed; Flickr photoset links appear on completion.

Upload state is stored in `flickr_upload.json` (configurable via `flickr_state_path` in the config). Re-running an upload skips already-uploaded photos.

### CLI

```sh
go run ./cmd/flickr_upload \
  -config /path/to/goallery2.toml \
  -flickr-key <key> \
  -flickr-secret <secret> \
  -flickr-token <token> \
  -flickr-token-secret <token-secret> \
  <album_id> [album_id ...]
```

Useful flags: `-dryrun` (print plan without uploading), `-check` (verify credentials only), `-state` (path to progress file).

## Development

```sh
go test ./...           # run tests
go vet ./...            # static analysis
golangci-lint run ./... # full lint
```

---

`CLAUDE.md` contains detailed project context, the Gallery 2 schema, disk layout notes, and development guidance.
