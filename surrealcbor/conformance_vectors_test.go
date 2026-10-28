package surrealcbor

// Shared CBOR conformance vectors from surrealdb/surrealdb-protocol.
//
// The vectors are not vendored into this repository. Check the protocol
// repository out at the pinned tag and point SURREALDB_PROTOCOL_DIR at it:
//
//	eval "$(scripts/fetch-surrealdb-protocol.sh)"
//	go test ./surrealcbor -run TestConformanceVectors
//
// Without SURREALDB_PROTOCOL_DIR the tests skip. CI checks out the commit
// the tag points at.
//
// Each vector runs through the plain cbor codec (New) and the cbor.2 codec
// (NewV2), as the protocol repository's spec/README.md describes: canonical
// and decode vectors must decode to their value, canonical values must encode
// to something that decodes back to them, and reject vectors must fail to
// decode. Byte-exact encoding is recommended there, not required, and is not
// checked here.

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"gopkg.in/yaml.v3"
)

// EnvProtocolDir names a checkout of github.com/surrealdb/surrealdb-protocol.
const envProtocolDir = "SURREALDB_PROTOCOL_DIR"

type vectorFile struct {
	Version int      `yaml:"version"`
	Vectors []vector `yaml:"vectors"`
}

type vector struct {
	Name      string   `yaml:"name"`
	Kind      string   `yaml:"kind"`
	Formats   []string `yaml:"formats"`
	Value     any      `yaml:"value"`
	CBOR      string   `yaml:"cbor"`
	Canonical string   `yaml:"canonical"`
}

func (v *vector) holdsFor(format string) bool {
	if len(v.Formats) == 0 {
		return true
	}
	for _, f := range v.Formats {
		if f == format {
			return true
		}
	}
	return false
}

func loadVectors(t *testing.T, file string) []vector {
	t.Helper()
	dir := os.Getenv(envProtocolDir)
	if dir == "" {
		t.Skipf("%s is not set; run scripts/fetch-surrealdb-protocol.sh to check out the conformance vectors", envProtocolDir)
	}
	path := filepath.Join(dir, "spec", "vectors", file)
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the test environment
	if err != nil {
		t.Fatalf("%s is set but the vectors cannot be read: %v", envProtocolDir, err)
	}
	var vf vectorFile
	if err := yaml.Unmarshal(data, &vf); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if vf.Version != 1 {
		t.Fatalf("%s: unsupported vector file version %d", path, vf.Version)
	}
	if len(vf.Vectors) == 0 {
		t.Fatalf("%s: no vectors", path)
	}
	return vf.Vectors
}

