package flickr

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func okUploadResponse(photoID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><rsp stat="ok"><photoid>%s</photoid></rsp>`, photoID)
}

func errResponse(code, msg string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><rsp stat="fail"><err code="%s" msg="%s"/></rsp>`, code, msg)
}

func okPhotosetResponse(id string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><rsp stat="ok"><photoset id="%s"/></rsp>`, id)
}

func okAddPhotoResponse() string {
	return `<?xml version="1.0" encoding="utf-8"?><rsp stat="ok"/>`
}

func okSetPermsResponse() string {
	return `<?xml version="1.0" encoding="utf-8"?><rsp stat="ok"/>`
}

func newTestClient(uploadSrv, apiSrv *httptest.Server) *Client {
	c := NewClient(Credentials{
		APIKey:      "key",
		APISecret:   "secret",
		Token:       "token",
		TokenSecret: "tokensecret",
	})
	if uploadSrv != nil {
		c.uploadURL = uploadSrv.URL
	}
	if apiSrv != nil {
		c.apiURL = apiSrv.URL
	}
	return c
}

// ── UploadPhoto ───────────────────────────────────────────────────────────────

func TestUploadPhoto_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, okUploadResponse("photo123"))
	}))
	defer srv.Close()

	// Write a temp file for the upload.
	tmpFile := t.TempDir() + "/photo.jpg"
	if err := os.WriteFile(tmpFile, []byte("fake jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(srv, nil)
	id, err := c.UploadPhoto(tmpFile, "My Photo", "A description", "tag1 tag2")
	if err != nil {
		t.Fatalf("UploadPhoto: %v", err)
	}
	if id != "photo123" {
		t.Errorf("photo ID = %q, want %q", id, "photo123")
	}
}

func TestUploadPhoto_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("100", "invalid api key"))
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/photo.jpg"
	if err := os.WriteFile(tmpFile, []byte("fake jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(srv, nil)
	_, err := c.UploadPhoto(tmpFile, "", "", "")
	if err == nil {
		t.Fatal("expected error for failed upload, got nil")
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should contain server message; got %v", err)
	}
}

// TestUploadPhoto_ParamsAsQueryAndFileAsMultipart verifies that non-file params
// are sent as URL query parameters (so they are covered by the OAuth signature)
// and that the multipart body contains only the photo file.
func TestUploadPhoto_ParamsAsQueryAndFileAsMultipart(t *testing.T) {
	var gotQuery url.Values
	var gotParts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				gotParts = append(gotParts, p.FormName())
			}
		}
		_, _ = fmt.Fprint(w, okUploadResponse("p1"))
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/img.jpg"
	if err := os.WriteFile(tmpFile, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(srv, nil)
	if _, err := c.UploadPhoto(tmpFile, "My Title", "My Desc", "tag1"); err != nil {
		t.Fatalf("UploadPhoto: %v", err)
	}

	// API params must appear in the query string with the correct values.
	for _, tc := range []struct{ key, want string }{
		{"title", "My Title"},
		{"description", "My Desc"},
		{"tags", "tag1"},
		{"is_public", "0"},
		{"is_friend", "0"},
		{"is_family", "0"},
	} {
		if got := gotQuery.Get(tc.key); got != tc.want {
			t.Errorf("query param %q = %q, want %q", tc.key, got, tc.want)
		}
	}

	// Multipart body must contain only the photo file.
	if len(gotParts) != 1 || gotParts[0] != "photo" {
		t.Errorf("multipart parts = %v, want [photo]", gotParts)
	}
}

// ── CreatePhotoset ────────────────────────────────────────────────────────────

func TestCreatePhotoset_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, okPhotosetResponse("ps42"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	id, err := c.CreatePhotoset("My Album", "photo123")
	if err != nil {
		t.Fatalf("CreatePhotoset: %v", err)
	}
	if id != "ps42" {
		t.Errorf("photoset ID = %q, want %q", id, "ps42")
	}
}

func TestCreatePhotoset_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("1", "photoset not created"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	_, err := c.CreatePhotoset("Album", "pid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── CheckAuth ─────────────────────────────────────────────────────────────────

func TestCheckAuth_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if r.FormValue("method") != "flickr.test.echo" {
			t.Errorf("method = %q, want flickr.test.echo", r.FormValue("method"))
		}
		// flickr.test.echo echoes all params back inside the response XML.
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?><rsp stat="ok"/>`)
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	if err := c.CheckAuth(); err != nil {
		t.Fatalf("CheckAuth: %v", err)
	}
}

