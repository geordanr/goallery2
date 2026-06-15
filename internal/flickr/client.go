// Package flickr provides a minimal Flickr API client covering photo upload
// and photoset management. Authentication uses OAuth 1.0a with pre-obtained
// access tokens (no interactive flow — tokens must be obtained separately).
package flickr

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/dghubble/oauth1"
)

const (
	uploadURL = "https://up.flickr.com/services/upload/"
	apiURL    = "https://www.flickr.com/services/rest/"
)

// Credentials holds Flickr OAuth 1.0a tokens.
type Credentials struct {
	APIKey      string
	APISecret   string
	Token       string
	TokenSecret string
}

// Permissions controls who can see a Flickr photo.
// The zero value makes a photo completely private (owner-only).
type Permissions struct {
	IsPublic bool // visible to everyone
	IsFriend bool // visible to contacts marked as friends
	IsFamily bool // visible to contacts marked as family
}

// Client is a minimal Flickr API client.
type Client struct {
	creds     Credentials
	http      *http.Client
	uploadURL string // if non-empty, overrides the uploadURL constant (for testing)
	apiURL    string // if non-empty, overrides the apiURL constant (for testing)
}

// NewClient creates a Client with the given credentials.
func NewClient(creds Credentials) *Client {
	cfg := oauth1.NewConfig(creds.APIKey, creds.APISecret)
	tok := oauth1.NewToken(creds.Token, creds.TokenSecret)
	httpClient := cfg.Client(context.Background(), tok)
	httpClient.Timeout = 120 * time.Second
	return &Client{
		creds: creds,
		http:  httpClient,
	}
}

func (c *Client) effectiveUploadURL() string {
	if c.uploadURL != "" {
		return c.uploadURL
	}
	return uploadURL
}

func (c *Client) effectiveAPIURL() string {
	if c.apiURL != "" {
		return c.apiURL
	}
	return apiURL
}

