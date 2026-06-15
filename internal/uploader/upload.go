package uploader

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/geordanr/goallery2/internal/flickr"
	"github.com/geordanr/goallery2/internal/gallery"
)

// FlickrClient is the subset of the Flickr API used by Uploader. *flickr.Client
// satisfies this interface; tests substitute a fake.
type FlickrClient interface {
	UploadPhoto(diskPath, title, description, tags string) (string, error)
	SetPermissions(photoID string, p flickr.Permissions) error
	SetDateTaken(photoID string, dateTaken time.Time) error
	CreatePhotoset(title, primaryPhotoID string) (string, error)
	AddPhotoToPhotoset(photosetID, photoID string) error
}

// Uploader drives the Flickr export: walks albums, uploads photos, and
// maintains the state file so re-runs skip already-completed work.
type Uploader struct {
	client    FlickrClient
	state     State
	statePath string
	dataDir   string
	dryRun    bool
	// Progress is called after each photo is processed (uploaded or skipped).
	// It is optional; nil means no progress reporting.
	Progress func(title string, skipped bool)
}

// New creates an Uploader.
//
//   - client: authenticated Flickr API client (nil is allowed when dryRun is true)
//   - state: progress loaded from disk (may be empty)
//   - statePath: path to write updated state after each photo/photoset
//   - dataDir: absolute path to the Gallery 2 data directory (g2data)
//   - dryRun: if true, print planned actions without uploading anything
func New(client FlickrClient, state State, statePath, dataDir string, dryRun bool) *Uploader {
	return &Uploader{
		client:    client,
		state:     state,
		statePath: statePath,
		dataDir:   dataDir,
		dryRun:    dryRun,
	}
}

// Run uploads all albums in uploads, in order, to Flickr. For each album it
// creates a photoset (if needed) and uploads each photo (if not already done).
// State is saved after every successful photo and photoset so progress is
// preserved across restarts.
func (u *Uploader) Run(uploads []AlbumUpload) error {
	for _, au := range uploads {
		if err := u.processAlbum(au); err != nil {
			return err
		}
	}
	return nil
}

func (u *Uploader) processAlbum(au AlbumUpload) error {
	albumID := au.SourceAlbum.ID

	if u.dryRun {
		slog.Info("dry-run: album",
			"flickr_title", au.FlickrTitle,
			"gallery_album_id", albumID,
			"photo_count", len(au.Photos),
		)
		for _, p := range au.Photos {
			slog.Info("dry-run: photo",
				"gallery_photo_id", p.ID,
				"title", photoTitle(p),
			)
		}
		return nil
	}

	if len(au.Photos) == 0 {
		slog.Info("album has no photos, skipping photoset creation",
			"flickr_title", au.FlickrTitle,
			"gallery_album_id", albumID,
		)
		return nil
	}

	// Phase 1: ensure every photo is uploaded. Track which were already done
	// before this run so phase 2 can avoid re-adding them to an existing photoset.
	flickrIDs := make([]string, len(au.Photos))
	wasPreUploaded := make([]bool, len(au.Photos))
	for i, photo := range au.Photos {
		_, wasPreUploaded[i] = u.state.Photos[photo.ID]
		fid, err := u.ensureUploaded(photo)
		if err != nil {
			return fmt.Errorf("album %q photo %d: %w", au.FlickrTitle, photo.ID, err)
		}
		flickrIDs[i] = fid
	}

	// Phase 2: create or extend the photoset.
	//
	// Two distinct cases:
	//   A) Photoset does not exist yet — add ALL photos (even ones uploaded in a
	//      prior run) because they were never placed in a photoset.
	//   B) Photoset already exists — add only photos that are new this run; the
	//      rest were already added when the photoset was first populated.
	photosetID, photosetPreExisted := u.state.Photosets[albumID]
	if photosetPreExisted {
		slog.Info("resuming existing photoset",
			"flickr_title", au.FlickrTitle,
			"flickr_photoset_id", photosetID,
		)
	}

	for i, flickrID := range flickrIDs {
		switch {
		case photosetID == "":
			// Photoset doesn't exist yet; first photo creates it.
			psID, err := u.client.CreatePhotoset(au.FlickrTitle, flickrID)
			if err != nil {
				return fmt.Errorf("creating photoset %q: %w", au.FlickrTitle, err)
			}
			photosetID = psID
			u.state.Photosets[albumID] = photosetID
			if err := SaveState(u.statePath, u.state); err != nil {
				slog.Warn("failed to save state after creating photoset", "err", err)
			}
			slog.Info("created photoset",
				"flickr_title", au.FlickrTitle,
				"flickr_photoset_id", photosetID,
			)
		case !photosetPreExisted || !wasPreUploaded[i]:
			// Photoset was just created this run, OR photoset pre-existed but
			// this photo is new — add it.
			if err := u.client.AddPhotoToPhotoset(photosetID, flickrID); err != nil {
				return fmt.Errorf("adding photo %s to photoset %s: %w", flickrID, photosetID, err)
			}
		}
	}

	return nil
}

