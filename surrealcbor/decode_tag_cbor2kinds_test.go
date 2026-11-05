package surrealcbor

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// Pairs of vectors from surrealdb-protocol v0.15.0: a value of each 4.0 kind
// as cbor.2 carries it (spec/vectors/cbor2.yaml), and as plain cbor carries it
// (spec/vectors/cbor.yaml). The date infinity is widened as the server v4.0.0
// widens it, to its latest datetime.
var kindPairs = []struct {
	name, cbor2, plain string
}{
	{"date", "d864194d46", "cc821a65dfc90000"},
	{"date before 1970", "d86420", "cc823a0001517f00"},
	{"date infinity", "d8641a05a9fc3c", "cc821b000007779a0a6b7f1a3b9ac9ff"},
	{"date -infinity", "d8643a05bff18b", "cc823b000007948cf211ff00"},
	{"time", "d8bd1b000029327b048f40", "ce8219b0f01a2f072f40"},
	{"time at the end of the day", "d8bd1b00004e94914f0000", "ce811a00015180"},
	{"timestamp", "d8be821a6592a94001", "cc821a6592a94001"},
	{"timestamp infinity", "d8be821b000007779a0a6b7f1a3b9ac9ff", "cc821b000007779a0a6b7f1a3b9ac9ff"},
	{"timestamp -infinity", "d8be823b000007948cf211ff00", "cc823b000007948cf211ff00"},
	{"interval", "d8bf830e031b000000036c9361a0", "70503159324d3344543448354d362e3553"},
	{"negative interval", "d8bf8300003a0016e35f", "6750542d312e3553"},
	{
		"bigdecimal past a decimal",
		"d8c0781d3739323238313632353134323634333337353933353433393530333336",
		"781d3739323238313632353134323634333337353933353433393530333336",
	},
	{"bigdecimal that fits a decimal", "d8c063312e35", "ca63312e35"},
	{"bigdecimal with trailing zeros", "d8c064312e3530", "ca63312e35"},
}

// A field decodes a 4.0 kind over cbor.2 exactly as it decodes the form plain
// cbor carries that value in: the same value, or an error in both. The one
// exception is a models.BigDecimal, which keeps the scale cbor.2 carries,
// such as the 0 of 1.50, where plain cbor sends the decimal 1.5.
func TestDecode_cbor2KindsLikePlainCBOR(t *testing.T) {
	destinations := []func() any{
		func() any { return new(time.Time) },
		func() any { return new(*time.Time) },
		func() any { return new(models.CustomDateTime) },
		func() any { return new(time.Duration) },
		func() any { return new(models.CustomDuration) },
		func() any { return new(models.CustomDurationString) },
		func() any { return new(string) },
		func() any { return new(models.DecimalString) },
		func() any { return new(float64) },
		func() any { return new(int64) },
		func() any { return new(bool) },
		func() any { return new(models.PlainDate) },
		func() any { return new(models.PlainTime) },
		func() any { return new(models.Timestamp) },
		func() any { return new(models.Interval) },
		func() any { return new(models.BigDecimal) },
		func() any { return new(models.Regex) },
	}
	for _, pair := range kindPairs {
		for _, newDestination := range destinations {
			viaCBOR2, viaPlain := newDestination(), newDestination()
			name := pair.name + "/" + reflect.TypeOf(viaCBOR2).Elem().String()
			t.Run(name, func(t *testing.T) {
				errCBOR2 := NewV2().Unmarshal(mustDecodeHex(t, pair.cbor2), viaCBOR2)
				errPlain := New().Unmarshal(mustDecodeHex(t, pair.plain), viaPlain)
				require.Equal(t, errPlain == nil, errCBOR2 == nil, "cbor.2: %v; plain: %v", errCBOR2, errPlain)
				switch b := viaCBOR2.(type) {
				case *models.BigDecimal:
					assert.True(t, b.Equal(*viaPlain.(*models.BigDecimal)), "%v equals %v", b, viaPlain)
				default:
					if errPlain == nil {
						assert.Equal(t, viaPlain, viaCBOR2)
					}
				}
			})
		}
	}
}

func TestDecode_cbor2KindsIntoInterface(t *testing.T) {
	var got []any
	// cbor2-kinds-in-array: [date"2024-02-29", time"01:02:03", interval"P1D"]
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "83d864194d46d8bd1b00000362d417ae00d8bf83000100"), &got))
	assert.Equal(t, []any{
		models.MustParsePlainDate("2024-02-29"),
		models.MustParsePlainTime("01:02:03"),
		models.Interval{Days: 1},
	}, got)

	for in, want := range map[string]any{
		"d8be821a6592a94001": models.MustParseTimestamp("2024-01-01T12:00:00.000000001"),
		"d8c06431653430":     models.MustParseBigDecimal("1e40"),
		"d8c063312e35":       models.MustParseBigDecimal("1.5"),
		"d8c1655e5c642b24":   models.Regex{Pattern: `^\d+$`},
		"d83881d8bf83010000": models.Set{models.Interval{Months: 1}},
		"ca63312e35":         models.DecimalString("1.5"), // a decimal stays a decimal
	} {
		var v any
		require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, in), &v), in)
		assert.Equal(t, want, v, in)
	}

	var m map[string]any
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "a16161d86400"), &m))
	assert.Equal(t, map[string]any{"a": models.PlainDate{}}, m)

	var stringer interface{ String() string }
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "d86400"), &stringer))
	assert.Equal(t, "1970-01-01", stringer.String())
	var e error
	assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, "d86400"), &e), "a PlainDate is not an error")
}

