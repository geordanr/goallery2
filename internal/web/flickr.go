package web

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/flickr"
	"github.com/geordanr/goallery2/internal/gallery"
	"github.com/geordanr/goallery2/internal/uploader"
)

// TODO(geordan): move OAuth provider implementations to a separate oauth.go file in this package.

// localCookie returns a cookie pre-configured for local-only use: domain
// locked to localhost, one-hour expiry, HTTP-only. Secure is intentionally
// omitted because this server runs without TLS.
func localCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Domain:   "localhost",
		Path:     "/",
		Value:    value,
		Expires:  time.Now().Add(time.Hour),
		HttpOnly: true,
	}
}

// oauth initiates an OAuth v1 flow with the given provider, creating a request
// token and redirecting the user to the provider's authorization page. After
// the user approves, the provider calls back to oauthV1Callback which
// exchanges the request token for a long-lived access token stored in a cookie.
func (h *handler) oauth(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	config, err := h.config.GetOAuthConfig(provider)
	if err != nil {
		slog.Error("could not get OAuth config for provider", "provider", provider, "err", err)
		http.Error(w, "error performing OAuth", http.StatusInternalServerError)
		return
	}

	requestToken, requestSecret, err := config.RequestToken()
	if err != nil {
		slog.Error("could not get request token", "provider", provider, "err", err)
		http.Error(w, "error performing OAuth", http.StatusInternalServerError)
		return
	}

	authorizationURL, err := config.AuthorizationURL(requestToken)
	if err != nil {
		slog.Error("could not get authorization URL from provider", "provider", provider, "err", err)
		http.Error(w, "error performing OAuth", http.StatusInternalServerError)
		return
	}

	// Storing the request secret in a short-lived cookie is not ideal but
	// acceptable for a local-only server.
	http.SetCookie(w, localCookie(provider+"_request_secret", requestSecret))

	// Always set the return_to cookie (even when blank) to overwrite any stale
	// value left by a previous flow.
	http.SetCookie(w, localCookie(provider+"_return_to", r.URL.Query().Get("return_to")))

	http.Redirect(w, r, authorizationURL.String(), http.StatusFound)
}

// oauthV1Callback handles callbacks for OAuth v1 workflows.
func (h *handler) oauthV1Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	// Flickr (and most OAuth 1.0a providers) deliver the callback via query
	// parameters rather than a POST body, so ParseAuthorizationCallback reads
	// from r.URL.Query().
	requestToken, verifier, err := oauth1.ParseAuthorizationCallback(r)
	if err != nil {
		slog.Error("could not parse OAuth v1 callback request", "provider", provider, "err", err)
		http.Error(w, "error performing authentication", http.StatusInternalServerError)
		return
	}

	secretCookie, err := r.Cookie(provider + "_request_secret")
	if err != nil {
		slog.Error("could not get request secret cookie for provider", "provider", provider, "err", err)
		http.Error(w, "error performing authentication", http.StatusInternalServerError)
		return
	}

	oauthConfig, err := h.config.GetOAuthConfig(provider)
	if err != nil {
		slog.Error("could not get OAuth config for provider", "provider", provider, "err", err)
		http.Error(w, "error performing authentication", http.StatusInternalServerError)
		return
	}

	accessToken, accessSecret, err := oauthConfig.AccessToken(requestToken, secretCookie.Value, verifier)
	if err != nil {
		slog.Error("could not get access token for provider", "provider", provider, "err", err)
		http.Error(w, "error performing authentication", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, localCookie(provider+"_access_token", accessToken))
	http.SetCookie(w, localCookie(provider+"_access_secret", accessSecret))

	redirCookie, err := r.Cookie(provider + "_return_to")
	if err != nil {
		if !errors.Is(err, http.ErrNoCookie) {
			slog.Warn("could not get return path cookie for provider", "provider", provider, "err", err)
		}
		http.Redirect(w, r, h.config.Server.BaseURL(), http.StatusFound)
		return
	}
	redirPath := redirCookie.Value

	// Guard against open redirects: return_to must be a non-empty relative path.
	if redirPath == "" || !strings.HasPrefix(redirPath, "/") || strings.HasPrefix(redirPath, "//") {
		http.Redirect(w, r, h.config.Server.BaseURL(), http.StatusFound)
		return
	}

	// path.Join cleans the result and avoids a double-slash if BaseURL ever
	// gains a trailing slash.
	http.Redirect(w, r, h.config.Server.BaseURL()+path.Join("/", redirPath), http.StatusFound)
}

// flickrCookies reads the Flickr access token and secret from cookies.
// Returns the values and ok=true if both are present and non-empty.
func flickrCookies(r *http.Request) (token, secret string, ok bool) {
	tc, err1 := r.Cookie("flickr_access_token")
	sc, err2 := r.Cookie("flickr_access_secret")
	if err1 != nil || err2 != nil || tc.Value == "" || sc.Value == "" {
		return "", "", false
	}
	return tc.Value, sc.Value, true
}

