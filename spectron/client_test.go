package spectron

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient spins up a Spectron client pointing at the provided
// httptest.Server with a short retry schedule for fast tests.
func newTestClient(t *testing.T, srv *httptest.Server, opts ...Option) *Client {
	t.Helper()
	opts = append([]Option{WithTimeout(2 * time.Second), WithMaxRetries(3)}, opts...)
	c, err := New("ctx-1", srv.URL, "sk-test", opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestRememberSendsBearerAndIdempotencyKey(t *testing.T) {
	var gotAuth, gotIdem, gotCT, gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotIdem = r.Header.Get("Idempotency-Key")
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"mode":"fact","sessionId":"s1","turnId":"t1"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Remember(context.Background(), RememberRequest{
		Text:      "hi",
		SessionID: "s1",
		Scope:     Scope{"org": "acme"},
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if resp.Mode != "fact" || resp.SessionID != "s1" || resp.TurnID != "t1" {
		t.Errorf("unexpected response %+v", resp)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/api/v1/ctx-1/facts" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotIdem == "" || len(gotIdem) != 64 {
		t.Errorf("idempotency key = %q", gotIdem)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotBody["text"] != "hi" {
		t.Errorf("body text = %v", gotBody["text"])
	}
	scope, ok := gotBody["scope"].([]any)
	if !ok || len(scope) != 1 {
		t.Errorf("scope wire shape = %#v", gotBody["scope"])
	} else {
		first := scope[0].(map[string]any)
		if first["key"] != "org" || first["value"] != "acme" {
			t.Errorf("scope[0] = %v", first)
		}
	}
}

func TestRecallNoIdempotencyKey(t *testing.T) {
	var gotIdem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdem = r.Header.Get("Idempotency-Key")
		_, _ = io.WriteString(w, `{"hits":[{"id":"x","score":0.9,"source":"s","text":"t"}]}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Recall(context.Background(), RecallRequest{Query: "q"})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if gotIdem != "" {
		t.Errorf("Recall should not send Idempotency-Key, got %q", gotIdem)
	}
	if len(resp.Hits) != 1 || resp.Hits[0].ID != "x" {
		t.Errorf("hits = %+v", resp.Hits)
	}
}

func TestRetryOn503Succeeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "boom", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"mode":"fact","sessionId":"s","turnId":"t"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Remember(context.Background(), RememberRequest{Text: "x"})
	if err != nil {
		t.Fatalf("Remember after retries: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 server calls, got %d", got)
	}
}

func TestNoRetryOn400(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"message":"nope"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Remember(context.Background(), RememberRequest{Text: "x"})
	if err == nil {
		t.Fatal("expected error on 400")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("expected APIError 400, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 call (no retry), got %d", got)
	}
}

func TestNotFoundMapsToErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace-Id", "tr-1")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"gone"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Recall(context.Background(), RecallRequest{Query: "q"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false; err=%v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("errors.As *APIError failed")
	}
	if apiErr.TraceID != "tr-1" {
		t.Errorf("TraceID = %q", apiErr.TraceID)
	}
}

func TestAuthAndScopeMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{401, ErrAuth},
		{403, ErrScope},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, `{"message":"x"}`)
		}))
		c := newTestClient(t, srv)
		_, err := c.Recall(context.Background(), RecallRequest{Query: "q"})
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d -> want %v, got %v", tc.status, tc.want, err)
		}
		srv.Close()
	}
}

func TestForgetWithPurge(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"deleted":2}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Forget(context.Background(), "old", WithPurge())
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if resp.Deleted != 2 {
		t.Errorf("Deleted = %d", resp.Deleted)
	}
	if body["query"] != "old" {
		t.Errorf("query = %v", body["query"])
	}
	if got, _ := body["purge"].(bool); !got {
		t.Errorf("purge = %v", body["purge"])
	}
}

func TestChatNonStreaming(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"reply":"hi","sessionId":"s","traceId":"t"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Chat(context.Background(), ChatRequest{Message: "hello"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Reply != "hi" {
		t.Errorf("reply = %q", resp.Reply)
	}
	if body["stream"] != nil {
		t.Errorf("non-streaming Chat should omit stream, got %v", body["stream"])
	}
}

func TestChatStreamEndToEnd(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, "data: {\"delta\":\"hel\"}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "data: {\"delta\":\"lo\"}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	var got strings.Builder
	var done bool
	for chunk, err := range c.ChatStream(context.Background(), ChatRequest{Message: "hi"}) {
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		got.WriteString(chunk.Delta)
		if chunk.Done {
			done = true
			break
		}
	}
	if got.String() != "hello" {
		t.Errorf("delta total = %q, want %q", got.String(), "hello")
	}
	if !done {
		t.Error("expected Done chunk")
	}
	if body["stream"] != true {
		t.Errorf("expected stream=true on wire, got %v", body["stream"])
	}
}

func TestNewRejectsMissingArgs(t *testing.T) {
	if _, err := New("", "http://x", "k"); err == nil {
		t.Error("missing context should error")
	}
	if _, err := New("c", "", "k"); err == nil {
		t.Error("missing endpoint should error")
	}
	if _, err := New("c", "http://x", ""); err == nil {
		t.Error("missing api key should error")
	}
}

func TestContextEndpointAccessors(t *testing.T) {
	c, _ := New("c", "http://x/", "k")
	if c.Context() != "c" {
		t.Errorf("Context = %q", c.Context())
	}
	if c.Endpoint() != "http://x" {
		t.Errorf("Endpoint = %q (should be trimmed)", c.Endpoint())
	}
}
