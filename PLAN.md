# goallery2 — Project Plan

## Status

### Done

- **`internal/config`** — TOML config file + CLI flag overrides; `db.Connect` takes typed `DBConfig`
- **`internal/gallery`** — `Store` with all DB queries for albums, photos, movies, derivatives; `itemPath` walk (prepends `albums/`, joins with `/`); `Derivative.CachePath()`; lazy-load methods on domain types
- **`internal/web`** — chi router; `GET /`, `GET /album/{id}`, `GET /photo/{id}`, `GET /photo/{id}/thumbnail`, `GET /movie/{id}`, `GET /movie/{id}/thumbnail`, `GET /files/*`; embedded HTML templates; breadcrumb nav; path-traversal-safe file serving
- **Thumbnail serving** — `Derivative.CachePath()` confirmed against real data; thumbnails and full images serving correctly
- **Movie playback** — 206 Partial Content is correct range-request behavior. Real issue: `.avi`/`.mov` not natively browser-playable. Fixed `<source type="">` placement; added download link fallback.
- **SQL layer** — Stayed with sqlx. Introduced `gallery.Reader` interface; domain types hold `Reader` instead of `*Store`; `internal/web` decoupled from concrete implementation.
- **Photo resized derivative** — Photo detail page shows resized image when available. Handler stats each candidate on disk before linking; falls back to full-size if cache file is missing (Gallery 2 sometimes writes DB records without generating the file).
- **Album cover thumbnail** — Album listing shows a 48×48 cover image per child album via a correlated subquery picking the first photo by origination timestamp. Generic `/thumbnail/{id}` route serves any item's thumbnail without knowing its entity type.
- **Prev/next navigation** — Photo detail page shows ← Previous / Next → links based on sibling order within the album. Fails gracefully if siblings can't be loaded.
- **Sort order** — Album listing supports `?sort=date|title|modified` via a sort bar; `SortOrder` type with `String()` + `orderByClause()` keeps SQL fragments hardcoded and never user-derived.
- **Movie thumbnails** — Album listing shows movie thumbnails via `/movie/{id}/thumbnail`; movie detail page sets `poster=` on the `<video>` element. `onerror="this.remove()"` handles missing thumbnails gracefully.
- **Pagination** — Album photo grid paginates at 50 per page. Page links carry sort; sort links reset to page 1. Invalid/out-of-range `?page=` silently clamps to 1.
- **Tests** — `internal/config`: full unit test suite. `internal/web`: index redirect, album/photo 404, file-serving (OK, traversal guard, not-found, directory). Photo handler: resized fallback + prev/next nav. Album handler: pagination edge cases.

---

## Next up

### 1. `internal/gallery` integration tests

Integration tests against a real MySQL instance. The Gallery 2 schema is fixed and read-only, so mocking the DB would hide real query errors.

- Use a test helper that connects with `config.DBConfig` read from env or a test config file; skip the suite if the DB is unavailable
- `TestGetRootAlbum` — ID 7, parent 0
- `TestGetAlbum` — known album ID, verify fields
- `TestChildAlbums` — known parent, expected child count / IDs
- `TestAlbumPhotos` / `TestAlbumMovies` — known album, spot-check returned items
- `TestGetPhoto` / `TestGetMovie` — known IDs, verify fields
- `TestItemPath` — known photo ID, verify reconstructed path starts with `albums/` and uses forward slashes
- `TestGetDerivatives` / `TestGetThumbnail` — known photo ID with derivatives

### 2. Flickr export / upload (`cmd/flickr_upload`)

A standalone binary that walks albums and uploads photos and movies to Flickr using the [Flickr upload API](https://www.flickr.com/services/api/upload.api.html). Items are uploaded as private. Preserve as much Gallery 2 metadata as possible (title, description/caption, tags, date taken).

Design notes:

- Reuse `gallery.Store` for all data access — no new DB queries in the upload layer
- OAuth 1.0a flow for Flickr authentication; store credentials in a local config or env vars
- Walk albums depth-first; create a matching Flickr photoset per album
- Skip already-uploaded items (track by storing Flickr IDs somewhere — a local SQLite sidecar or a flat file index)
- Respect Flickr rate limits; log progress per item
- `cmd/flickr_upload` takes `-config` (same TOML as the server) plus Flickr-specific flags (`-flickr-key`, `-flickr-secret`, `-flickr-token`, `-flickr-token-secret`)

### 3. gRPC / protobuf interface (future)

A secondary read-only interface for mobile clients. Design notes:

- Define proto messages mirroring the domain types in `internal/gallery`
- gRPC server in `cmd/grpc` (separate binary or combined with HTTP)
- Keep all data access through `gallery.Store` — no new queries in the gRPC layer
