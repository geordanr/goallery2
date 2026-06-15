# goallery2 — Project Plan

## Status

### Done

- **`internal/config`** — TOML config file + CLI flag overrides; `db.Connect` takes typed `DBConfig`; `ServerConfig.BaseURL()` derived from `Addr`; `GetOAuthConfig()` builds `oauth1.Config` per provider; `FlickrStatePath` for upload state (default `flickr_upload.json`)
- **`internal/gallery`** — `Store` with all DB queries for albums, photos, movies, derivatives; `itemPath` walk (prepends `albums/`, joins with `/`); `Derivative.CachePath()`; lazy-load methods on domain types; `gallery.Reader` interface decouples web/upload layers from concrete DB
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
- **Flickr export — CLI (`cmd/flickr_upload`)** — standalone binary; walks albums recursively; uploads photos as private with title/description/tags/date-taken; resumes via JSON state file; `-dryrun`, `-check`, `-state` flags; two-phase upload (collect IDs, then create/extend photosets) handles resume correctly
- **Flickr export — uploader package (`internal/uploader`)** — `Walk` builds flat `[]AlbumUpload` with prefixed titles; `Uploader.Run` drives upload with per-photo state saves; `FlickrClient` interface for testability; `State` with atomic save via temp+rename
- **Flickr client (`internal/flickr`)** — OAuth 1.0a signing via `dghubble/oauth1` transport; `UploadPhoto`, `SetPermissions`, `SetDateTaken`, `CreatePhotoset`, `AddPhotoToPhotoset`, `TestLogin`, `CheckAuth`; `Permissions{IsPublic, IsFriend, IsFamily}` struct (zero value = private); `SetPermissions` called after every upload (idempotent, runs on re-runs too) to enforce privacy regardless of the account's default setting, which Flickr would otherwise silently apply; full unit test suite
- **Browser OAuth flow** — `GET /oauth/start/{provider}` initiates OAuth 1.0a; `GET /oauth/callback/v1/{provider}` exchanges token and stores access credentials in cookies; `return_to` cookie preserves post-auth destination; open redirect guard
- **Flickr verify** — `GET /flickr/verify` confirms OAuth credentials end-to-end via `flickr.test.login`
- **Browser-triggered album upload** — `GET /flickr/upload/{id}` shows confirmation (photoset list + photo counts); `POST /flickr/upload/{id}` streams chunked HTML progress (one `<li>` per photo, auto-scroll via MutationObserver); shows per-photoset Flickr links on completion; redirects through OAuth if no token cookie present
- **Upload progress streaming** — `Uploader.Progress func(title string, skipped bool)` callback called per photo; POST handler passes a closure that writes+flushes a `<li>`; CLI leaves it nil

---

## Known limitations / future work

- **No rate limiting** — The uploader does not throttle Flickr API calls. Flickr's limits are generous but a very large upload could hit them.

---

## Next up

### 1. Conditional "Upload to Flickr" link

Only show the "Upload to Flickr" footer link on the album page when there are photos in the album tree that have not yet been uploaded.

#### Approach

- Load the Flickr state file in the album handler (`LoadState`); if the file is missing or unreadable, treat all photos as not uploaded (i.e., show the link)
- Call `uploader.Walk` to get the flat list of photos in the album tree
- Compare each photo ID against `state.Photos`; if any photo is absent from the state, show the link
- If all photos are present in the state (or the album tree has no photos), suppress the link
- Pass a `ShowFlickrUpload bool` field in `albumData` to the template

#### Notes

- `uploader.Walk` is already used by the upload handler, so the logic is reusable
- The state file read is a fast local JSON parse; acceptable overhead per album page load
- Albums with no photos at all should suppress the link (nothing to upload)

### 2. `internal/gallery` integration tests

Integration tests against a real MySQL instance. The Gallery 2 schema is fixed and read-only, so mocking the DB would hide real query errors.

- Use a test helper that connects with `config.DBConfig` read from env or a test config file; skip the suite if the DB is unavailable
- `TestGetRootAlbum` — ID 7, parent 0
- `TestGetAlbum` — known album ID, verify fields
- `TestChildAlbums` — known parent, expected child count / IDs
- `TestAlbumPhotos` / `TestAlbumMovies` — known album, spot-check returned items
- `TestGetPhoto` / `TestGetMovie` — known IDs, verify fields
- `TestItemPath` — known photo ID, verify reconstructed path starts with `albums/` and uses forward slashes
- `TestGetDerivatives` / `TestGetThumbnail` — known photo ID with derivatives

### 2. gRPC / protobuf interface (future)

A secondary read-only interface for mobile clients. Design notes:

- Define proto messages mirroring the domain types in `internal/gallery`
- gRPC server in `cmd/grpc` (separate binary or combined with HTTP)
- Keep all data access through `gallery.Store` — no new queries in the gRPC layer
