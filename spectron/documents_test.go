package spectron

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDocumentsUploadMultipart(t *testing.T) {
	var (
		gotPath     string
		gotFile     []byte
		gotFilename string
		gotMime     string
		gotScope    string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("parse content-type: %v", err)
			return
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("media type = %q", mediaType)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("next part: %v", err)
				return
			}
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "file":
				gotFile = data
				gotFilename = part.FileName()
				gotMime = part.Header.Get("Content-Type")
			case "scope":
				gotScope = string(data)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"contentHash":"h","deduplicated":false,"id":"d1","status":"ok"}`)
	}))
	defer srv.Close()

	c, err := New("ctx-1", srv.URL, "sk", WithTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	body := bytes.NewReader([]byte("PDFBYTES"))
	resp, err := c.Documents().Upload(context.Background(), body,
		WithFilename("returns.pdf"),
		WithContentType("application/pdf"),
		WithScope(Scope{"org": "acme", "user": "tobie"}),
	)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if resp.ID != "d1" || resp.ContentHash != "h" || resp.Status != "ok" {
		t.Errorf("response = %+v", resp)
	}
	if gotPath != "/api/v1/ctx-1/documents" {
		t.Errorf("path = %q", gotPath)
	}
	if string(gotFile) != "PDFBYTES" {
		t.Errorf("file bytes = %q", string(gotFile))
	}
	if gotFilename != "returns.pdf" {
		t.Errorf("filename = %q", gotFilename)
	}
	if gotMime != "application/pdf" {
		t.Errorf("file mime = %q", gotMime)
	}

	// Scope must be sent as a JSON list-of-pairs.
	var scope []map[string]string
	if err := json.Unmarshal([]byte(gotScope), &scope); err != nil {
		t.Fatalf("scope json: %v (raw=%q)", err, gotScope)
	}
	if len(scope) != 2 {
		t.Fatalf("scope len = %d", len(scope))
	}
	// Sorted by key.
	if scope[0]["key"] != "org" || scope[0]["value"] != "acme" {
		t.Errorf("scope[0] = %v", scope[0])
	}
	if scope[1]["key"] != "user" || scope[1]["value"] != "tobie" {
		t.Errorf("scope[1] = %v", scope[1])
	}
}

func TestDocumentsUploadDefaultsAndSanitisation(t *testing.T) {
	var gotFilename string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			t.Errorf("part: %v", err)
			return
		}
		gotFilename = part.FileName()
		_, _ = io.Copy(io.Discard, part)
		_, _ = io.WriteString(w, `{"contentHash":"h","deduplicated":false,"id":"d","status":"s"}`)
	}))
	defer srv.Close()

	c, _ := New("c", srv.URL, "k")
	defer c.Close()

	_, err := c.Documents().Upload(context.Background(), strings.NewReader("x"),
		WithFilename("path/with/slash.txt"),
	)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if strings.ContainsAny(gotFilename, "/\\") {
		t.Errorf("filename should be sanitised, got %q", gotFilename)
	}
}

func TestDocumentsUploadRequiresBody(t *testing.T) {
	c, _ := New("c", "http://x", "k")
	defer c.Close()
	if _, err := c.Documents().Upload(context.Background(), nil); err == nil {
		t.Error("expected error for nil body")
	}
}