// ensureUploaded uploads a single photo to Flickr if not already done. Returns
// the Flickr photo ID (from state or freshly uploaded), or an error.
//
// Post-upload API calls (SetPermissions, SetDateTaken) run on every invocation,
// even when the photo was found in state. This lets a re-run finish work that a
// prior crash interrupted after the upload was persisted but before those calls
// completed.
func (u *Uploader) ensureUploaded(photo gallery.Photo) (string, error) {
	title := photoTitle(photo)

	flickrID, alreadyUploaded := u.state.Photos[photo.ID]

	if !alreadyUploaded {
		relPath, err := photo.Path()
		if err != nil {
			return "", fmt.Errorf("resolving disk path for photo %d: %w", photo.ID, err)
		}
		diskPath := filepath.Join(u.dataDir, filepath.FromSlash(relPath))

		tags := buildTags(photo.Keywords)
		flickrID, err = u.client.UploadPhoto(diskPath, title, photo.Description, tags)
		if err != nil {
			return "", fmt.Errorf("uploading photo %d: %w", photo.ID, err)
		}

		// Persist before post-upload calls. If state cannot be saved the upload
		// is still orphaned on Flickr, but returning an error here prevents
		// further in-run work against an un-checkpointed ID and makes the
		// failure visible rather than silently risking duplicate uploads.
		u.state.Photos[photo.ID] = flickrID
		if err := SaveState(u.statePath, u.state); err != nil {
			return "", fmt.Errorf("saving state after uploading photo %d: %w", photo.ID, err)
		}
	}

	// Enforce private permissions explicitly. Flickr ignores per-upload privacy
	// params when the account's default privacy is "public", so this call is
	// required to ensure photos are never publicly visible. It is idempotent.
	if err := u.client.SetPermissions(flickrID, flickr.Permissions{}); err != nil {
		return "", fmt.Errorf("setting permissions for photo %d: %w", photo.ID, err)
	}

	// Explicitly set date taken from Gallery 2 metadata even though Flickr can
	// extract it from EXIF. Gallery 2's date is authoritative for this migration:
	// photos that were scanned, digitized, or had their EXIF stripped may have a
	// manually-corrected date in Gallery 2 that differs from (or is absent in)
	// the file's EXIF.
	if !photo.OriginatedAt.IsZero() {
		if err := u.client.SetDateTaken(flickrID, photo.OriginatedAt); err != nil {
			return "", fmt.Errorf("setting date taken for photo %d: %w", photo.ID, err)
		}
	}

	slog.Info("processed photo",
		"gallery_photo_id", photo.ID,
		"flickr_photo_id", flickrID,
		"title", title,
		"skipped_upload", alreadyUploaded,
	)
	if u.Progress != nil {
		u.Progress(title, alreadyUploaded)
	}
	return flickrID, nil
}

// photoTitle returns the display title for a photo: Title if set, otherwise
// PathComponent (the filename without extension is a reasonable fallback).
func photoTitle(p gallery.Photo) string {
	if p.Title != "" {
		return p.Title
	}
	return p.PathComponent
}

// buildTags converts a Gallery 2 comma-separated keyword string into a Flickr
// tags string (space-separated, multi-word tags double-quoted).
func buildTags(keywords string) string {
	if keywords == "" {
		return ""
	}
	parts := strings.Split(keywords, ",")
	tags := make([]string, 0, len(parts))
	for _, kw := range parts {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if strings.ContainsRune(kw, ' ') {
			tags = append(tags, `"`+kw+`"`)
		} else {
			tags = append(tags, kw)
		}
	}
	return strings.Join(tags, " ")
}
