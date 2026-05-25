package spectron

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

func backoffFor(max int) []time.Duration {
	if max < 0 {
		max = 0
	}
	if max > len(backoffSchedule) {
		max = len(backoffSchedule)
	}
	return backoffSchedule[:max]
}

// shouldRetry returns true when an attempt that produced the given status
// (or no status, for transport-level failures) is worth retrying. Only GET
// and idempotent writes are eligible; 5xx and connection failures trigger
// a retry, 4xx never does.
//
// status == 0 means "no response received" (transport failure).
func shouldRetry(method string, status, attempt, max int, idempotent bool) bool {
	if attempt >= max {
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