// knownVectorGaps lists the vectors this codec does not pass yet, with why.
// They are all about plain cbor, and so hold for cbor.2 as well: the codec
// predates the vectors and decodes more leniently than SurrealDB 4.0 does.
// A listed vector that starts passing fails the test, so the list stays true.
// Keys are "<format>/<vector name>", or "*/<vector name>" for both formats.
var knownVectorGaps = map[string]string{
	// Decoding into an interface turns NONE into nil, the same as NULL (by design, see issue 291).
	"*/none":                    "decoding into an interface turns NONE into nil, the same as NULL (by design, see issue 291)",
	"*/none-and-null-in-array":  "decoding into an interface turns NONE into nil, the same as NULL (by design, see issue 291)",
	"*/none-and-null-in-object": "decoding into an interface turns NONE into nil, the same as NULL (by design, see issue 291)",
	// Decimals are kept as the string sent, not normalised.
	"*/decimal-trailing-zeros-normalised": "decimals are kept as the string sent, not normalised",
	// Models.CustomDateTime treats 0001-01-01T00:00:00Z, Go's zero time, as NONE.
	"*/datetime-year-one": "models.CustomDateTime treats 0001-01-01T00:00:00Z, Go's zero time, as NONE",
	// Durations are time.Duration, whose int64 nanoseconds cannot hold u64 seconds.
	"*/duration-u64-seconds": "durations are time.Duration, whose int64 nanoseconds cannot hold u64 seconds",
	// Sets are not sorted or deduplicated on decode.
	"*/set-unsorted-input": "sets are not sorted or deduplicated on decode",
	// Ranges decode as a plain array of their bounds' values, losing whether each bound is included.
	"*/record-id-range-key":                 "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/record-id-range-key-unbounded-start": "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-start-included-end-excluded":   "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-both-included":                 "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-both-excluded":                 "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-start-excluded-end-included":   "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-unbounded-start":               "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-unbounded-end":                 "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-unbounded":                     "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	"*/range-strings":                       "ranges decode as a plain array of their bounds' values, losing whether each bound is included",
	// The string form of a record id (tag 8 around text) is not parsed.
	"*/record-id-string-form":                 "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-int-key":         "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-array-key":       "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-object-key":      "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-uuid-key":        "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-escapes":         "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-escaped-table":   "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-backtick-table":  "the string form of a record id (tag 8 around text) is not parsed",
	"*/record-id-string-form-digits-past-i64": "the string form of a record id (tag 8 around text) is not parsed",
	// Open polygon rings are not closed on decode.
	"*/geometry-polygon-unclosed-ring": "open polygon rings are not closed on decode",
	// Geometry points with integer coordinates are not decoded.
	"*/geometry-point-integer-coordinates": "geometry points with integer coordinates are not decoded",
	// The decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses.
	"*/reject-uuid-string-tag-9":                            "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-datetime-string-tag-0":                        "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-duration-string-tag-13":                       "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-record-id-uuid-string-key":                    "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-uuid-bad-string":                              "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-duration-string-beyond-u64":                   "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-duration-string-nanoseconds-carry-beyond-u64": "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	"*/reject-duration-bad-string":                          "the decoder accepts the text-form tags 0, 9 and 13 and their invalid contents, which SurrealDB 4.0 refuses",
	// The decoder does not validate the contents of tags 8, 12, 14, 49 and 90.
	"*/reject-duration-negative-seconds":     "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-duration-negative-nanoseconds": "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-datetime-negative-nanoseconds": "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-record-id-one-element":         "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-record-id-three-elements":      "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-record-id-bool-key":            "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-record-id-range-bool-bound":    "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-datetime-after-chrono-max":     "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-datetime-before-chrono-min":    "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-record-id-float-key":           "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-record-id-decimal-key":         "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-range-one-bound":               "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-range-untagged-bound":          "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-polygon-empty":                 "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	"*/reject-none-with-payload":             "the decoder does not validate the contents of tags 8, 12, 14, 49 and 90",
	// Decimals (tag 10) are not validated.
	"*/reject-decimal-not-a-number":            "decimals (tag 10) are not validated",
	"*/reject-decimal-beyond-96-bits":          "decimals (tag 10) are not validated",
	"*/reject-decimal-surrounding-whitespace":  "decimals (tag 10) are not validated",
	"*/reject-decimal-trailing-whitespace":     "decimals (tag 10) are not validated",
	"*/reject-decimal-mantissa-beyond-96-bits": "decimals (tag 10) are not validated",
	"*/reject-decimal-scale-beyond-28":         "decimals (tag 10) are not validated",
	// Unknown and reserved tags are decoded as their content.
	"*/reject-tag-future":             "unknown and reserved tags are decoded as their content",
	"*/reject-tag-binary-decimal":     "unknown and reserved tags are decoded as their content",
	"*/reject-shaped-block-malformed": "unknown and reserved tags are decoded as their content",
	"*/reject-unknown-tag":            "unknown and reserved tags are decoded as their content",
	// The decoder accepts text chunks that are not valid UTF-8 on their own.
	"*/reject-indefinite-text-split-code-point": "the decoder accepts text chunks that are not valid UTF-8 on their own",
	// The decoder accepts untagged integers beyond the int64 range.
	"*/reject-int-beyond-i64":            "the decoder accepts untagged integers beyond the int64 range",
	"*/reject-untagged-integer-past-i64": "the decoder accepts untagged integers beyond the int64 range",
	// The decoder keeps the last of duplicate map keys.
	"*/reject-object-duplicate-key": "the decoder keeps the last of duplicate map keys",
}