func TestDecode_cbor2KindsRefusedInPlainCBOR(t *testing.T) {
	for _, in := range []string{"d86400", "d8bd00", "d8be820000", "d8bf83000000", "d8c06431653430", "d8c163612e2a"} {
		var v any
		assert.ErrorContains(t, New().Unmarshal(mustDecodeHex(t, in), &v), "only valid in cbor.2", in)
		var at time.Time
		assert.ErrorContains(t, New().Unmarshal(mustDecodeHex(t, in), &at), "only valid in cbor.2", in)
	}
	// Nested, too.
	var v any
	assert.ErrorContains(t, New().Unmarshal(mustDecodeHex(t, "81d86400"), &v), "CBOR tag 100 (date)")

	// A type with its own UnmarshalCBOR gets the item as it is, as any other
	// Unmarshaler does.
	var d models.PlainDate
	require.NoError(t, New().Unmarshal(mustDecodeHex(t, "d864194d46"), &d))
	assert.Equal(t, models.MustParsePlainDate("2024-02-29"), d)
}

func TestDecode_cbor2KindsRegex(t *testing.T) {
	data := mustDecodeHex(t, "d8c163612e2a")
	var s string
	require.NoError(t, NewV2().Unmarshal(data, &s))
	assert.Equal(t, "a.*", s)
	var r models.Regex
	require.NoError(t, NewV2().Unmarshal(data, &r))
	assert.Equal(t, models.Regex{Pattern: "a.*"}, r)

	// The SDK never compiles a pattern: Rust's syntax is not Go's.
	var compiled *regexp.Regexp
	assert.ErrorContains(t, NewV2().Unmarshal(data, &compiled), "cannot decode regex")
	var n int
	assert.Error(t, NewV2().Unmarshal(data, &n))
}

func TestDecode_cbor2KindsRejectVectors(t *testing.T) {
	for _, in := range []string{
		"d8641a05a9fc3d", "d8641a80000000", "d864f93c00", "d8bd1b00004e94914f0001", "d8bd20",
		"d8be821b000007779a0a6b8000", "d8be8100", "d8be822020", "d8bf831a800000000000", "d8bf820000",
		"d8bf830000f93c00", "d8c0634e614e", "d8c063312c35", "d8c06420312e35", "d8c001", "d8c101",
	} {
		var v any
		assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, in), &v), in)
		var at time.Time
		assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, in), &at), in)
	}
}

func TestDecode_cbor2KindsRecordIDKeys(t *testing.T) {
	// reject-date-record-id-key: 8(["t", 100(1)])
	var v any
	assert.ErrorContains(t, NewV2().Unmarshal(mustDecodeHex(t, "c8826174d86401"), &v), "not a valid record id key")
	var id models.RecordID
	assert.ErrorContains(t, NewV2().Unmarshal(mustDecodeHex(t, "c8826174d86401"), &id), "not a valid record id key")

	// Inside an array key a kind is fine.
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "c882617482d86401d8bf83000100"), &v))
	assert.Equal(t, models.RecordID{
		Table: "t",
		ID:    []any{models.MustParsePlainDate("1970-01-02"), models.Interval{Days: 1}},
	}, v)
}

func TestDecode_cbor2KindsNoneAndNull(t *testing.T) {
	for _, in := range []string{"f6", "c6f6"} {
		date := new(models.PlainDate)
		require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, in), &date), in)
		assert.Nil(t, date, "NONE and NULL leave a pointer nil: %s", in)
	}

	// Into a value field, NONE is the zero value, 1970-01-01: use a pointer
	// to tell them apart.
	var row struct {
		Due models.PlainDate `cbor:"due"`
	}
	row.Due = models.MustParsePlainDate("2024-02-29")
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "a163647565c6f6"), &row))
	assert.Equal(t, models.PlainDate{}, row.Due)
}

// A text head that claims more bytes than the input holds is an error, not
// a panic, even with a length near math.MaxInt64.
func TestDecode_cbor2KindsHugeLength(t *testing.T) {
	for _, in := range []string{"d8c07b7fffffffffffffff", "d8c17b7fffffffffffffff", "d8647b7fffffffffffffff"} {
		var v any
		assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, in), &v), in)
		var s string
		assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, in), &s), in)
		var at time.Time
		assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, in), &at), in)
	}
	var raw cbor.RawMessage
	assert.Error(t, NewV2().Unmarshal(mustDecodeHex(t, "7b7fffffffffffffff"), &raw))

	// A pattern past the limit on text is refused too.
	defer func(old uint64) { MaxCBORStringLength = old }(MaxCBORStringLength)
	MaxCBORStringLength = 4
	var r models.Regex
	require.NoError(t, NewV2().Unmarshal(mustDecodeHex(t, "d8c16461626364"), &r), "four bytes are fine")
	var v any
	long, err := models.Regex{Pattern: "abcdefghijklmnopqrst"}.MarshalCBOR()
	require.NoError(t, err)
	assert.ErrorContains(t, NewV2().Unmarshal(long, &v), "exceeds maximum allowed")
}
