package surrealcbor_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

type timeDoc struct {
	ID   string     `cbor:"id"`
	At   time.Time  `cbor:"at"`
	Ptr  *time.Time `cbor:"ptr"`
	List []any      `cbor:"list"`
}

func rawTag12(t *testing.T, v any, sec, ns int64) {
	t.Helper()
	tag, ok := v.(cbor.Tag)
	if !ok || tag.Number != models.TagCustomDatetime {
		t.Fatalf("want tag 12, got %#v", v)
	}
	arr := tag.Content.([]any)
	toI := func(x any) int64 {
		switch n := x.(type) {
		case int64:
			return n
		case uint64:
			return int64(n)
		}
		t.Fatalf("not an integer: %#v", x)
		return 0
	}
	if len(arr) != 2 || toI(arr[0]) != sec || toI(arr[1]) != ns {
		t.Fatalf("got %#v, want [%d, %d]", arr, sec, ns)
	}
}

func TestBareTimeEncodesAsTag12(t *testing.T) {
	at := time.Unix(1700000000, 123456789).UTC()
	pre := time.Date(1950, 1, 1, 0, 0, 0, 7, time.UTC)
	doc := timeDoc{ID: "a", At: at, Ptr: &pre, List: []any{at}}

	encoders := map[string]func(any) ([]byte, error){
		"Marshal": surrealcbor.Marshal,
		"Codec":   surrealcbor.New().Marshal,
		"Encoder": func(v any) ([]byte, error) {
			var buf bytes.Buffer
			err := surrealcbor.NewEncoder(&buf).Encode(v)
			return buf.Bytes(), err
		},
	}
	for name, enc := range encoders {
		t.Run(name, func(t *testing.T) {
			b, err := enc(doc)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := cbor.Unmarshal(b, &raw); err != nil {
				t.Fatal(err)
			}
			rawTag12(t, raw["at"], at.Unix(), 123456789)
			rawTag12(t, raw["ptr"], pre.Unix(), 7) // negative seconds
			rawTag12(t, raw["list"].([]any)[0], at.Unix(), 123456789)

			// Tag 12 decodes into plain time.Time and into CustomDateTime.
			var back timeDoc
			if err := surrealcbor.Unmarshal(b, &back); err != nil {
				t.Fatal(err)
			}
			if !back.At.Equal(at) || back.At.Nanosecond() != 123456789 {
				t.Fatalf("time.Time round trip: got %v, want %v", back.At, at)
			}
			if back.Ptr == nil || !back.Ptr.Equal(pre) {
				t.Fatalf("*time.Time round trip: got %v, want %v", back.Ptr, pre)
			}
			var wrapped struct {
				At models.CustomDateTime `cbor:"at"`
			}
			if err := surrealcbor.Unmarshal(b, &wrapped); err != nil {
				t.Fatal(err)
			}
			if !wrapped.At.Time.Equal(at) {
				t.Fatalf("CustomDateTime round trip: got %v, want %v", wrapped.At.Time, at)
			}
		})
	}
}

func TestZeroAndWrappedTimeUnchanged(t *testing.T) {
	zero, err := surrealcbor.Marshal(struct {
		At time.Time `cbor:"at"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	wrappedZero, err := surrealcbor.Marshal(struct {
		At models.CustomDateTime `cbor:"at"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(zero, wrappedZero) {
		t.Fatalf("zero time %x != zero CustomDateTime %x", zero, wrappedZero)
	}
	var raw map[string]any
	if err := cbor.Unmarshal(zero, &raw); err != nil {
		t.Fatal(err)
	}
	if tag, ok := raw["at"].(cbor.Tag); !ok || tag.Number != models.TagNone {
		t.Fatalf("zero time should stay tag 6, got %#v", raw["at"])
	}

	at := time.Unix(1700000000, 42).UTC()
	w, err := surrealcbor.Marshal(models.CustomDateTime{Time: at})
	if err != nil {
		t.Fatal(err)
	}
	bare, err := surrealcbor.Marshal(at)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w, bare) {
		t.Fatalf("CustomDateTime %x != bare time %x", w, bare)
	}
}