func knownGap(format, name string) (string, bool) {
	if r, ok := knownVectorGaps[format+"/"+name]; ok {
		return r, true
	}
	r, ok := knownVectorGaps["*/"+name]
	return r, ok
}

func TestConformanceVectors(t *testing.T) {
	cbor1 := loadVectors(t, "cbor.yaml")
	cbor2 := loadVectors(t, "cbor2.yaml")

	t.Run("cbor", func(t *testing.T) {
		for i := range cbor1 {
			if v := &cbor1[i]; v.holdsFor("cbor") {
				runVector(t, New(), "cbor", v)
			}
		}
	})
	t.Run("cbor.2", func(t *testing.T) {
		for i := range cbor1 {
			if v := &cbor1[i]; v.holdsFor("cbor.2") {
				runVector(t, NewV2(), "cbor.2", v)
			}
		}
		for i := range cbor2 {
			runVector(t, NewV2(), "cbor.2", &cbor2[i])
		}
	})
	t.Run("cbor.2 accepts the sized-number tags plain cbor refuses", func(t *testing.T) {
		for i := range cbor1 {
			v := &cbor1[i]
			if v.Kind != "reject" || v.holdsFor("cbor.2") {
				continue
			}
			var got any
			if err := NewV2().Unmarshal(mustHex(t, v.CBOR), &got); err != nil {
				t.Errorf("%s: cbor.2 refuses it: %v", v.Name, err)
			}
		}
	})
}

func runVector(t *testing.T, c *Codec, format string, v *vector) {
	t.Run(v.Name, func(t *testing.T) {
		err := checkVector(c, format, v)
		reason, known := knownGap(format, v.Name)
		switch {
		case err != nil && known:
			t.Skipf("known gap: %s (%v)", reason, err)
		case err != nil:
			t.Error(err)
		case known:
			t.Errorf("passes now: remove %q from knownVectorGaps", v.Name)
		}
	})
}

func checkVector(c *Codec, format string, v *vector) error {
	data, err := hex.DecodeString(v.CBOR)
	if err != nil {
		return fmt.Errorf("bad hex: %w", err)
	}
	switch v.Kind {
	case "reject":
		var got any
		if c.Unmarshal(data, &got) == nil {
			return fmt.Errorf("decoded to %#v, want an error", got)
		}
		return nil
	case "canonical", "decode":
	default:
		return fmt.Errorf("unknown vector kind %q", v.Kind)
	}

	var got any
	if err = c.Unmarshal(data, &got); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if err = matchTyped(v.Value, got, format == "cbor.2"); err != nil {
		return fmt.Errorf("decode: %w (got %#v)", err, got)
	}
	if v.Kind != "canonical" {
		return nil
	}

	value, err := typedToGo(v.Value)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	encoded, err := c.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	var back any
	if err := c.Unmarshal(encoded, &back); err != nil {
		return fmt.Errorf("encode: %x does not decode: %w", encoded, err)
	}
	if err := matchTyped(v.Value, back, format == "cbor.2"); err != nil {
		return fmt.Errorf("encode: %x decodes to %#v: %w", encoded, back, err)
	}
	return nil
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// typedKey returns the single "$" key of a typed JSON value, if it is one.
func typedKey(m map[string]any) (key string, val any, ok bool) {
	if len(m) != 1 {
		return "", nil, false
	}
	for k, v := range m {
		switch k {
		case "$none", "$int", "$float", "$i8", "$u8", "$i16", "$u16", "$i32", "$u32", "$u64", "$f16", "$f32",
			"$decimal", "$string", "$bytes", "$uuid", "$datetime", "$duration", "$array", "$object", "$set",
			"$table", "$record", "$range", "$file", "$geometry", "$regex":
			return k, v, true
		}
	}
	return "", nil, false
}

// typedInt reads an integer written as a YAML number or a decimal string.
func typedInt(v any) (*big.Int, error) {
	switch n := v.(type) {
	case int:
		return big.NewInt(int64(n)), nil
	case string:
		i, ok := new(big.Int).SetString(n, 10)
		if !ok {
			return nil, fmt.Errorf("bad integer %q", n)
		}
		return i, nil
	default:
		return nil, fmt.Errorf("bad integer %#v", v)
	}
}

// typedFloat reads a float written as a YAML number or one of the special strings.
func typedFloat(v any) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case float64:
		return n, nil
	case string:
		switch n {
		case "NaN":
			return math.NaN(), nil
		case "Infinity":
			return math.Inf(1), nil
		case "-Infinity":
			return math.Inf(-1), nil
		case "-0":
			return math.Copysign(0, -1), nil
		}
		return strconv.ParseFloat(n, 64)
	default:
		return 0, fmt.Errorf("bad float %#v", v)
	}
}

