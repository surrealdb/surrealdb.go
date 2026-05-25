package spectron

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Spectron API client pinned to a single context.
//
// A Client is safe for concurrent use; the underlying *http.Client is
// reused across requests. Call [Client.Close] when the Client is no
// longer needed to release idle connections.
type Client struct {
	contextID string
	endpoint  string // trimmed of trailing slash
	apiKey    string
	cfg       config
	http      *http.Client
	base      string // /api/v1/{context}

	docs *Documents
}

// New constructs a Spectron Client.
//
// All three positional arguments are required:
//   - contextID — context id, e.g. "acme-prod".
//   - endpoint  — full URL of the Spectron host, e.g. "https://api.spectron.example".
//   - apiKey    — bearer token, sent as Authorization: Bearer <key>.
//
// Options may override timeout, retry cap, or user agent. The SDK never
// reads environment variables; pass secrets explicitly.
func New(contextID, endpoint, apiKey string, opts ...Option) (*Client, error) {
	if contextID == "" {
		return nil, errors.New("spectron: context is required")
	}
	if endpoint == "" {
		return nil, errors.New("spectron: endpoint is required")
	}
	if apiKey == "" {
		return nil, errors.New("spectron: api key is required")
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	c := &Client{
		contextID: contextID,
		endpoint:  strings.TrimRight(endpoint, "/"),
		apiKey:    apiKey,
		cfg:       cfg,
		http:      &http.Client{},
		base:      "/api/v1/" + url.PathEscape(contextID),
	}
	c.docs = &Documents{client: c}
	return c, nil
}

// Close releases idle connections held by the Client. It is safe to call
// multiple times. Outstanding in-flight requests are not cancelled.
func (c *Client) Close() error {
	c.http.CloseIdleConnections()
	return nil
}

// Context returns the context id the Client was constructed with.
func (c *Client) Context() string { return c.contextID }

// Endpoint returns the base URL the Client targets.
func (c *Client) Endpoint() string { return c.endpoint }

// Documents returns the document sub-client.
func (c *Client) Documents() *Documents { return c.docs }

func (c *Client) buildURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.endpoint + path
}

func (c *Client) baseHeaders(contentType string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.apiKey)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", c.cfg.userAgent)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return h
}

// doJSON marshals payload (if non-nil) as JSON, executes the request with
// the retry loop, and decodes the response body into dst (if non-nil).
//
// When idempotent is true, the request additionally carries an
// Idempotency-Key derived from the encoded body and a 30s bucket; the
// request is retried on transport failures and 5xx responses (along with
// GETs, which are always considered idempotent).
func (c *Client) doJSON(ctx context.Context, method, path string, payload, dst any, idempotent bool) error {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("spectron: marshal request body: %w", err)
		}
	}

	contentType := ""
	if body != nil {
		contentType = "application/json"
	}

	extra := http.Header{}
	if idempotent {
		extra.Set("Idempotency-Key", idempotencyKey(method, path, body, time.Now()))
	}

	resp, err := c.do(ctx, method, path, body, contentType, extra, idempotent, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || dst == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{Message: fmt.Sprintf("read response: %v", err)}
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("decode response: %v", err),
			Body:       decodeJSON(data),
		}
	}
	return nil
}

// do executes a single HTTP exchange with the retry loop. The bodyBytes
// argument carries a JSON body that can be safely replayed across retries;
// callers that need to send non-replayable bodies (multipart, SSE bodies)
// should ensure idempotent=false so retries never fire.
//
// When stream is true the caller receives the live *http.Response and is
// responsible for closing the body; otherwise the body is left open for
// the immediate caller and the deferred close is its responsibility.
func (c *Client) do(
	ctx context.Context,
	method, path string,
	bodyBytes []byte,
	contentType string,
	extraHeaders http.Header,
	idempotent bool,
	stream bool,
) (*http.Response, error) {
	url := c.buildURL(path)
	headers := c.baseHeaders(contentType)
	for k, vs := range extraHeaders {
		for _, v := range vs {
			headers.Set(k, v)
		}
	}
	if stream {
		// Caller asked for a streaming response; ask the server to keep the
		// connection open.
		headers.Set("Accept", "text/event-stream")
	}

	schedule := backoffFor(c.cfg.maxRetries)
	attempt := 0
	method = strings.ToUpper(method)

	for {
		reqCtx := ctx
		var cancel context.CancelFunc
		if !stream && c.cfg.timeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, c.cfg.timeout)
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return nil, &APIError{Message: fmt.Sprintf("build request: %v", err)}
		}
		req.Header = headers.Clone()

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			if cancel != nil {
				cancel()
			}
			if !shouldRetry(method, 0, attempt, c.cfg.maxRetries, idempotent) {
				return nil, &APIError{Message: fmt.Sprintf("connection failed: %v", doErr)}
			}
			if err := sleepCtx(ctx, schedule[attempt]); err != nil {
				return nil, &APIError{Message: fmt.Sprintf("connection failed: %v", err)}
			}
			attempt++
			continue
		}

		status := resp.StatusCode
		if status >= 400 && shouldRetry(method, status, attempt, c.cfg.maxRetries, idempotent) {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if cancel != nil {
				cancel()
			}
			if err := sleepCtx(ctx, schedule[attempt]); err != nil {
				return nil, &APIError{Message: fmt.Sprintf("connection failed: %v", err)}
			}
			attempt++
			continue
		}

		if status >= 400 {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if cancel != nil {
				cancel()
			}
			return nil, errorFromResponse(status, decodeJSON(data), resp.Header)
		}

		// Successful response. For non-streaming calls we rely on the per-
		// attempt timeout context already attached to the request; cancelling
		// it now would terminate the body read in flight. Keep the cancel
		// alive by deferring it to body close via a wrapper.
		if cancel != nil {
			resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
		}
		return resp, nil
	}
}

// cancelOnClose ties a context.CancelFunc to the lifetime of a response
// body so that the per-attempt timeout context lives long enough for the
// caller to finish reading.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// sleepCtx sleeps for d, returning early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
