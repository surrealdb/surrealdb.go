package spectron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// Documents is the document sub-client returned by [Client.Documents].
type Documents struct {
	client *Client
}

// UploadOption tweaks a [Documents.Upload] call.
type UploadOption func(*uploadOptions)

type uploadOptions struct {
	filename    string
	contentType string
	scopes      ScopeSets
	labels      []string
	title       string
}

// WithFilename sets the filename advertised in the multipart Content-
// Disposition header. Defaults to "upload" when unset.
func WithFilename(name string) UploadOption {
	return func(o *uploadOptions) { o.filename = name }
}

// WithContentType sets the MIME type for the uploaded file part. Defaults
// to application/octet-stream.
func WithContentType(ct string) UploadOption {
	return func(o *uploadOptions) { o.contentType = ct }
}

// WithScopes attaches a DNF scope selector to the upload. The scopes are sent
// in the "metadata" JSON part of the multipart payload (which the server reads
// before the file part).
func WithScopes(scopes ScopeSets) UploadOption {
	return func(o *uploadOptions) { o.scopes = scopes }
}

// WithLabels attaches "key=value" labels to the upload. The labels are sent in
// the "metadata" JSON part of the multipart payload.
func WithLabels(labels []string) UploadOption {
	return func(o *uploadOptions) { o.labels = labels }
}

// WithTitle sets the document title sent in the "metadata" JSON part of the
// multipart payload.
func WithTitle(title string) UploadOption {
	return func(o *uploadOptions) { o.title = title }
}

// Upload ingests a new document into the Client's context (POST /documents).
//
// body supplies the file bytes; pass an *os.File for filesystem paths or
// a bytes.Reader for in-memory uploads. The Reader is consumed in full
// before the request fires (so the body size is known and the request can
// be sent in a single shot).
//
// Upload is not idempotent; transient failures are surfaced rather than
// retried.
func (d *Documents) Upload(ctx context.Context, body io.Reader, opts ...UploadOption) (*UploadResponse, error) {
	return d.upload(ctx, http.MethodPost, d.client.base+"/documents", body, opts...)
}

// upload performs a multipart document write. method and path select the
// endpoint so Upload (POST /documents) and Reprocess (PUT /documents/{id})
// share one implementation.
//
// The body matches the spec's DocumentUploadForm: an optional "metadata" JSON
// part (carrying scopes/labels/title) followed by the "file" part. The server
// reads the metadata part before the file part, so it MUST be written first.
func (d *Documents) upload(ctx context.Context, method, path string, body io.Reader, opts ...UploadOption) (*UploadResponse, error) {
	if body == nil {
		return nil, &APIError{Message: "upload body is required"}
	}
	o := uploadOptions{
		filename:    "upload",
		contentType: "application/octet-stream",
	}
	for _, opt := range opts {
		opt(&o)
	}

	fileBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, &APIError{Message: fmt.Sprintf("read upload body: %v", err)}
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Optional metadata part, written before the file part so the server can
	// read scopes/labels/title before it streams the bytes (matching
	// UploadMetadataJson). Field names mirror the spec's camelCase shape.
	if len(o.scopes) > 0 || len(o.labels) > 0 || o.title != "" {
		meta := struct {
			Scopes ScopeSets `json:"scopes,omitempty"`
			Labels []string  `json:"labels,omitempty"`
			Title  string    `json:"title,omitempty"`
		}{Scopes: o.scopes, Labels: o.labels, Title: o.title}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return nil, &APIError{Message: fmt.Sprintf("marshal metadata: %v", err)}
		}
		metaHeader := textproto.MIMEHeader{}
		metaHeader.Set("Content-Disposition", `form-data; name="metadata"`)
		metaHeader.Set("Content-Type", "application/json")
		metaPart, err := mw.CreatePart(metaHeader)
		if err != nil {
			return nil, &APIError{Message: fmt.Sprintf("build multipart: %v", err)}
		}
		if _, err := metaPart.Write(metaJSON); err != nil {
			return nil, &APIError{Message: fmt.Sprintf("write multipart metadata: %v", err)}
		}
	}

	// File part.
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(
		`form-data; name="file"; filename=%q`, sanitiseFilename(o.filename),
	))
	partHeader.Set("Content-Type", o.contentType)
	filePart, err := mw.CreatePart(partHeader)
	if err != nil {
		return nil, &APIError{Message: fmt.Sprintf("build multipart: %v", err)}
	}
	if _, err := filePart.Write(fileBytes); err != nil {
		return nil, &APIError{Message: fmt.Sprintf("write multipart file: %v", err)}
	}

	if err := mw.Close(); err != nil {
		return nil, &APIError{Message: fmt.Sprintf("close multipart: %v", err)}
	}

	resp, err := d.client.do(
		ctx,
		method,
		path,
		buf.Bytes(),
		mw.FormDataContentType(),
		nil,
		false,
		false,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("read upload response: %v", err),
		}
	}
	var out UploadResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("decode upload response: %v", err),
			Body:       decodeJSON(data),
		}
	}
	return &out, nil
}

// sanitiseFilename strips path separators and quotes from a filename so it
// can be safely embedded in a Content-Disposition header.
func sanitiseFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}
