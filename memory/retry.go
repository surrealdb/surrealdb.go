package memory

import (
	"net/http"
	"time"
)

// backoffSchedule mirrors the Python SDK: 250ms, 500ms, 1s, capped at the
// configured maxRetries.
var backoffSchedule = []time.Duration{
	250 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
}

func backoffFor(maxRetries int) []time.Duration {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > len(backoffSchedule) {
		maxRetries = len(backoffSchedule)
	}
	return backoffSchedule[:maxRetries]
}

// shouldRetry returns true when an attempt that produced the given status
// (or no status, for transport-level failures) is worth retrying. Only GET
// and idempotent writes are eligible; 5xx and connection failures trigger
// a retry, 4xx never does.
//
// status == 0 means "no response received" (transport failure).
func shouldRetry(method string, status, attempt, maxRetries int, idempotent bool) bool {
	if attempt >= maxRetries {
		return false
	}
	if method != http.MethodGet && !idempotent {
		return false
	}
	if status == 0 {
		return true
	}
	return status >= 500
}