func sameFloatBits(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return a == b && math.Signbit(a) == math.Signbit(b)
}

// goInt returns the integer a decoded Go value holds, if it is one of the
// given kinds.
func goInt(got any, kinds ...reflect.Kind) (*big.Int, bool) {
	rv := reflect.ValueOf(got)
	if !rv.IsValid() {
		return nil, false
	}
	for _, k := range kinds {
		if rv.Kind() != k || rv.Type().PkgPath() != "" {
			continue
		}
		switch k {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return big.NewInt(rv.Int()), true
		default:
			return new(big.Int).SetUint64(rv.Uint()), true
		}
	}
	return nil, false
}

var sizedIntKinds = map[string]reflect.Kind{
	"$i8": reflect.Int8, "$u8": reflect.Uint8, "$i16": reflect.Int16, "$u16": reflect.Uint16,
	"$i32": reflect.Int32, "$u32": reflect.Uint32, "$u64": reflect.Uint64,
}

// matchTyped checks that got, a value decoded into an interface, is the Go
// representation of exp, a typed JSON value from a vector file.
//
// Plain integers decode as uint64 (non-negative) or int64 (negative); under
// cbor.2 a u64 is a uint64 too, so the two cannot be told apart after
// decoding. Plain floats decode as float64, or under plain cbor as float32
// when they were sent half- or single-width.
//
//nolint:gocyclo // one case per typed JSON form
func matchTyped(exp, got any, v2 bool) error {
	switch e := exp.(type) {
	case nil:
		if got != nil {
			return fmt.Errorf("want NULL, got %#v", got)
		}
		return nil
	case bool:
		if got != e {
			return fmt.Errorf("want %v, got %#v", e, got)
		}
		return nil
	case string:
		if got != e {
			return fmt.Errorf("want %q, got %#v", e, got)
		}
		return nil
	case int:
		return matchInt(big.NewInt(int64(e)), got)
	case []any:
		return matchArray(e, got, v2)
	case map[string]any:
		key, val, typed := typedKey(e)
		if !typed {
			return matchObject(e, got, v2)
		}
		return matchTypedValue(key, val, got, v2)
	default:
		return fmt.Errorf("unexpected YAML value %#v", exp)
	}
}

func matchInt(want *big.Int, got any) error {
	i, ok := goInt(got, reflect.Uint64, reflect.Int64)
	if !ok || i.Cmp(want) != 0 {
		return fmt.Errorf("want int %s, got %#v", want, got)
	}
	return nil
}

func matchArray(e []any, got any, v2 bool) error {
	g, ok := got.([]any)
	if !ok || len(g) != len(e) {
		return fmt.Errorf("want an array of %d, got %#v", len(e), got)
	}
	for i := range e {
		if err := matchTyped(e[i], g[i], v2); err != nil {
			return fmt.Errorf("[%d]: %w", i, err)
		}
	}
	return nil
}

