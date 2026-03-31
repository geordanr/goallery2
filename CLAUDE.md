# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

goallery2 is a read-only Go web application that serves image galleries originally created in the PHP-based [Gallery 2](https://sourceforge.net/projects/gallery/) application. It reads existing Gallery 2 data — images stored in a directory hierarchy on disk and metadata stored in MySQL — and presents them via HTTP. A secondary interface using gRPC/protobuf may be added for dedicated mobile app clients.

## Data Sources

- **Images**: Directory hierarchy under `data/` on disk (not committed to version control)
- **Metadata**: MySQL database originally created by Gallery 2; mysqldump files for reference are also in `data/`
- The `data/` directory is excluded from version control — it is reference-only

## Gallery 2 Schema

Gallery 2 uses **table-per-class inheritance** — every entity has a row in `g2_Entity` and additional rows in type-specific tables, all sharing the same `g_id`.

### Entity types in this database
| g_entityType | Count | Tables joined |
|---|---|---|
| GalleryAlbumItem | 347 | Entity + Item + FileSystemEntity + AlbumItem + ChildEntity |
| GalleryPhotoItem | 5314 | Entity + Item + FileSystemEntity + DataItem + PhotoItem + ChildEntity |
| GalleryMovieItem | 98 | Entity + Item + FileSystemEntity + DataItem + MovieItem + ChildEntity |
| GalleryDerivativeImage | 9508 | Entity + Derivative + DerivativeImage |
| GalleryComment | 209 | Entity + Comment |

### Key relationships
- **`g2_ChildEntity`**: maps every item/album to its `g_parentId` (albums and photos alike). Root album has `g_id = 7`, `g_parentId = 0`.
- **Full disk path**: walk `g2_ChildEntity` upward from item to root, collect `g_pathComponent` from `g2_FileSystemEntity` at each level, join with `/`. The root album (ID 7) has `g_pathComponent = NULL`.
- **Derivatives** (`g_derivativeType = 1` = thumbnail, `2` = resized): do NOT have `g2_FileSystemEntity` entries. Their cache file paths follow Gallery 2's cache layout — to be confirmed once image files are available.
- `g2_Derivative.g_derivativeSourceId` → source photo/movie ID.
- `g2_Derivative.g_derivativeOperations`: operation string, e.g. `thumbnail|150`.

### Go domain types
Defined in `internal/gallery/types.go`: `Album`, `Photo`, `Movie`, `Derivative`.

## Architecture (planned)

- **HTTP server**: Primary interface for web browsers
- **MySQL**: Backend for all gallery metadata (albums, items, captions, users, etc.)
- **gRPC/protobuf**: Optional secondary interface for mobile clients
- The app is read-only with respect to Gallery 2 data — it does not modify the MySQL schema or images

## Common Commands

```sh
go build ./...                          # build all packages
go run ./cmd/server                     # run the server
go test ./...                           # run all tests
go test ./internal/web/...              # run tests for a single package
go vet ./...                            # static analysis
```

## Configuration

The server is configured via environment variables (all have defaults for local dev):

| Variable        | Default     | Description          |
|-----------------|-------------|----------------------|
| `MYSQL_USER`    | `root`      | MySQL username       |
| `MYSQL_PASSWORD`| _(empty)_   | MySQL password       |
| `MYSQL_HOST`    | `127.0.0.1` | MySQL host           |
| `MYSQL_PORT`    | `3306`      | MySQL port           |
| `MYSQL_DBNAME`  | `gallery2`  | MySQL database name  |
