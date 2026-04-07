package flickr

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	orig := openFile
	openFile = func(path string) (*os.File, error) { return os.Open(path) }
	t.Cleanup(func() { openFile = orig })
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

func TestUploadPhoto_MultipartContainsFile(t *testing.T) {
	var gotParts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if _, err := c.UploadPhoto(tmpFile, "title", "desc", "t"); err != nil {
		t.Fatalf("UploadPhoto: %v", err)
	}

	wantParts := map[string]bool{"title": false, "description": false, "photo": false}
	for _, name := range gotParts {
		wantParts[name] = true
	}
	for name, found := range wantParts {
		if !found {
			t.Errorf("multipart request missing field %q", name)
		}
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

// ── sign ──────────────────────────────────────────────────────────────────────

func TestSign_Deterministic(t *testing.T) {
	c := NewClient(Credentials{APIKey: "k", APISecret: "s", Token: "t", TokenSecret: "ts"})
	params := map[string]string{"foo": "bar", "baz": "qux"}
	sig1 := c.sign("POST", "https://example.com/api", params)
	sig2 := c.sign("POST", "https://example.com/api", params)
	if sig1 != sig2 {
		t.Errorf("sign is not deterministic: %q vs %q", sig1, sig2)
	}
}

func TestSign_DifferentParamsProduceDifferentSigs(t *testing.T) {
	c := NewClient(Credentials{APIKey: "k", APISecret: "s", Token: "t", TokenSecret: "ts"})
	sig1 := c.sign("POST", "https://example.com/api", map[string]string{"a": "1"})
	sig2 := c.sign("POST", "https://example.com/api", map[string]string{"a": "2"})
	if sig1 == sig2 {
		t.Error("different params should produce different signatures")
	}
}
