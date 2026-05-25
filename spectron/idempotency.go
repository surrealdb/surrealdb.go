package spectron

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// idempotencyBucketSeconds is the window during which an identical request
// collapses onto the same Idempotency-Key. Matches the Python SDK.
const idempotencyBucketSeconds = 30

// idempotencyKey produces a deterministic key for a method+path+body within
// a fixed-size time bucket. A retry within the same bucket reuses the same
// key, so the server can deduplicate it.
func idempotencyKey(method, path string, body []byte, now time.Time) string {
	bucket := now.Unix() / idempotencyBucketSeconds
	h := sha256.New()
	h.Write([]byte(strings.ToUpper(method)))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(body)
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatInt(bucket, 10)))
	return hex.EncodeToString(h.Sum(nil))
}
