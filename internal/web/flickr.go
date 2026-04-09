package web

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/dghubble/oauth1"
	"github.com/go-chi/chi/v5"

	"github.com/geordanr/goallery2/internal/flickr"
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

// flickrVerify checks whether the user has a valid Flickr access token and,
// if so, calls flickr.test.login to confirm it works end-to-end. If no token
// cookie is present the user is redirected through the OAuth flow first.
func (h *handler) flickrVerify(w http.ResponseWriter, r *http.Request) {
	tokenCookie, tokenErr := r.Cookie("flickr_access_token")
	secretCookie, secretErr := r.Cookie("flickr_access_secret")
	if tokenErr != nil || secretErr != nil {
		q := url.Values{"return_to": {"/flickr/verify"}}
		http.Redirect(w, r, "/oauth/start/flickr?"+q.Encode(), http.StatusFound)
		return
	}
	if tokenCookie.Value == "" || secretCookie.Value == "" {
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
		Token:       tokenCookie.Value,
		TokenSecret: secretCookie.Value,
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
