package spectron

import (
	"strings"
	"testing"
	"time"
)

func TestIdempotencyKeyStableWithinBucket(t *testing.T) {
	// Land exactly on a bucket boundary so +29s is still inside it.
	base := int64(1_700_000_000) - (1_700_000_000 % idempotencyBucketSeconds)
	t0 := time.Unix(base, 0)
	t1 := time.Unix(base+idempotencyBucketSeconds-1, 0)

	a := idempotencyKey("POST", "/api/v1/ctx/facts", []byte(`{"text":"x"}`), t0)
	b := idempotencyKey("POST", "/api/v1/ctx/facts", []byte(`{"text":"x"}`), t1)

	if a != b {
		t.Fatalf("expected same key within bucket, got %s vs %s", a, b)
	}
	if len(a) != 64 || !isHex(a) {
		t.Fatalf("expected 64-char hex key, got %q", a)
	}
}

func TestIdempotencyKeyDiffersAcrossBuckets(t *testing.T) {
	base := int64(1_700_000_000) - (1_700_000_000 % idempotencyBucketSeconds)
	t0 := time.Unix(base, 0)
	t1 := time.Unix(base+idempotencyBucketSeconds, 0)

	a := idempotencyKey("POST", "/p", []byte(`{}`), t0)
	b := idempotencyKey("POST", "/p", []byte(`{}`), t1)

	if a == b {
		t.Fatalf("expected different keys across buckets")
	}
}

func TestIdempotencyKeyDiffersByMethodPathBody(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	base := idempotencyKey("POST", "/a", []byte("x"), t0)

	cases := map[string]string{
		"method": idempotencyKey("PUT", "/a", []byte("x"), t0),
		"path":   idempotencyKey("POST", "/b", []byte("x"), t0),
		"body":   idempotencyKey("POST", "/a", []byte("y"), t0),
	}
	for name, other := range cases {
		if other == base {
			t.Errorf("expected key to differ when %s changes", name)
		}
	}
}

func TestIdempotencyKeyMethodCaseInsensitive(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	upper := idempotencyKey("POST", "/a", []byte("x"), t0)
	lower := idempotencyKey("post", "/a", []byte("x"), t0)
	if upper != lower {
		t.Fatalf("expected case-insensitive method, got %s vs %s", upper, lower)
	}
}

func isHex(s string) bool {
	const hex = "0123456789abcdef"
	for _, r := range s {
		if !strings.ContainsRune(hex, r) {
			return false
		}
	}
	return true
}
