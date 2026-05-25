package spectron

import (
	"net/http"
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