// UploadPhoto uploads the photo at diskPath to Flickr as a private photo.
// title, description, and tags are set from Gallery 2 metadata. Returns the
// Flickr photo ID. Date taken must be set separately via SetDateTaken.
//
// Non-file parameters are sent as URL query parameters rather than multipart
// form fields. dghubble/oauth1 includes query parameters in the OAuth
// signature but excludes multipart body fields, so this ensures both sides
// compute the same signature base string.
func (c *Client) UploadPhoto(diskPath, title, description, tags string) (string, error) {
	q := url.Values{
		"title":       {title},
		"description": {description},
		"tags":        {tags},
		"is_public":   {"0"},
		"is_friend":   {"0"},
		"is_family":   {"0"},
	}
	rawURL := c.effectiveUploadURL() + "?" + q.Encode()

	body, contentType, err := buildUploadBody(diskPath)
	if err != nil {
		return "", fmt.Errorf("building upload request for %q: %w", diskPath, err)
	}

	resp, err := c.http.Post(rawURL, contentType, body) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("uploading %q: %w", diskPath, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading upload response: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"rsp"`
		Stat    string   `xml:"stat,attr"`
		PhotoID string   `xml:"photoid"`
		Err     struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(raw, &result); err != nil {
		slog.Debug("could not parse XML response", "raw", raw)
		return "", fmt.Errorf("parsing upload response: %w", err)
	}
	if result.Stat != "ok" {
		return "", fmt.Errorf("upload failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return result.PhotoID, nil
}

// TestLogin calls flickr.test.login to verify that the access token is valid
// and returns the authenticated user's username.
func (c *Client) TestLogin() (string, error) {
	params := map[string]string{
		"method":  "flickr.test.login",
		"api_key": c.creds.APIKey,
		"format":  "rest",
	}
	resp, err := c.callAPI(params)
	if err != nil {
		return "", fmt.Errorf("flickr.test.login: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"rsp"`
		Stat    string   `xml:"stat,attr"`
		User    struct {
			Username string `xml:"username"`
		} `xml:"user"`
		Err struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing flickr.test.login response: %w", err)
	}
	if result.Stat != "ok" {
		return "", fmt.Errorf("flickr.test.login failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return result.User.Username, nil
}

// CheckAuth calls flickr.test.echo to verify that the credentials are valid
// and that signed requests reach Flickr correctly. Returns nil on success, or
// an error describing the failure (network, signing, or API-level rejection).
func (c *Client) CheckAuth() error {
	// flickr.test.echo returns stat="ok" for any valid signed request; no
	// additional payload params are needed.
	params := map[string]string{
		"method":  "flickr.test.echo",
		"api_key": c.creds.APIKey,
		"format":  "rest",
	}
	resp, err := c.callAPI(params)
	if err != nil {
		return fmt.Errorf("flickr.test.echo: %w", err)
	}

	var result struct {
		XMLName xml.Name `xml:"rsp"`
		Stat    string   `xml:"stat,attr"`
		Err     struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parsing flickr.test.echo response: %w", err)
	}
	if result.Stat != "ok" {
		return fmt.Errorf("flickr.test.echo failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return nil
}

// SetPermissions applies p to a photo via flickr.photos.setPerms. This must be
// called after upload because Flickr ignores per-upload privacy params when the
// account's default privacy setting is "public". Pass Permissions{} to make a
// photo completely private.
func (c *Client) SetPermissions(photoID string, p Permissions) error {
	params := map[string]string{
		"method":       "flickr.photos.setPerms",
		"api_key":      c.creds.APIKey,
		"photo_id":     photoID,
		"is_public":    boolParam(p.IsPublic),
		"is_friend":    boolParam(p.IsFriend),
		"is_family":    boolParam(p.IsFamily),
		"perm_comment": "0",
		"perm_addmeta": "0",
		"format":       "rest",
	}
	resp, err := c.callAPI(params)
	if err != nil {
		return fmt.Errorf("setting permissions for photo %s: %w", photoID, err)
	}

	var result struct {
		XMLName xml.Name `xml:"rsp"`
		Stat    string   `xml:"stat,attr"`
		Err     struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parsing setPerms response: %w", err)
	}
	if result.Stat != "ok" {
		return fmt.Errorf("flickr.photos.setPerms failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return nil
}

// SetDateTaken sets the date taken on an already-uploaded photo via
// flickr.photos.setDates. dateTaken must be non-zero.
func (c *Client) SetDateTaken(photoID string, dateTaken time.Time) error {
	params := map[string]string{
		"method":     "flickr.photos.setDates",
		"api_key":    c.creds.APIKey,
		"photo_id":   photoID,
		"date_taken": dateTaken.Format("2006-01-02 15:04:05"),
		"format":     "rest",
	}
	resp, err := c.callAPI(params)
	if err != nil {
		return fmt.Errorf("flickr.photos.setDates for photo %s: %w", photoID, err)
	}

	var result struct {
		XMLName xml.Name `xml:"rsp"`
		Stat    string   `xml:"stat,attr"`
		Err     struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parsing setDates response: %w", err)
	}
	if result.Stat != "ok" {
		return fmt.Errorf("setDates failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return nil
}

// CreatePhotoset creates a new Flickr photoset with the given title, using
// primaryPhotoID as the cover photo. Returns the new photoset ID.
func (c *Client) CreatePhotoset(title, primaryPhotoID string) (string, error) {
	params := map[string]string{
		"method":           "flickr.photosets.create",
		"api_key":          c.creds.APIKey,
		"title":            title,
		"primary_photo_id": primaryPhotoID,
		"format":           "rest",
	}
	resp, err := c.callAPI(params)
	if err != nil {
		return "", fmt.Errorf("creating photoset %q: %w", title, err)
	}

	var result struct {
		XMLName  xml.Name `xml:"rsp"`
		Stat     string   `xml:"stat,attr"`
		Photoset struct {
			ID string `xml:"id,attr"`
		} `xml:"photoset"`
		Err struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("parsing photoset response: %w", err)
	}
	if result.Stat != "ok" {
		return "", fmt.Errorf("photoset creation failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return result.Photoset.ID, nil
}

// AddPhotoToPhotoset adds an existing Flickr photo to a photoset.
func (c *Client) AddPhotoToPhotoset(photosetID, photoID string) error {
	params := map[string]string{
		"method":      "flickr.photosets.addPhoto",
		"api_key":     c.creds.APIKey,
		"photoset_id": photosetID,
		"photo_id":    photoID,
		"format":      "rest",
	}
	resp, err := c.callAPI(params)
	if err != nil {
		return fmt.Errorf("adding photo %s to photoset %s: %w", photoID, photosetID, err)
	}

	var result struct {
		XMLName xml.Name `xml:"rsp"`
		Stat    string   `xml:"stat,attr"`
		Err     struct {
			Code string `xml:"code,attr"`
			Msg  string `xml:"msg,attr"`
		} `xml:"err"`
	}
	if err := xml.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parsing addPhoto response: %w", err)
	}
	if result.Stat != "ok" {
		return fmt.Errorf("addPhoto failed (code %s): %s", result.Err.Code, result.Err.Msg)
	}
	return nil
}

func boolParam(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// callAPI posts a signed REST API call and returns the raw response body.
func (c *Client) callAPI(params map[string]string) ([]byte, error) {
	target := c.effectiveAPIURL()

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	resp, err := c.http.PostForm(target, form) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

// openFile opens the file at path for reading. Extracted so it can be
// replaced in tests.
var openFile = func(path string) (*os.File, error) { return os.Open(path) }

// buildUploadBody constructs the multipart/form-data body for a photo upload.
// Only the photo file is included; API params are sent as URL query parameters
// so they are covered by the OAuth signature base string.
func buildUploadBody(diskPath string) (io.Reader, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var closeErr error
		defer func() { _ = pw.CloseWithError(closeErr) }()

		// Write the photo file.
		fw, err := mw.CreateFormFile("photo", filepath.Base(diskPath))
		if err != nil {
			closeErr = err
			return
		}
		f, err := openFile(diskPath)
		if err != nil {
			closeErr = fmt.Errorf("opening %q: %w", diskPath, err)
			return
		}
		defer func() { _ = f.Close() }()
		if _, err := io.Copy(fw, f); err != nil {
			closeErr = fmt.Errorf("reading %q: %w", diskPath, err)
			return
		}
		closeErr = mw.Close()
	}()

	return pr, mw.FormDataContentType(), nil
}
