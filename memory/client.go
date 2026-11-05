package memory

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

// headerOnBehalfOf is the delegation header. When a request carries it, the
// server attributes the work to the named principal, subject to the calling
// token's own grants. Set on a derived Client via [Client.OnBehalfOf].
const headerOnBehalfOf = "X-Spectron-On-Behalf-Of"

// Client is an Agent Memory API client pinned to a single context.
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

	// onBehalfOf, when non-empty, is sent as the X-Spectron-On-Behalf-Of
	// delegation header on every request. Set via [Client.OnBehalfOf].
	onBehalfOf string

	docs       *Documents
	entities   *Entities
	scopes     *Scopes
	sessions   *Sessions
	principals *Principals
	keys       *Keys
	traces     *Traces
	facts      *Facts
	uncertain  *Uncertainty
}

// New constructs an Agent Memory Client.
//
// All three positional arguments are required:
//   - contextID: context id, e.g. "acme-prod".
//   - endpoint:  full URL of the Agent Memory host, e.g. "https://api.memory.example".
//   - apiKey:    bearer token, sent as Authorization: Bearer <key>.
//
// Options may override timeout, retry cap, or user agent. The SDK never
// reads environment variables; pass secrets explicitly.
func New(contextID, endpoint, apiKey string, opts ...Option) (*Client, error) {
	if contextID == "" {
		return nil, errors.New("memory: context is required")
	}
	if endpoint == "" {
		return nil, errors.New("memory: endpoint is required")
	}
	if apiKey == "" {
		return nil, errors.New("memory: api key is required")
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
	c.bindNamespaces()
	return c, nil
}

// bindNamespaces (re)points every sub-client at c. Called once on construction
// and again on each [Client.OnBehalfOf] clone so the sub-clients delegate too.
func (c *Client) bindNamespaces() {
	c.docs = &Documents{client: c}
	c.entities = &Entities{client: c}
	c.scopes = &Scopes{client: c}
	c.sessions = &Sessions{client: c}
	c.principals = &Principals{client: c}
	c.keys = &Keys{client: c}
	c.traces = &Traces{client: c}
	c.facts = &Facts{client: c}
	c.uncertain = &Uncertainty{client: c}
}

// OnBehalfOf returns a derived Client that attributes every request to the
// named principal via the X-Spectron-On-Behalf-Of header. The server still
// enforces the calling token's own grants, so delegation can only narrow
// access. The returned Client shares the underlying HTTP transport with the
// receiver; do not call [Client.Close] on both. An empty principal id returns
// an undelegated clone.
//
//	hits, err := client.OnBehalfOf("user:bob").Recall(ctx, req)
func (c *Client) OnBehalfOf(principalID string) *Client {
	clone := *c
	clone.onBehalfOf = principalID
	clone.bindNamespaces()
	return &clone
}

// Close releases idle connections held by the Client. It is safe to call
// multiple times. Outstanding in-flight requests are not canceled.
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

// Entities returns the entity sub-client.
func (c *Client) Entities() *Entities { return c.entities }

// Scopes returns the scope sub-client.
func (c *Client) Scopes() *Scopes { return c.scopes }

// Sessions returns the session sub-client.
func (c *Client) Sessions() *Sessions { return c.sessions }

// Principals returns the principal sub-client.
func (c *Client) Principals() *Principals { return c.principals }

// Keys returns the self-service key sub-client.
func (c *Client) Keys() *Keys { return c.keys }

// Traces returns the trace sub-client.
func (c *Client) Traces() *Traces { return c.traces }

// Facts returns the fact sub-client, which walks the attribute, relation and
// action collections.
func (c *Client) Facts() *Facts { return c.facts }

// Uncertainty returns the uncertainty sub-client, which lists what the context
// knows it is unsure about and settles those flags.
func (c *Client) Uncertainty() *Uncertainty { return c.uncertain }

// getJSON issues a GET to path with the supplied query parameters and decodes
// the JSON response into dst. GETs are safe to retry on 5xx and transport
// failures, so they carry no Idempotency-Key but still pass through the retry
// loop (see shouldRetry).
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, dst any) error {
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return c.doJSON(ctx, http.MethodGet, path, nil, dst, false)
}

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
			return fmt.Errorf("memory: marshal request body: %w", err)
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
	reqURL := c.buildURL(path)
	headers := c.baseHeaders(contentType)
	for k, vs := range extraHeaders {
		for _, v := range vs {
			headers.Set(k, v)
		}
	}
	// Delegation: a Client derived via OnBehalfOf carries a principal id that is
	// attributed to every request it issues. Applied here, the single transport
	// choke point, so JSON, multipart, SSE, and raw reads all delegate alike.
	if c.onBehalfOf != "" {
		headers.Set(headerOnBehalfOf, c.onBehalfOf)
	}
	if stream {
		// Caller asked for a streaming response; ask the server to keep the
		// connection open.
		headers.Set("Accept", "text/event-stream")
	}

	schedule := backoffFor(c.cfg.maxRetries)
	method = strings.ToUpper(method)

	for attempt := 0; ; attempt++ {
		resp, retry, err := c.attemptOnce(ctx, method, reqURL, bodyBytes, headers, attempt, idempotent, stream)
		if err != nil {
			return nil, err
		}
		if !retry {
			return resp, nil
		}
		if err := sleepCtx(ctx, schedule[attempt]); err != nil {
			return nil, &APIError{Message: fmt.Sprintf("connection failed: %v", err)}
		}
	}
}

