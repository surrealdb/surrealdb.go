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
		gotMetadata string
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
		var partOrder []string
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("next part: %v", err)
				return
			}
			partOrder = append(partOrder, part.FormName())
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "file":
				gotFile = data
				gotFilename = part.FileName()
				gotMime = part.Header.Get("Content-Type")
			case "metadata":
				gotMetadata = string(data)
			}
		}
		// The server reads the metadata part before it streams the file, so it
		// must come first on the wire (spectron #713).
		if len(partOrder) != 2 || partOrder[0] != "metadata" || partOrder[1] != "file" {
			t.Errorf("part order = %v", partOrder)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"contentHash":"h","deduplicated":false,"id":"d1","status":"ready"}`)
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
		WithScopes(ScopeSets{{scopeAcme, "user/tobie"}}),
	)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if resp.ID != "d1" || resp.ContentHash != "h" || resp.Status != DocReady {
		t.Errorf("response = %+v", resp)
	}
	if gotPath != docsPath {
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

	// Scope is sent in the "metadata" JSON part as a DNF selector: an array of
	// clauses, each a string array of slash-paths (spectron #713).
	var meta struct {
		Scopes [][]string `json:"scopes"`
	}
	if err := json.Unmarshal([]byte(gotMetadata), &meta); err != nil {
		t.Fatalf("metadata json: %v (raw=%q)", err, gotMetadata)
	}
	if len(meta.Scopes) != 1 || len(meta.Scopes[0]) != 2 ||
		meta.Scopes[0][0] != scopeAcme || meta.Scopes[0][1] != "user/tobie" {
		t.Errorf("scopes = %v", meta.Scopes)
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
		t.Errorf("filename should be sanitized, got %q", gotFilename)
	}
}

func TestDocumentsUploadRequiresBody(t *testing.T) {
	c, _ := New("c", "http://x", "k")
	defer c.Close()
	if _, err := c.Documents().Upload(context.Background(), nil); err == nil {
		t.Error("expected error for nil body")
	}
}