func matchObject(e map[string]any, got any, v2 bool) error {
	g, ok := got.(map[string]any)
	if !ok || len(g) != len(e) {
		return fmt.Errorf("want an object of %d keys, got %#v", len(e), got)
	}
	for k, ev := range e {
		gv, ok := g[k]
		if !ok {
			return fmt.Errorf("missing key %q", k)
		}
		if err := matchTyped(ev, gv, v2); err != nil {
			return fmt.Errorf(".%s: %w", k, err)
		}
	}
	return nil
}

//nolint:gocyclo // one case per typed JSON form
func matchTypedValue(key string, val, got any, v2 bool) error {
	switch key {
	case "$none":
		if _, ok := got.(models.CustomNil); !ok {
			return fmt.Errorf("want NONE, got %#v", got)
		}
		return nil
	case "$int":
		want, err := typedInt(val)
		if err != nil {
			return err
		}
		return matchInt(want, got)
	case "$i8", "$u8", "$i16", "$u16", "$i32", "$u32", "$u64":
		want, err := typedInt(val)
		if err != nil {
			return err
		}
		i, ok := goInt(got, sizedIntKinds[key])
		if !ok || i.Cmp(want) != 0 {
			return fmt.Errorf("want %s %s, got %#v", key[1:], want, got)
		}
		return nil
	case "$float", "$f32", "$f16":
		want, err := typedFloat(val)
		if err != nil {
			return err
		}
		var f float64
		switch g := got.(type) {
		case float64:
			if key != "$float" {
				return fmt.Errorf("want %s, got float64 %v", key[1:], g)
			}
			f = g
		case float32:
			if key == "$float" && v2 || key == "$f16" {
				return fmt.Errorf("want %s, got float32 %v", key[1:], g)
			}
			f = float64(g)
		case models.Float16:
			if key != "$f16" {
				return fmt.Errorf("want %s, got Float16 %v", key[1:], g)
			}
			f = g.Float64()
		default:
			return fmt.Errorf("want %s %v, got %#v", key[1:], want, got)
		}
		if !sameFloatBits(f, want) {
			return fmt.Errorf("want %s %v, got %v", key[1:], want, f)
		}
		return nil
	case "$decimal":
		if got != models.DecimalString(fmt.Sprint(val)) {
			return fmt.Errorf("want decimal %v, got %#v", val, got)
		}
		return nil
	case "$string":
		return matchTyped(fmt.Sprint(val), got, v2)
	case "$bytes":
		want, err := hex.DecodeString(fmt.Sprint(val))
		if err != nil {
			return err
		}
		g, ok := got.([]byte)
		if !ok || hex.EncodeToString(g) != hex.EncodeToString(want) {
			return fmt.Errorf("want bytes %x, got %#v", want, got)
		}
		return nil
	case "$uuid":
		g, ok := got.(models.UUID)
		if !ok || g.String() != val {
			return fmt.Errorf("want uuid %v, got %#v", val, got)
		}
		return nil
	case "$datetime":
		want, err := time.Parse(time.RFC3339Nano, fmt.Sprint(val))
		if err != nil {
			return err
		}
		var g time.Time
		switch d := got.(type) {
		case models.CustomDateTime:
			g = d.Time
		case time.Time:
			g = d
		default:
			return fmt.Errorf("want datetime %v, got %#v", val, got)
		}
		if !g.Equal(want) {
			return fmt.Errorf("want datetime %v, got %v", want, g)
		}
		return nil
	case "$duration":
		want, err := typedDuration(val)
		if err != nil {
			return err
		}
		g, ok := got.(models.CustomDuration)
		if !ok || g.Duration != want {
			return fmt.Errorf("want duration %v, got %#v", want, got)
		}
		return nil
	case "$array":
		e, _ := val.([]any)
		return matchArray(e, got, v2)
	case "$object":
		e, _ := val.(map[string]any)
		return matchObject(e, got, v2)
	case "$set":
		e, _ := val.([]any)
		g, ok := got.(models.Set)
		if !ok {
			return fmt.Errorf("want a set, got %#v", got)
		}
		return matchArray(e, []any(g), v2)
	case "$table":
		if got != models.Table(fmt.Sprint(val)) {
			return fmt.Errorf("want table %v, got %#v", val, got)
		}
		return nil
	case "$record":
		r, _ := val.(map[string]any)
		g, ok := got.(models.RecordID)
		if !ok || g.Table != r["table"] {
			return fmt.Errorf("want record id on %v, got %#v", r["table"], got)
		}
		if err := matchTyped(r["key"], g.ID, v2); err != nil {
			return fmt.Errorf("record id key: %w", err)
		}
		return nil
	case "$file":
		f, _ := val.(map[string]any)
		if got != (models.File{Bucket: fmt.Sprint(f["bucket"]), Key: fmt.Sprint(f["key"])}) {
			return fmt.Errorf("want file %v, got %#v", f, got)
		}
		return nil
	case "$geometry":
		want, err := typedGeometry(val)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(want, got) {
			return fmt.Errorf("want geometry %#v, got %#v", want, got)
		}
		return nil
	case "$range":
		return fmt.Errorf("ranges decode into an interface as a plain array of their bounds' values, losing which bounds are inclusive (got %#v)", got)
	default:
		return fmt.Errorf("unsupported typed value %s", key)
	}
}

