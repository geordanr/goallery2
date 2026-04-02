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

---

## Next up

### 1. Movie thumbnails

Investigate whether derivatives exist for movies in the DB. If so, serve them through the existing derivative path; if not, decide on a fallback.

### 2. Album thumbnail / cover image

Gallery 2 albums can have a highlight image. Add `g2_AlbumItem.g_highlightId` to the album query and show a cover thumbnail on the album listing page.

### 3. Previous / next links in photo view

Add previous/next navigation links on the photo detail page. Requires the store to return an ordered list of photos for an album so adjacent IDs can be looked up.

### 4. Sort order

Extend album/photo listings to support sorting by title (current default), creation date, and last-modified date. Build on the ordered list introduced in §3.

### 5. Pagination

Show N items per page on album listings. Depends on stable sort order (§4) so page boundaries are consistent.

### 6. Tests

#### `internal/config`

Unit tests; no external dependencies.

- `TestDefaults` — `Load("", Overrides{})` returns expected default values
- `TestTOMLFile` — write a temp TOML file, verify fields are loaded correctly
- `TestFlagOverrides` — TOML file sets values, overrides replace specific fields; zero-value overrides do not clobber file values
- `TestMissingFile` — non-empty path to a nonexistent file returns an error

#### `internal/gallery`

Integration tests against a real MySQL instance. The Gallery 2 schema is fixed and read-only, so mocking the DB would hide real query errors.

- Use a test helper that connects with `config.DBConfig` read from env or a test config file; skip the suite if the DB is unavailable
- `TestGetRootAlbum` — ID 7, parent 0
- `TestGetAlbum` — known album ID, verify fields
- `TestChildAlbums` — known parent, expected child count / IDs
- `TestAlbumPhotos` / `TestAlbumMovies` — known album, spot-check returned items
- `TestGetPhoto` / `TestGetMovie` — known IDs, verify fields
- `TestItemPath` — known photo ID, verify reconstructed path starts with `albums/` and uses forward slashes
- `TestGetDerivatives` / `TestGetThumbnail` — known photo ID with derivatives

#### `internal/web`

HTTP handler tests using `net/http/httptest`. The handlers depend on `gallery.Reader`, so test fakes implement the interface without a real DB.

- `TestIndexRedirect` — `GET /` → 302 to `/album/7`
- `TestAlbumHandler` — fake reader returns a known album; verify status 200, template renders title and child links
- `TestAlbumNotFound` — reader returns `sql.ErrNoRows`-wrapped error; verify 404
- `TestPhotoHandler` / `TestMovieHandler` — similar shape
- `TestFileRouteMatches` — verify chi `/*` wildcard matches single and multi-segment paths ✓ (done)
- `TestServeFile_OK` — temp file under a temp data dir; verify 200 and correct content
- `TestServeFile_Traversal` — paths like `../secret`; verify 403
- `TestServeFile_NotFound` — nonexistent path; verify 404
- `TestServeFile_Directory` — path resolves to a directory; verify 404

### 7. Flickr export / upload (`cmd/flickr_upload`)

A standalone binary that walks albums and uploads photos and movies to Flickr using the [Flickr upload API](https://www.flickr.com/services/api/upload.api.html). Items are uploaded as private. Preserve as much Gallery 2 metadata as possible (title, description/caption, tags, date taken).

Design notes:

- Reuse `gallery.Store` for all data access — no new DB queries in the upload layer
- OAuth 1.0a flow for Flickr authentication; store credentials in a local config or env vars
- Walk albums depth-first; create a matching Flickr photoset per album
- Skip already-uploaded items (track by storing Flickr IDs somewhere — a local SQLite sidecar or a flat file index)
- Respect Flickr rate limits; log progress per item
- `cmd/flickr_upload` takes `-config` (same TOML as the server) plus Flickr-specific flags (`-flickr-key`, `-flickr-secret`, `-flickr-token`, `-flickr-token-secret`)

### 8. gRPC / protobuf interface (future)

A secondary read-only interface for mobile clients. Design notes:

- Define proto messages mirroring the domain types in `internal/gallery`
- gRPC server in `cmd/grpc` (separate binary or combined with HTTP)
- Keep all data access through `gallery.Store` — no new queries in the gRPC layer
