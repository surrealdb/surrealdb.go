package models_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	. "github.com/surrealdb/surrealdb.go/v2/pkg/models"
	"github.com/surrealdb/surrealdb.go/v2/surrealcbor"
)

type convInner struct {
	At time.Time `cbor:"at"`
}

type convBase struct {
	Created time.Time `cbor:"created"`
}

type convOuter struct {
	convBase
	Name    string                  `cbor:"name"`
	At      time.Time               `cbor:"at"`
	Ptr     *time.Time              `cbor:"ptr"`
	NilPtr  *time.Time              `cbor:"nilptr"`
	Wrapped CustomDateTime          `cbor:"wrapped"`
	Inner   convInner               `cbor:"inner"`
	List    []time.Time             `cbor:"list"`
	Arr     [2]time.Time            `cbor:"arr"`
	M       map[string]time.Time    `cbor:"m"`
	Any     any                     `cbor:"any"`
	Skip    time.Time               `cbor:"-"`
	Opt     *convInner              `cbor:"opt,omitempty"`
	Nested  map[string][]*convInner `cbor:"nested"`
	hidden  time.Time
}

// tag12 asserts that raw decodes to a tag 12 holding [seconds, nanos].
func tag12(t *testing.T, raw any, wantSec, wantNs int64) {
	t.Helper()
	tag, ok := raw.(cbor.Tag)
	if !ok || tag.Number != TagCustomDatetime {
		t.Fatalf("want tag 12, got %#v", raw)
	}
	arr, ok := tag.Content.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("want 2-element array, got %#v", tag.Content)
	}
	var sec, ns int64
	switch s := arr[0].(type) {
	case int64:
		sec = s
	case uint64:
		sec = int64(s) //nolint:gosec // the test's seconds fit an int64
	}
	switch n := arr[1].(type) {
	case int64:
		ns = n
	case uint64:
		ns = int64(n) //nolint:gosec // below a second
	}
	if sec != wantSec || ns != wantNs {
		t.Fatalf("got [%d, %d], want [%d, %d]", sec, ns, wantSec, wantNs)
	}
}

