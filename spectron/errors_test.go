package spectron

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorFromResponseSentinels(t *testing.T) {
	cases := []struct {
		status   int
		sentinel error
	}{
		{http.StatusUnauthorized, ErrAuth},
		{http.StatusForbidden, ErrScope},
		{http.StatusNotFound, ErrNotFound},
	}
	for _, tc := range cases {
		err := errorFromResponse(tc.status, map[string]any{"message": "nope"}, nil)
		if !errors.Is(err, tc.sentinel) {
			t.Errorf("status %d: errors.Is should match %v", tc.status, tc.sentinel)
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Errorf("status %d: errors.As *APIError failed", tc.status)
			continue
		}
		if apiErr.StatusCode != tc.status {
			t.Errorf("status %d: APIError.StatusCode = %d", tc.status, apiErr.StatusCode)
		}
		if apiErr.Message != "nope" {
			t.Errorf("status %d: APIError.Message = %q", tc.status, apiErr.Message)
		}
	}
}

func TestErrorFromResponseGenericStatus(t *testing.T) {
	err := errorFromResponse(http.StatusBadRequest, map[string]any{"title": "bad"}, nil)
	if errors.Is(err, ErrAuth) || errors.Is(err, ErrScope) || errors.Is(err, ErrNotFound) {
		t.Error("400 should not match a status sentinel")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("expected *APIError")
	}
	if apiErr.Message != "bad" {
		t.Errorf("expected message 'bad', got %q", apiErr.Message)
	}
}

func TestErrorTraceID(t *testing.T) {
	h := http.Header{}
	h.Set("X-Trace-Id", "trace-123")
	err := errorFromResponse(500, nil, h)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("expected *APIError")
	}
	if apiErr.TraceID != "trace-123" {
		t.Errorf("TraceID = %q", apiErr.TraceID)
	}
}

func TestErrorMessageFallbacks(t *testing.T) {
	cases := []struct {
		body any
		want string
	}{
		{map[string]any{"message": "m"}, "m"},
		{map[string]any{"title": "t"}, "t"},
		{map[string]any{"error": "e"}, "e"},
		{"raw text", "raw text"},
		{nil, "request failed"},
	}
	for _, tc := range cases {
		err := errorFromResponse(500, tc.body, nil)
		if err.Message != tc.want {
			t.Errorf("body %v: message = %q, want %q", tc.body, err.Message, tc.want)
		}
	}
}
