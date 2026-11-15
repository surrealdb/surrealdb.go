package memory

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestShouldRetryGET(t *testing.T) {
	if !shouldRetry(http.MethodGet, 0, 0, 3, false) {
		t.Error("GET with transport failure should retry")
	}
	if !shouldRetry(http.MethodGet, 503, 0, 3, false) {
		t.Error("GET 503 should retry")
	}
	if shouldRetry(http.MethodGet, 400, 0, 3, false) {
		t.Error("GET 400 should not retry")
	}
}

func TestShouldRetryPOSTIdempotent(t *testing.T) {
	if !shouldRetry(http.MethodPost, 0, 0, 3, true) {
		t.Error("idempotent POST transport failure should retry")
	}
	if !shouldRetry(http.MethodPost, 502, 0, 3, true) {
		t.Error("idempotent POST 502 should retry")
	}
	if shouldRetry(http.MethodPost, 0, 0, 3, false) {
		t.Error("non-idempotent POST should not retry")
	}
}

func TestShouldRetryExhausts(t *testing.T) {
	if shouldRetry(http.MethodGet, 503, 3, 3, false) {
		t.Error("attempts >= max should stop retrying")
	}
}

func TestBackoffSchedule(t *testing.T) {
	if got := len(backoffFor(0)); got != 0 {
		t.Errorf("backoffFor(0) len = %d, want 0", got)
	}
	if got := len(backoffFor(2)); got != 2 {
		t.Errorf("backoffFor(2) len = %d, want 2", got)
	}
	if got := len(backoffFor(99)); got != len(backoffSchedule) {
		t.Errorf("backoffFor(99) len = %d, want %d", got, len(backoffSchedule))
	}
}

// retryFailureTransport exercises retries without a running server.
type retryFailureTransport struct {
	attempts int
	status   int
}

func (tr *retryFailureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	tr.attempts++
	if tr.status == 0 {
		return nil, errors.New("connection unavailable")
	}
	return &http.Response{
		StatusCode: tr.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"error":"temporarily unavailable"}`)),
	}, nil
}

func TestRetriesClampedToSchedule(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, 0} {
		name := "http"
		if status == 0 {
			name = "transport"
		}
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name         string
				maxRetries   int
				wantAttempts int
			}{
				{"above_schedule", 5, 4},
				{"within_schedule", 1, 2},
				{"zero", 0, 1},
				{"negative", -1, 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					client, err := New("demo", "https://memory.invalid", "token", WithMaxRetries(tc.maxRetries))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = client.Close() })
					transport := &retryFailureTransport{status: status}
					client.http.Transport = transport
					err = client.Health(context.Background())
					var apiErr *APIError
					if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
						t.Fatalf("Health error = %v, want APIError with status %d", err, status)
					}
					if transport.attempts != tc.wantAttempts {
						t.Errorf("attempts = %d, want %d", transport.attempts, tc.wantAttempts)
					}
				})
			}
		})
	}
}
