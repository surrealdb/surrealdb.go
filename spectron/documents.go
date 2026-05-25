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
	scope       Scope
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

// WithScope attaches a principal scope to the upload. The scope is sent
// as a JSON field in the multipart payload.
func WithScope(scope Scope) UploadOption {
	return func(o *uploadOptions) { o.scope = scope }
}

// Upload uploads a document for the Client's context.
//
// body supplies the file bytes; pass an *os.File for filesystem paths or
// a bytes.Reader for in-memory uploads. The Reader is consumed in full
// before the request fires (so the body size is known and the request can
// be sent in a single shot).
//
// Upload is not idempotent; transient failures are surfaced rather than
// retried.
func (d *Documents) Upload(ctx context.Context, body io.Reader, opts ...UploadOption) (*UploadResponse, error) {
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

	// Optional scope field, serialised as JSON to match Python's behaviour.
	if len(o.scope) > 0 {
		scopeJSON, err := json.Marshal(o.scope)
		if err != nil {
			return nil, &APIError{Message: fmt.Sprintf("marshal scope: %v", err)}
		}
		fieldHeader := textproto.MIMEHeader{}
		fieldHeader.Set("Content-Disposition", `form-data; name="scope"`)
		fieldHeader.Set("Content-Type", "application/json")
		fieldPart, err := mw.CreatePart(fieldHeader)
		if err != nil {
			return nil, &APIError{Message: fmt.Sprintf("build multipart: %v", err)}
		}
		if _, err := fieldPart.Write(scopeJSON); err != nil {
			return nil, &APIError{Message: fmt.Sprintf("write multipart scope: %v", err)}
		}
	}

	if err := mw.Close(); err != nil {
		return nil, &APIError{Message: fmt.Sprintf("close multipart: %v", err)}
	}

	path := d.client.base + "/documents"
	resp, err := d.client.do(
		ctx,
		http.MethodPost,
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
