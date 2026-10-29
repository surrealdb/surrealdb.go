package memory

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

	buf, contentType, err := buildUploadBody(&o, fileBytes)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.do(ctx, method, path, buf.Bytes(), contentType, nil, false, false)
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

// buildUploadBody assembles the DocumentUploadForm multipart body: an optional
// "metadata" JSON part (scopes/labels/title) written before the "file" part, in
// the order the server parses them. It returns the body buffer and the
// content type to send.
func buildUploadBody(o *uploadOptions, fileBytes []byte) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Metadata part first so the server can read scopes/labels/title before it
	// streams the bytes (matching UploadMetadataJson's camelCase shape).
	if len(o.scopes) > 0 || len(o.labels) > 0 || o.title != "" {
		meta := struct {
			Scopes ScopeSets `json:"scopes,omitempty"`
			Labels []string  `json:"labels,omitempty"`
			Title  string    `json:"title,omitempty"`
		}{Scopes: o.scopes, Labels: o.labels, Title: o.title}
		metaJSON, marshalErr := json.Marshal(meta)
		if marshalErr != nil {
			return nil, "", &APIError{Message: fmt.Sprintf("marshal metadata: %v", marshalErr)}
		}
		if err := writeMultipartPart(mw, `form-data; name="metadata"`, "application/json", metaJSON); err != nil {
			return nil, "", err
		}
	}

	disposition := fmt.Sprintf(`form-data; name="file"; filename=%q`, sanitiseFilename(o.filename))
	if err := writeMultipartPart(mw, disposition, o.contentType, fileBytes); err != nil {
		return nil, "", err
	}

	if err := mw.Close(); err != nil {
		return nil, "", &APIError{Message: fmt.Sprintf("close multipart: %v", err)}
	}
	return &buf, mw.FormDataContentType(), nil
}

// writeMultipartPart writes a single multipart part with the given
// Content-Disposition, content type, and body bytes.
func writeMultipartPart(mw *multipart.Writer, disposition, contentType string, content []byte) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", disposition)
	header.Set("Content-Type", contentType)
	part, err := mw.CreatePart(header)
	if err != nil {
		return &APIError{Message: fmt.Sprintf("build multipart: %v", err)}
	}
	if _, err = part.Write(content); err != nil {
		return &APIError{Message: fmt.Sprintf("write multipart: %v", err)}
	}
	return nil
}

// sanitiseFilename strips path separators and quotes from a filename so it
// can be safely embedded in a Content-Disposition header.
func sanitiseFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}