func decodeRaw(t *testing.T, b []byte) map[any]any {
	t.Helper()
	dm, err := cbor.DecOptions{DefaultMapType: nil}.DecMode()
	if err != nil {
		t.Fatal(err)
	}
	var out map[any]any
	if err := dm.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPrepareForEncode_BareTimeIsTag12(t *testing.T) {
	at := time.Unix(1700000000, 123456789).UTC()
	pre := time.Date(1960, 6, 15, 12, 0, 0, 500, time.UTC)
	in := convOuter{
		convBase: convBase{Created: at},
		Name:     "x",
		At:       at,
		Ptr:      &pre,
		Wrapped:  CustomDateTime{at},
		Inner:    convInner{At: at},
		List:     []time.Time{at, pre},
		Arr:      [2]time.Time{at, pre},
		M:        map[string]time.Time{"k": at},
		Any:      at,
		Skip:     at,
		Opt:      &convInner{At: at},
		Nested:   map[string][]*convInner{"a": {{At: at}, nil}},
		hidden:   at,
	}

	m := &CborMarshaler{}
	b, err := m.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeRaw(t, b)

	tag12(t, got["created"], at.Unix(), 123456789)
	tag12(t, got["at"], at.Unix(), 123456789)
	tag12(t, got["ptr"], pre.Unix(), 500)
	tag12(t, got["wrapped"], at.Unix(), 123456789)
	tag12(t, got["inner"].(map[any]any)["at"], at.Unix(), 123456789)
	tag12(t, got["list"].([]any)[0], at.Unix(), 123456789)
	tag12(t, got["list"].([]any)[1], pre.Unix(), 500)
	tag12(t, got["arr"].([]any)[1], pre.Unix(), 500)
	tag12(t, got["m"].(map[any]any)["k"], at.Unix(), 123456789)
	tag12(t, got["any"], at.Unix(), 123456789)
	tag12(t, got["opt"].(map[any]any)["at"], at.Unix(), 123456789)
	tag12(t, got["nested"].(map[any]any)["a"].([]any)[0].(map[any]any)["at"], at.Unix(), 123456789)

	if got["nilptr"] != nil {
		t.Fatalf("nil *time.Time should stay null, got %#v", got["nilptr"])
	}
	if _, ok := got["Skip"]; ok {
		t.Fatal("cbor:\"-\" field was encoded")
	}
	if _, ok := got["hidden"]; ok {
		t.Fatal("unexported field was encoded")
	}
}

func TestPrepareForEncode_PreEpochKeepsNegativeSeconds(t *testing.T) {
	pre := time.Date(1969, 12, 31, 23, 59, 58, 250, time.UTC) // -2s + 250ns
	b, err := (&CborMarshaler{}).Marshal(convInner{At: pre})
	if err != nil {
		t.Fatal(err)
	}
	tag12(t, decodeRaw(t, b)["at"], pre.Unix(), 250)
	if pre.Unix() >= 0 {
		t.Fatal("test time is not pre-1970")
	}
}

func TestPrepareForEncode_ZeroTimeEncodesAsNone(t *testing.T) {
	b, err := (&CborMarshaler{}).Marshal(convInner{})
	if err != nil {
		t.Fatal(err)
	}
	tag, ok := decodeRaw(t, b)["at"].(cbor.Tag)
	if !ok || tag.Number != TagNone {
		t.Fatalf("zero time should be tag 6, got %#v", decodeRaw(t, b)["at"])
	}
	// Same as the wrapped type has always done.
	wb, _ := (&CborMarshaler{}).Marshal(struct {
		At CustomDateTime `cbor:"at"`
	}{})
	if !bytes.Equal(wb, b) {
		t.Fatalf("bare zero %x != wrapped zero %x", b, wb)
	}
}

func TestPrepareForEncode_TopLevelAndWrappedUnchanged(t *testing.T) {
	at := time.Unix(1700000000, 5).UTC()
	m := &CborMarshaler{}

	bare, err := m.Marshal(at)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := m.Marshal(CustomDateTime{at})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bare, wrapped) {
		t.Fatalf("bare %x != wrapped %x", bare, wrapped)
	}
	// Tag 12 header (0xcc) then a 2-element array.
	if bare[0] != 0xcc || bare[1] != 0x82 {
		t.Fatalf("unexpected encoding %x", bare)
	}

	// A CustomDateTime must be returned untouched, not rewrapped.
	in := CustomDateTime{at}
	if got := PrepareForEncode(in); got != any(in) {
		t.Fatalf("CustomDateTime was altered: %#v", got)
	}
}

func TestPrepareForEncode_ShortCircuitsWithoutTime(t *testing.T) {
	type plain struct {
		A string
		B []int
		C map[string]any
	}
	v := plain{A: "a", B: []int{1}, C: map[string]any{"x": 1, "y": []any{"z"}}}
	out := PrepareForEncode(v)
	if _, ok := out.(plain); !ok {
		t.Fatalf("value without time.Time was rebuilt as %T", out)
	}
	if PrepareForEncode(nil) != nil {
		t.Fatal("nil must pass through")
	}
	var np *convInner
	if PrepareForEncode(np) != any(np) {
		t.Fatal("typed nil pointer must pass through")
	}
}

func TestPrepareForEncode_DecodesBackIntoTimeAndCustomDateTime(t *testing.T) {
	at := time.Unix(1700000000, 123456789).UTC()
	b, err := (&CborMarshaler{}).Marshal(convInner{At: at})
	if err != nil {
		t.Fatal(err)
	}

	var wrapped struct {
		At CustomDateTime `cbor:"at"`
	}
	if err := (&CborUnmarshaler{}).Unmarshal(b, &wrapped); err != nil {
		t.Fatal(err)
	}
	if !wrapped.At.Equal(at) {
		t.Fatalf("got %v, want %v", wrapped.At.Time, at)
	}
}

type convNode struct {
	At   time.Time `cbor:"at"`
	Next *convNode `cbor:"next"`
}

func TestPrepareForEncode_RecursiveTypeDoesNotPanic(t *testing.T) {
	at := time.Unix(1700000000, 0).UTC()
	b, err := (&CborMarshaler{}).Marshal(convNode{At: at, Next: &convNode{At: at}})
	if err != nil {
		t.Fatal(err)
	}
	// The top-level time is converted; the recursive edge is documented as left alone.
	tag12(t, decodeRaw(t, b)["at"], at.Unix(), 0)
}