func typedDuration(val any) (time.Duration, error) {
	pair, ok := val.([]any)
	if !ok || len(pair) != 2 {
		return 0, fmt.Errorf("bad duration %#v", val)
	}
	s, err := typedInt(pair[0])
	if err != nil {
		return 0, err
	}
	ns, err := typedInt(pair[1])
	if err != nil {
		return 0, err
	}
	total := new(big.Int).Mul(s, big.NewInt(int64(time.Second)))
	total.Add(total, ns)
	if !total.IsInt64() {
		return 0, fmt.Errorf("duration %s ns does not fit time.Duration", total)
	}
	return time.Duration(total.Int64()), nil
}

func typedPoint(v any) (models.GeometryPoint, error) {
	c, ok := v.([]any)
	if !ok || len(c) != 2 {
		return models.GeometryPoint{}, fmt.Errorf("bad point %#v", v)
	}
	x, err := typedFloat(c[0])
	if err != nil {
		return models.GeometryPoint{}, err
	}
	y, err := typedFloat(c[1])
	if err != nil {
		return models.GeometryPoint{}, err
	}
	return models.GeometryPoint{Longitude: x, Latitude: y}, nil
}

func typedLine(v any) (models.GeometryLine, error) {
	pts, _ := v.([]any)
	line := models.GeometryLine{}
	for _, p := range pts {
		pt, err := typedPoint(p)
		if err != nil {
			return nil, err
		}
		line = append(line, pt)
	}
	return line, nil
}

func typedPolygon(v any) (models.GeometryPolygon, error) {
	rings, _ := v.([]any)
	poly := models.GeometryPolygon{}
	for _, r := range rings {
		line, err := typedLine(r)
		if err != nil {
			return nil, err
		}
		poly = append(poly, line)
	}
	return poly, nil
}

//nolint:gocyclo // one case per geometry type
func typedGeometry(val any) (any, error) {
	g, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("bad geometry %#v", val)
	}
	coords := g["coordinates"]
	switch g["type"] {
	case "Point":
		return typedPoint(coords)
	case "LineString":
		return typedLine(coords)
	case "Polygon":
		return typedPolygon(coords)
	case "MultiPoint":
		line, err := typedLine(coords)
		return models.GeometryMultiPoint(line), err
	case "MultiLineString":
		poly, err := typedPolygon(coords)
		return models.GeometryMultiLine(poly), err
	case "MultiPolygon":
		polys, _ := coords.([]any)
		mp := models.GeometryMultiPolygon{}
		for _, p := range polys {
			poly, err := typedPolygon(p)
			if err != nil {
				return nil, err
			}
			mp = append(mp, poly)
		}
		return mp, nil
	case "GeometryCollection":
		geoms, _ := g["geometries"].([]any)
		gc := models.GeometryCollection{}
		for _, sub := range geoms {
			v, err := typedGeometry(sub)
			if err != nil {
				return nil, err
			}
			gc = append(gc, v)
		}
		return gc, nil
	default:
		return nil, fmt.Errorf("unknown geometry type %v", g["type"])
	}
}