// attemptOnce performs a single HTTP exchange. It returns the response on
// success; otherwise retry reports whether the caller should back off and try
// again, and err carries the terminal error when retry is false.
func (c *Client) attemptOnce(
	ctx context.Context,
	method, reqURL string,
	bodyBytes []byte,
	headers http.Header,
	attempt int,
	idempotent, stream bool,
) (resp *http.Response, retry bool, err error) {
	// requestContext returns a non-nil cancel (a no-op when there is no
	// per-attempt timeout) so every exit path can call it unconditionally.
	reqCtx, cancel := c.requestContext(ctx, stream)

	req, err := http.NewRequestWithContext(reqCtx, method, reqURL, bodyReaderFor(bodyBytes))
	if err != nil {
		cancel()
		return nil, false, &APIError{Message: fmt.Sprintf("build request: %v", err)}
	}
	req.Header = headers.Clone()

	resp, err = c.http.Do(req)
	if err != nil {
		cancel()
		if !shouldRetry(method, 0, attempt, c.cfg.maxRetries, idempotent) {
			return nil, false, &APIError{Message: fmt.Sprintf("connection failed: %v", err)}
		}
		return nil, true, nil
	}

	status := resp.StatusCode
	switch {
	case status >= 400 && shouldRetry(method, status, attempt, c.cfg.maxRetries, idempotent):
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		cancel()
		return nil, true, nil
	case status >= 400:
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		return nil, false, errorFromResponse(status, decodeJSON(data), resp.Header)
	}

	// Successful response. The per-attempt timeout context is still attached to
	// the request; canceling it now would terminate the body read in flight, so
	// keep it alive by tying cancel to body close.
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, false, nil
}

// requestContext derives the per-attempt context. When a timeout is configured
// for a non-streaming call it returns a timeout context; otherwise it returns
// the parent context paired with a no-op cancel, so callers never need a nil
// check before releasing it.
func (c *Client) requestContext(ctx context.Context, stream bool) (context.Context, context.CancelFunc) {
	if !stream && c.cfg.timeout > 0 {
		return context.WithTimeout(ctx, c.cfg.timeout)
	}
	return ctx, func() {}
}

// bodyReaderFor wraps a replayable JSON body for a single attempt, returning a
// nil reader (a bodyless request) when there are no bytes to send.
func bodyReaderFor(bodyBytes []byte) io.Reader {
	if bodyBytes == nil {
		return nil
	}
	return bytes.NewReader(bodyBytes)
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

// sleepCtx sleeps for d, returning early if ctx is canceled.
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
