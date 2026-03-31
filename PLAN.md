# goallery2 — Project Plan

## Status

### Done
- **`internal/config`** — TOML config file + CLI flag overrides; `db.Connect` takes typed `DBConfig`
- **`internal/gallery`** — `Store` with all DB queries for albums, photos, movies, derivatives; `itemPath` walk; lazy-load methods on domain types
- **`internal/web`** — chi router; `GET /`, `GET /album/{id}`, `GET /photo/{id}`, `GET /movie/{id}`, `GET /files/{path...}`; embedded HTML templates; breadcrumb nav; path-traversal-safe file serving

### Blocked / deferred
- **Thumbnail serving** (`GET /photo/{id}/thumbnail`) — derivative cache file paths follow Gallery 2's own layout, which must be confirmed against the actual data directory before implementing. The album template currently emits these links but they will 404 until this is resolved.

---

## Next up

### 1. Derivative cache path resolution
Once the data directory is available, inspect it to determine Gallery 2's cache layout (likely `cache/` under the data root, with paths derived from the derivative ID). Implement:
- A `Derivative.CachePath(dataDir string) string` helper (or equivalent on `Store`)
- `GET /photo/{id}/thumbnail` handler that resolves the cache path and serves the file via the same traversal-safe logic as `serveFile`
- Update the album template to use the real thumbnail URL

### 2. Photo thumbnail on the photo detail page
The photo detail page currently shows the full-size image directly. Once thumbnails are working, add a resized derivative to the photo detail view if one exists.

### 3. Album thumbnail / cover image
Gallery 2 albums can have a highlight image. Add `g2_AlbumItem.g_highlightId` to the album query and show a cover thumbnail on the album listing page.

### 4. gRPC / protobuf interface (future)
A secondary read-only interface for mobile clients. Design notes:
- Define proto messages mirroring the domain types in `internal/gallery`
- gRPC server in `cmd/grpc` (separate binary or combined with HTTP)
- Keep all data access through `gallery.Store` — no new queries in the gRPC layer

---

## Tests

### `internal/config`
Unit tests; no external dependencies.
- `TestDefaults` — `Load("", Overrides{})` returns expected default values
- `TestTOMLFile` — write a temp TOML file, verify fields are loaded correctly
- `TestFlagOverrides` — TOML file sets values, overrides replace specific fields; zero-value overrides do not clobber file values
- `TestMissingFile` — non-empty path to a nonexistent file returns an error

### `internal/gallery`
Integration tests against a real MySQL instance. The Gallery 2 schema is fixed and read-only, so mocking the DB would hide real query errors.
- Use a test helper that connects with `config.DBConfig` read from env or a test config file; skip the suite if the DB is unavailable
- `TestGetRootAlbum` — ID 7, parent 0
- `TestGetAlbum` — known album ID, verify fields
- `TestChildAlbums` — known parent, expected child count / IDs
- `TestAlbumPhotos` / `TestAlbumMovies` — known album, spot-check returned items
- `TestGetPhoto` / `TestGetMovie` — known IDs, verify fields
- `TestItemPath` — known photo ID, verify reconstructed path matches expected string
- `TestGetDerivatives` / `TestGetThumbnail` — known photo ID with derivatives

### `internal/web`
HTTP handler tests using `net/http/httptest`. The handlers depend on `gallery.Store`, so introduce a minimal store interface covering only the methods the handlers call — this allows tests to run without a DB while keeping production code unaffected.
- Extract a `StoreInterface` (or per-handler interfaces) in `internal/web`; `gallery.Store` satisfies it automatically
- `TestIndexRedirect` — `GET /` → 302 to `/album/7`
- `TestAlbumHandler` — mock store returns a known album; verify status 200, template renders title and child links
- `TestAlbumNotFound` — store returns `sql.ErrNoRows`-wrapped error; verify 404
- `TestPhotoHandler` / `TestMovieHandler` — similar shape
- `TestServeFile_OK` — temp file under a temp data dir; verify 200 and correct content
- `TestServeFile_Traversal` — paths like `../secret`; verify 403
- `TestServeFile_NotFound` — nonexistent path; verify 404
- `TestServeFile_Directory` — path resolves to a directory; verify 404