func TestCheckAuth_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("100", "Invalid API Key"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	if err := c.CheckAuth(); err == nil {
		t.Fatal("expected error for invalid credentials, got nil")
	}
}

// ── AddPhotoToPhotoset ────────────────────────────────────────────────────────

func TestAddPhotoToPhotoset_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, okAddPhotoResponse())
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	if err := c.AddPhotoToPhotoset("ps1", "p1"); err != nil {
		t.Fatalf("AddPhotoToPhotoset: %v", err)
	}
}

func TestAddPhotoToPhotoset_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("2", "photo not found"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	if err := c.AddPhotoToPhotoset("ps1", "bad"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── SetPermissions ────────────────────────────────────────────────────────────

func TestSetPermissions_OK(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
			return
		}
		gotForm = r.Form
		_, _ = fmt.Fprint(w, okSetPermsResponse())
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	if err := c.SetPermissions("p1", Permissions{}); err != nil {
		t.Fatalf("SetPermissions: %v", err)
	}
	if got := gotForm.Get("method"); got != "flickr.photos.setPerms" {
		t.Errorf("method = %q, want %q", got, "flickr.photos.setPerms")
	}
	if got := gotForm.Get("photo_id"); got != "p1" {
		t.Errorf("photo_id = %q, want %q", got, "p1")
	}
	for _, field := range []string{"is_public", "is_friend", "is_family", "perm_comment", "perm_addmeta"} {
		if got := gotForm.Get(field); got != "0" {
			t.Errorf("%s = %q, want %q", field, got, "0")
		}
	}
}

func TestSetPermissions_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("1", "Photo not found"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	err := c.SetPermissions("bad", Permissions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Photo not found") {
		t.Errorf("error should contain Flickr message; got %v", err)
	}
}

// ── SetDateTaken ──────────────────────────────────────────────────────────────

func TestSetDateTaken_OK(t *testing.T) {
	var gotForm string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			gotForm = r.FormValue("date_taken")
		}
		_, _ = fmt.Fprint(w, okAddPhotoResponse())
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	ts := time.Date(2003, 7, 14, 10, 30, 0, 0, time.UTC)
	if err := c.SetDateTaken("p1", ts); err != nil {
		t.Fatalf("SetDateTaken: %v", err)
	}
	if gotForm != "2003-07-14 10:30:00" {
		t.Errorf("date_taken = %q, want %q", gotForm, "2003-07-14 10:30:00")
	}
}

func TestSetDateTaken_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("5", "The date taken is not in the format that we support"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	if err := c.SetDateTaken("p1", time.Now()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── TestLogin ─────────────────────────────────────────────────────────────────

func TestTestLogin_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if r.FormValue("method") != "flickr.test.login" {
			t.Errorf("method = %q, want flickr.test.login", r.FormValue("method"))
		}
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?><rsp stat="ok"><user id="123"><username>testuser</username></user></rsp>`)
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	username, err := c.TestLogin()
	if err != nil {
		t.Fatalf("TestLogin: %v", err)
	}
	if username != "testuser" {
		t.Errorf("username = %q, want %q", username, "testuser")
	}
}

func TestTestLogin_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, errResponse("98", "Login failed / Invalid auth token"))
	}))
	defer srv.Close()

	c := newTestClient(nil, srv)
	_, err := c.TestLogin()
	if err == nil {
		t.Fatal("expected error for failed login, got nil")
	}
	if !strings.Contains(err.Error(), "Login failed") {
		t.Errorf("error should contain server message; got %v", err)
	}
}
