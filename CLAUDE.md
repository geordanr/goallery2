# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

goallery2 is a read-only Go web application that serves image galleries originally created in the PHP-based [Gallery 2](https://sourceforge.net/projects/gallery/) application. It reads existing Gallery 2 data — images stored in a directory hierarchy on disk and metadata stored in MySQL — and presents them via HTTP. A secondary interface using gRPC/protobuf may be added for dedicated mobile app clients.

## Data Sources

- **Images**: Directory hierarchy under `data/g2data/` on disk (not committed to version control)
- **Metadata**: MySQL database originally created by Gallery 2; mysqldump files for reference are also in `data/`
- The `data/` directory is excluded from version control — it is reference-only

## Disk Layout

Gallery 2 stores all data under a single root directory (`g2data/` in this installation):

```text
g2data/
  albums/          # photo/movie/album files; mirrors the ChildEntity hierarchy
  cache/
    derivative/    # thumbnail and resized image cache
      {d[0]}/      # first digit of derivative ID
        {d[1]}/    # second digit of derivative ID
          {id}.dat # JPEG file regardless of .dat extension
```

- **Album/photo/movie paths**: walk `g2_ChildEntity` upward from item to root, collect `g_pathComponent` values, join with `/`, prepend `albums/`. The root album (ID 7) has `g_pathComponent = NULL` and is not included in the path.
- **Derivative cache paths**: `cache/derivative/{id[0]}/{id[1]}/{id}.dat` — e.g. ID 10047 → `cache/derivative/1/0/10047.dat`. IDs < 10 are zero-padded to two digits for the shard dirs. Implemented in `Derivative.CachePath()`.
- Derivative `.dat` files are JPEG images; serve with `g2_Derivative.g_mimeType` from the DB, not the filename extension.

## Gallery 2 Schema

Gallery 2 uses **table-per-class inheritance** — every entity has a row in `g2_Entity` and additional rows in type-specific tables, all sharing the same `g_id`.

### Entity types in this database

| g_entityType | Count | Tables joined |
| --- | --- | --- |
| GalleryAlbumItem | 347 | Entity + Item + FileSystemEntity + AlbumItem + ChildEntity |
| GalleryPhotoItem | 5314 | Entity + Item + FileSystemEntity + DataItem + PhotoItem + ChildEntity |
| GalleryMovieItem | 98 | Entity + Item + FileSystemEntity + DataItem + MovieItem + ChildEntity |
| GalleryDerivativeImage | 9508 | Entity + Derivative + DerivativeImage |
| GalleryComment | 209 | Entity + Comment |

### Key relationships

- **`g2_ChildEntity`**: maps every item/album to its `g_parentId` (albums and photos alike). Root album has `g_id = 7`, `g_parentId = 0`.
- **Full disk path**: walk `g2_ChildEntity` upward from item to root, collect `g_pathComponent` from `g2_FileSystemEntity` at each level, join with `/`, prepend `albums/`. See `Store.itemPath()`.
- **Derivatives** (`g_derivativeType = 1` = thumbnail, `2` = resized): do NOT have `g2_FileSystemEntity` entries. Cache paths are computed by `Derivative.CachePath()`.
- `g2_Derivative.g_derivativeSourceId` → source photo/movie ID.
- `g2_Derivative.g_derivativeOperations`: operation string, e.g. `thumbnail|150`.

### Go domain types

Defined in `internal/gallery/types.go`: `Album`, `Photo`, `Movie`, `Derivative`.

## Architecture

- **HTTP server**: Primary interface for web browsers (`cmd/server`)
- **MySQL**: Backend for all gallery metadata (albums, items, captions, users, etc.)
- **gRPC/protobuf**: Optional secondary interface for mobile clients (future)
- The app is read-only with respect to Gallery 2 data — it does not modify the MySQL schema or images
- All DB query logic lives in `internal/gallery` as `gallery.Store` — web handlers call Store methods, never raw SQL

## Common Commands

```sh
go build ./...                                              # build all packages
go run ./cmd/server -config data/goallery2.toml            # run the server
go run ./cmd/server -config data/goallery2.toml -debug     # run with debug logging
go test ./...                                               # run all tests
go test ./internal/web/...                                  # run tests for a single package
go vet ./...                                                # static analysis
golangci-lint run ./...                                     # full lint (see .golangci.yml)
golangci-lint fmt ./...                                     # auto-fix formatting
```

## Configuration

The server is configured via a TOML file passed with `-config`. CLI flags override file values.

```toml
[server]
addr     = ":8080"       # listen address
data_dir = ""            # absolute path to the Gallery 2 data directory (g2data)

[db]
user     = "root"
password = ""
host     = "127.0.0.1"
port     = 3306
name     = "gallery2"
```

Available CLI flags: `-config`, `-addr`, `-data-dir`, `-db-user`, `-db-password`, `-db-host`, `-db-port`, `-db-name`, `-debug`.

## Chi Routing Notes

- Multi-segment wildcard routes use bare-star syntax: `r.Get("/files/*", handler)`
- The wildcard value is retrieved with `chi.URLParam(r, "*")` — not `"path"` or the segment name
- `{name...}` and `{*name}` only match a single path segment and should not be used for file-serving routes