// flickrVerify checks whether the user has a valid Flickr access token and,
// if so, calls flickr.test.login to confirm it works end-to-end. If no token
// cookie is present the user is redirected through the OAuth flow first.
func (h *handler) flickrVerify(w http.ResponseWriter, r *http.Request) {
	token, secret, ok := flickrCookies(r)
	if !ok {
		q := url.Values{"return_to": {"/flickr/verify"}}
		http.Redirect(w, r, "/oauth/start/flickr?"+q.Encode(), http.StatusFound)
		return
	}

	oauthCfg, err := h.config.GetOAuthConfig("flickr")
	if err != nil {
		slog.Error("could not get OAuth config for flickr", "err", err)
		http.Error(w, "flickr not configured", http.StatusInternalServerError)
		return
	}

	client := flickr.NewClient(flickr.Credentials{
		APIKey:      oauthCfg.ConsumerKey,
		APISecret:   oauthCfg.ConsumerSecret,
		Token:       token,
		TokenSecret: secret,
	})

	username, err := client.TestLogin()
	if err != nil {
		slog.Error("flickr.test.login failed", "err", err)
		http.Error(w, "Flickr API error", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "Connected to Flickr as: %s\n", username)
}

// flickrUploadPageData is shared by the confirmation (GET) and result (POST)
// views of the upload page.
type flickrUploadPageData struct {
	Album       *gallery.Album
	Uploads     []uploader.AlbumUpload
	TotalPhotos int
	Done        bool   // true after a POST completes
	UploadErr   string // non-empty if the upload failed
}

// flickrUpload handles GET (confirmation) and POST (run upload) for a single
// album. It checks for valid Flickr cookies first; missing tokens redirect
// through the OAuth flow with return_to pointing back here.
func (h *handler) flickrUpload(w http.ResponseWriter, r *http.Request) {
	token, secret, ok := flickrCookies(r)
	if !ok {
		q := url.Values{"return_to": {r.URL.RequestURI()}}
		http.Redirect(w, r, "/oauth/start/flickr?"+q.Encode(), http.StatusFound)
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid album id", http.StatusBadRequest)
		return
	}

	album, err := h.store.GetAlbum(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "album not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("fetching album for Flickr upload", "album_id", id, "err", err)
		http.Error(w, "error loading album", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		uploads, err := uploader.Walk(h.store, []int{id})
		if err != nil {
			slog.Error("walking albums for Flickr upload", "album_id", id, "err", err)
			http.Error(w, "error loading album structure", http.StatusInternalServerError)
			return
		}
		totalPhotos := 0
		for _, u := range uploads {
			totalPhotos += len(u.Photos)
		}
		data := flickrUploadPageData{
			Album:       album,
			Uploads:     uploads,
			TotalPhotos: totalPhotos,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := flickrUploadTmpl.Execute(w, data); err != nil {
			slog.Error("rendering flickr upload page", "err", err)
		}
		return
	}

	// POST: run the upload.
	oauthCfg, err := h.config.GetOAuthConfig("flickr")
	if err != nil {
		slog.Error("could not get OAuth config for flickr", "err", err)
		http.Error(w, "flickr not configured", http.StatusInternalServerError)
		return
	}

	uploads, err := uploader.Walk(h.store, []int{id})
	if err != nil {
		slog.Error("walking albums for Flickr upload", "album_id", id, "err", err)
		http.Error(w, "error loading album structure", http.StatusInternalServerError)
		return
	}

	statePath := h.config.Server.FlickrStatePath
	state, err := uploader.LoadState(statePath)
	if err != nil {
		slog.Error("loading Flickr upload state", "path", statePath, "err", err)
		http.Error(w, "error loading upload state", http.StatusInternalServerError)
		return
	}

	client := flickr.NewClient(flickr.Credentials{
		APIKey:      oauthCfg.ConsumerKey,
		APISecret:   oauthCfg.ConsumerSecret,
		Token:       token,
		TokenSecret: secret,
	})

	// TODO: the upload runs synchronously in the HTTP handler; for large albums
	// it can take many minutes. Monitor server logs for per-photo progress.
	u := uploader.New(client, state, statePath, h.absDataDir, false)
	data := flickrUploadPageData{Album: album, Done: true}
	if uploadErr := u.Run(uploads); uploadErr != nil {
		slog.Error("Flickr upload failed", "album_id", id, "err", uploadErr)
		data.UploadErr = uploadErr.Error()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := flickrUploadTmpl.Execute(w, data); err != nil {
		slog.Error("rendering flickr upload result page", "err", err)
	}
}