type durHolder struct {
	Every   time.Duration            `cbor:"every"`
	Ptr     *time.Duration           `cbor:"ptr"`
	List    []time.Duration          `cbor:"list"`
	M       map[string]time.Duration `cbor:"m"`
	Any     any                      `cbor:"any"`
	Zero    time.Duration            `cbor:"zero"`
	Omitted time.Duration            `cbor:"omitted,omitempty"`
	JSONOpt time.Duration            `json:"jsonopt,omitempty"`
	Kept    time.Duration            `cbor:"kept,omitempty"`
}

// tag14 asserts that raw decodes to a tag 14 holding [seconds, nanos].
func tag14(t *testing.T, raw any, wantSec, wantNs uint64) {
	t.Helper()
	tag, ok := raw.(cbor.Tag)
	if !ok || tag.Number != TagCustomDuration {
		t.Fatalf("want tag 14, got %#v", raw)
	}
	arr, ok := tag.Content.([]any)
	if !ok || len(arr) != 2 || arr[0] != wantSec || arr[1] != wantNs {
		t.Fatalf("got %#v, want [%d, %d]", tag.Content, wantSec, wantNs)
	}
}

// A time.Duration is sent as a duration, which SurrealDB reads as one, rather
// than as an integer number of nanoseconds, which it stored as an int.
func TestPrepareForEncode_DurationIsTag14(t *testing.T) {
	d := 90*time.Second + 5
	for name, m := range map[string]interface{ Marshal(any) ([]byte, error) }{
		"surrealcbor": surrealcbor.New(),
		"fxamacker":   &CborMarshaler{},
	} {
		t.Run(name, func(t *testing.T) {
			b, err := m.Marshal(durHolder{
				Every: d, Ptr: &d, List: []time.Duration{d}, M: map[string]time.Duration{"k": d}, Any: d, Kept: d,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := decodeRaw(t, b)
			tag14(t, got["every"], 90, 5)
			tag14(t, got["ptr"], 90, 5)
			tag14(t, got["list"].([]any)[0], 90, 5)
			tag14(t, got["m"].(map[any]any)["k"], 90, 5)
			tag14(t, got["any"], 90, 5)
			tag14(t, got["zero"], 0, 0)
			tag14(t, got["kept"], 90, 5)
			for _, key := range []string{"omitted", "jsonopt"} {
				if _, ok := got[key]; ok {
					t.Fatalf("a zero duration tagged omitempty was encoded as %q", key)
				}
			}

			b, err = m.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			var top any
			if err := cbor.Unmarshal(b, &top); err != nil {
				t.Fatal(err)
			}
			tag14(t, top, 90, 5)
		})
	}

	var back durHolder
	b, err := surrealcbor.New().Marshal(durHolder{Every: d})
	if err != nil {
		t.Fatal(err)
	}
	if err := surrealcbor.New().Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Every != d {
		t.Fatalf("decoded %v, want %v", back.Every, d)
	}
}

// A *regexp.Regexp used to be sent as an empty object. It is refused, as Go's
// regular expression syntax is not SurrealQL's.
func TestPrepareForEncode_RegexpIsRefused(t *testing.T) {
	re := regexp.MustCompile("a.b")
	var none *regexp.Regexp
	for name, m := range map[string]interface{ Marshal(any) ([]byte, error) }{
		"surrealcbor": surrealcbor.New(),
		"fxamacker":   &CborMarshaler{},
	} {
		t.Run(name, func(t *testing.T) {
			for _, v := range []any{re, *re, map[string]any{"re": re}, []any{1, re}, struct{ Re *regexp.Regexp }{re}} {
				if _, err := m.Marshal(v); err == nil || !strings.Contains(err.Error(), "models.Regex") {
					t.Fatalf("%T: want an error naming models.Regex, got %v", v, err)
				}
			}
			b, err := m.Marshal(map[string]any{"re": none})
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeRaw(t, b)["re"]; got != nil {
				t.Fatalf("a nil *regexp.Regexp should stay null, got %#v", got)
			}
		})
	}
}