// typedToGo builds the Go value a typed JSON value stands for, to encode it.
//
//nolint:gocyclo // one case per typed JSON form
func typedToGo(exp any) (any, error) {
	switch e := exp.(type) {
	case nil, bool, string:
		return e, nil
	case int:
		return int64(e), nil
	case []any:
		out := make([]any, len(e))
		for i := range e {
			v, err := typedToGo(e[i])
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	case map[string]any:
		key, val, typed := typedKey(e)
		if !typed {
			return objectToGo(e)
		}
		return typedValueToGo(key, val)
	default:
		return nil, fmt.Errorf("unexpected YAML value %#v", exp)
	}
}

func objectToGo(e map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(e))
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, err := typedToGo(e[k])
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

//nolint:gocyclo // one case per typed JSON form
func typedValueToGo(key string, val any) (any, error) {
	switch key {
	case "$none":
		return models.CustomNil{}, nil
	case "$int":
		i, err := typedInt(val)
		if err != nil {
			return nil, err
		}
		if !i.IsInt64() {
			return nil, fmt.Errorf("int %s does not fit int64", i)
		}
		return i.Int64(), nil
	case "$i8", "$u8", "$i16", "$u16", "$i32", "$u32", "$u64":
		i, err := typedInt(val)
		if err != nil {
			return nil, err
		}
		v := reflect.New(reflect.TypeOf(sizedZero[key])).Elem()
		if strings.HasPrefix(key, "$u") {
			v.SetUint(i.Uint64())
		} else {
			v.SetInt(i.Int64())
		}
		return v.Interface(), nil
	case "$float":
		return typedFloat(val)
	case "$f32":
		f, err := typedFloat(val)
		return float32(f), err
	case "$f16":
		f, err := typedFloat(val)
		if err != nil {
			return nil, err
		}
		return models.NewFloat16(f)
	case "$decimal":
		return models.DecimalString(fmt.Sprint(val)), nil
	case "$string":
		return fmt.Sprint(val), nil
	case "$bytes":
		return hex.DecodeString(fmt.Sprint(val))
	case "$uuid":
		u, err := uuid.FromString(fmt.Sprint(val))
		return models.UUID{UUID: u}, err
	case "$datetime":
		t, err := time.Parse(time.RFC3339Nano, fmt.Sprint(val))
		return models.CustomDateTime{Time: t}, err
	case "$duration":
		d, err := typedDuration(val)
		return models.CustomDuration{Duration: d}, err
	case "$array":
		return typedToGo(val)
	case "$object":
		e, _ := val.(map[string]any)
		return objectToGo(e)
	case "$set":
		v, err := typedToGo(val)
		if err != nil {
			return nil, err
		}
		arr, _ := v.([]any)
		return models.Set(arr), nil
	case "$table":
		return models.Table(fmt.Sprint(val)), nil
	case "$record":
		r, _ := val.(map[string]any)
		k, err := typedToGo(r["key"])
		if err != nil {
			return nil, err
		}
		return models.RecordID{Table: fmt.Sprint(r["table"]), ID: k}, nil
	case "$file":
		f, _ := val.(map[string]any)
		return models.File{Bucket: fmt.Sprint(f["bucket"]), Key: fmt.Sprint(f["key"])}, nil
	case "$geometry":
		return typedGeometry(val)
	default:
		return nil, fmt.Errorf("no Go value for %s", key)
	}
}

var sizedZero = map[string]any{
	"$i8": int8(0), "$u8": uint8(0), "$i16": int16(0), "$u16": uint16(0),
	"$i32": int32(0), "$u32": uint32(0), "$u64": uint64(0),
}
