package models

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zones from UTC-12 to UTC+14, so a date that silently used time.Local or
// UTC would show.
var zones = []*time.Location{
	time.FixedZone("UTC-12", -12*3600),
	time.FixedZone("UTC-8", -8*3600),
	time.UTC,
	time.FixedZone("UTC+5:30", 5*3600+1800),
	time.FixedZone("UTC+14", 14*3600),
}

func TestIsCBOR2OnlyTag(t *testing.T) {
	for _, tag := range []uint64{100, 180, 184, 188, 189, 190, 191, 192, 193} {
		assert.True(t, IsCBOR2OnlyTag(tag), tag)
	}
	for _, tag := range []uint64{0, 10, 12, 14, 99, 101, 179, 194} {
		assert.False(t, IsCBOR2OnlyTag(tag), tag)
	}
}

func TestPlainDate(t *testing.T) {
	for text, days := range map[string]int64{
		"2024-02-29":   19782,
		"1970-01-01":   0,
		"1969-12-31":   -1,
		"0000-02-29":   -719469,
		"-0001-12-31":  -719529,
		"+10000-01-01": 2932897,
	} {
		d, err := ParsePlainDate(text)
		require.NoError(t, err, text)
		assert.Equal(t, days, d.Days(), text)
		assert.Equal(t, text, d.String())
		back, err := PlainDateFromDays(days)
		require.NoError(t, err)
		assert.Equal(t, d, back)
	}

	d := MustParsePlainDate("2024-02-29")
	year, month, day := d.Date()
	assert.Equal(t, []int{2024, 2, 29}, []int{year, int(month), day})
	assert.Equal(t, "pd'2024-02-29'", d.SurrealString())
	assert.Equal(t, time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), d.In(time.UTC))
	assert.Equal(t, d, MustParsePlainDate(" 2024-02-29 "), "spaces around the text are ignored")
	assert.Equal(t, PlainDate{}, MustParsePlainDate("EPOCH"))

	built, err := NewPlainDate(2024, time.February, 29)
	require.NoError(t, err)
	assert.Equal(t, d, built)
	for _, loc := range zones {
		at := time.Date(2024, 2, 29, 23, 59, 59, 999999999, loc)
		of, err := PlainDateOf(at)
		require.NoError(t, err, loc)
		assert.Equal(t, d, of, "the date in the time's own location, %v", loc)
		assert.Equal(t, d, plainDateOf(t, d.In(loc)), loc)
	}

	for _, bad := range []string{
		"2023-02-29", "2024-13-01", "2024-00-10", "2024-01-00", "2024-01-01T00:00", "2024-001-01", "2024/01/01",
		"1234567-01-01", "10000-01-01", "100000-01-01", "", "infinite", "+262142-12-31", "-262143-01-01", "+262143-01-01",
	} {
		_, err := ParsePlainDate(bad)
		assert.Error(t, err, bad)
	}

	// The server reads a date with chrono, which also takes a short year
	// without a sign, one-digit months and days, and spaces around each
	// number. A year past 9999 needs a sign.
	for text, want := range map[string]string{
		"24-01-01":       "0024-01-01",
		"2024-1-5":       "2024-01-05",
		"+1-01-01":       "0001-01-01",
		"2024 - 01 - 15": "2024-01-15",
		"+0002024-01-15": "2024-01-15",
		"-123456-1-2":    "-123456-01-02",
	} {
		d, err := ParsePlainDate(text)
		require.NoError(t, err, text)
		assert.Equal(t, want, d.String(), text)
	}
	for _, bad := range [][3]int{{2023, 2, 29}, {2024, 2, 30}, {2024, 13, 1}, {262142, 12, 31}, {-262143, 1, 1}, {300000, 1, 1}} {
		_, err := NewPlainDate(bad[0], time.Month(bad[1]), bad[2])
		assert.Error(t, err, bad)
	}
	assert.Panics(t, func() { MustParsePlainDate("2023-02-29") })
}

func plainDateOf(t *testing.T, at time.Time) PlainDate {
	t.Helper()
	d, err := PlainDateOf(at)
	require.NoError(t, err)
	return d
}

func TestPlainDate_infinities(t *testing.T) {
	assert.Equal(t, "infinity", PlainDateInfinity.String())
	assert.Equal(t, "-infinity", PlainDateNegInfinity.String())
	assert.Equal(t, "pd'infinity'", PlainDateInfinity.SurrealString())
	for text, want := range map[string]PlainDate{
		"infinity": PlainDateInfinity, "+Infinity": PlainDateInfinity, "INFINITY": PlainDateInfinity,
		"-infinity": PlainDateNegInfinity, "-InFiNiTy": PlainDateNegInfinity,
	} {
		assert.Equal(t, want, MustParsePlainDate(text), text)
	}
	assert.Equal(t, PlainDateInfinity, MustParsePlainDate("\t infinity\n"), "spaces around it are ignored")
	assert.True(t, PlainDateInfinity.IsInfinite())
	assert.True(t, PlainDateNegInfinity.IsInfinite())
	assert.False(t, PlainDate{}.IsInfinite())
	assert.Equal(t, int64(95026236), PlainDateInfinity.Days())
	assert.Equal(t, int64(-96465292), PlainDateNegInfinity.Days())
	_, err := PlainDateFromDays(95026237)
	assert.Error(t, err)
	_, err = PlainDateFromDays(-96465293)
	assert.Error(t, err)

	for _, inf := range []PlainDate{PlainDateInfinity, PlainDateNegInfinity} {
		assert.Equal(t, inf, plainDateOf(t, inf.In(time.UTC)), "a time on an end day is that infinity")
	}
	assert.Equal(t, -1, PlainDateNegInfinity.Compare(PlainDate{}))
	assert.Equal(t, 1, PlainDateInfinity.Compare(MustParsePlainDate("+10000-01-01")))
	assert.Equal(t, 0, PlainDate{}.Compare(MustParsePlainDate("1970-01-01")))
}

func TestPlainTime(t *testing.T) {
	for text, want := range map[string]string{
		"9:05":               "09:05:00",
		"12:34:56.5":         "12:34:56.500",
		"12:34:56.123456":    "12:34:56.123456",
		"12:34:56.123456789": "12:34:56.123456789",
		"12:34:56.120":       "12:34:56.120",
		"1:2:3":              "01:02:03",
		"24:00:00":           "24:00:00",
		"24:00":              "24:00:00",
		"0:00":               "00:00:00",
	} {
		pt, err := ParsePlainTime(text)
		require.NoError(t, err, text)
		assert.Equal(t, want, pt.String(), text)
		assert.Equal(t, pt, MustParsePlainTime(want))
	}
	assert.Equal(t, 45296*time.Second+1, MustParsePlainTime("12:34:56.000000001").SinceMidnight())
	assert.Equal(t, "23:59:59", MustParsePlainTime("\t23:59:59 ").String(), "spaces around it are ignored")

	for _, bad := range []string{
		"24:00:00.1", "24:00:01", "24:01", "12:60", "12:00:60", "12", "123:00", "12:00:00.", "12:00:00.1234567890",
		"12:00:00Z", "-1:00", "", "12:000",
	} {
		_, err := ParsePlainTime(bad)
		assert.Error(t, err, bad)
	}

	end := MustParsePlainTime("24:00:00")
	hour, minute, sec := end.Clock()
	assert.Equal(t, []int{24, 0, 0}, []int{hour, minute, sec}, "24:00:00 is hour 24, not a day's rollover")
	assert.Equal(t, 24*time.Hour, end.SinceMidnight())

	pt, err := NewPlainTime(12, 34, 56, 789000000)
	require.NoError(t, err)
	assert.Equal(t, "12:34:56.789", pt.String())
	assert.Equal(t, 789000000, pt.Nanosecond())
	assert.Equal(t, "pt'12:34:56.789'", pt.SurrealString())
	_, err = NewPlainTime(24, 0, 0, 0)
	require.NoError(t, err)
	for _, bad := range [][4]int{{24, 0, 0, 1}, {24, 1, 0, 0}, {25, 0, 0, 0}, {12, 60, 0, 0}, {12, 0, 60, 0}, {12, 0, 0, 1e9}, {-1, 0, 0, 0}} {
		_, badErr := NewPlainTime(bad[0], bad[1], bad[2], bad[3])
		assert.Error(t, badErr, bad)
	}

	fromDuration, err := PlainTimeFromDuration(12*time.Hour + 34*time.Minute + 56*time.Second + 789*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, pt, fromDuration)
	_, err = PlainTimeFromDuration(24*time.Hour + 1)
	assert.Error(t, err)
	_, err = PlainTimeFromDuration(-1)
	assert.Error(t, err)

	for _, loc := range zones {
		assert.Equal(t, pt, PlainTimeOf(time.Date(2024, 1, 1, 12, 34, 56, 789000000, loc)), loc)
	}
	assert.Equal(t, -1, PlainTime{}.Compare(end))
}

func TestTimestamp(t *testing.T) {
	for text, want := range map[string]struct {
		sec  int64
		nsec int
		back string
	}{
		"2024-01-15T12:34:56.789":       {1705322096, 789000000, "2024-01-15T12:34:56.789"},
		"2024-01-15 00:00":              {1705276800, 0, "2024-01-15T00:00:00"},
		"1969-12-31T23:59:59.5":         {-1, 500000000, "1969-12-31T23:59:59.500"},
		"2024-01-01T12:00:00.000000001": {1704110400, 1, "2024-01-01T12:00:00.000000001"},
		"epoch":                         {0, 0, "1970-01-01T00:00:00"},
		"+262142-12-31T00:00:00":        {8210266790400, 0, "+262142-12-31T00:00:00"},
		"-0001-12-31T23:59:59":          {-62167219201, 0, "-0001-12-31T23:59:59"},
	} {
		ts, err := ParseTimestamp(text)
		require.NoError(t, err, text)
		assert.Equal(t, want.sec, ts.Unix(), text)
		assert.Equal(t, want.nsec, ts.Nanosecond(), text)
		assert.Equal(t, want.back, ts.String(), text)
	}

	for _, bad := range []string{
		"2024-01-15T24:00:00", "2024-01-15", "2024-01-15t12:00", "2024-02-30T00:00", "+262142-12-31T23:59:59.999999999",
		"-262143-01-01T00:00:00", "", "10000-01-01T00:00:00",
	} {
		_, err := ParseTimestamp(bad)
		assert.Error(t, err, bad)
	}
	// The server trims the time after the T or the space.
	for _, text := range []string{"2024-01-15T 12:00", "2024-01-15  12:00:00", "2024-1-15 12:00"} {
		ts, err := ParseTimestamp(text)
		require.NoError(t, err, text)
		assert.Equal(t, "2024-01-15T12:00:00", ts.String(), text)
	}

	assert.Equal(t, "infinity", TimestampInfinity.String())
	assert.Equal(t, "-infinity", TimestampNegInfinity.String())
	assert.Equal(t, TimestampInfinity, MustParseTimestamp("+Infinity"))
	assert.Equal(t, TimestampNegInfinity, MustParseTimestamp("-infinity"))
	assert.True(t, MustParseTimestamp("infinity").IsInfinite())
	assert.Equal(t, "ts'2024-01-15T12:34:56'", MustParseTimestamp("2024-01-15T12:34:56").SurrealString())

	ts, err := TimestampFromUnix(-1, 0)
	require.NoError(t, err)
	assert.Equal(t, "1969-12-31T23:59:59", ts.String())
	carried, err := TimestampFromUnix(1, -1)
	require.NoError(t, err)
	assert.Equal(t, MustParseTimestamp("1970-01-01T00:00:00.999999999"), carried)
	carried, err = TimestampFromUnix(0, 2_500_000_000)
	require.NoError(t, err)
	assert.Equal(t, MustParseTimestamp("1970-01-01T00:00:02.5"), carried)
	for _, bad := range [][2]int64{{8210266876800, 0}, {-8334601228801, 0}, {math.MaxInt64, 0}, {math.MinInt64, -1}, {8210266876799, 1e9}} {
		_, badErr := TimestampFromUnix(bad[0], bad[1])
		assert.Error(t, badErr, bad)
	}

	// TimestampOf reads the wall clock in the time's own location, and In
	// gives back the instant at which a clock there reads it.
	want := MustParseTimestamp("2024-01-15T12:34:56.789")
	for _, loc := range zones {
		at := time.Date(2024, 1, 15, 12, 34, 56, 789000000, loc)
		got, ofErr := TimestampOf(at)
		require.NoError(t, ofErr, loc)
		assert.Equal(t, want, got, loc)
		assert.True(t, at.Equal(want.In(loc)), loc)
	}
	_, err = TimestampOf(time.Date(300000, 1, 1, 0, 0, 0, 0, time.UTC))
	assert.Error(t, err)

	assert.Equal(t, -1, MustParseTimestamp("1970-01-01T00:00:00.1").Compare(MustParseTimestamp("1970-01-01T00:00:00.2")))
	assert.Equal(t, 1, TimestampInfinity.Compare(Timestamp{}))
}

func TestInterval(t *testing.T) {
	month := Interval{Months: 1}
	days30 := Interval{Days: 30}
	assert.True(t, month.Equal(days30), "a month is 30 days, as on the server")
	assert.NotEqual(t, month, days30, "== compares the parts")
	assert.True(t, Interval{Microseconds: 24 * 3600 * 1e6}.Equal(Interval{Days: 1}))
	assert.Equal(t, 1, month.Compare(Interval{Days: 29}))
	assert.Equal(t, -1, Interval{Microseconds: -1}.Compare(Interval{}))
	assert.Equal(t, 0, Interval{Days: -1, Microseconds: 86_400_000_000}.Compare(Interval{}))

	// The totals cannot overflow at the widest parts.
	widest := Interval{Months: math.MaxInt32, Days: math.MaxInt32, Microseconds: math.MaxInt64}
	assert.Equal(t, 1, widest.Compare(Interval{Months: math.MaxInt32, Days: math.MaxInt32}))
	assert.Equal(t, -1, Interval{Months: math.MinInt32, Days: math.MinInt32, Microseconds: math.MinInt64}.Compare(widest))

	assert.Equal(t, "P1Y2M3DT4H5M6.5S", Interval{14, 3, 14706500000}.String())
	assert.Equal(t, "PT0S", Interval{}.String())
	assert.Equal(t, "PT-1M-30S", Interval{Microseconds: -90_000_000}.String())
	assert.Equal(t, "ivl'P1D'", Interval{Days: 1}.SurrealString())

	var back Interval
	require.NoError(t, back.UnmarshalText([]byte("P1Y2M3DT4H5M6.5S")))
	assert.Equal(t, Interval{14, 3, 14706500000}, back)
	assert.Error(t, back.UnmarshalText([]byte("1 mon")))
}

func TestBigDecimal(t *testing.T) {
	for in, want := range map[string]string{
		"1.5":                              "1.5",
		"007.50":                           "7.50",
		"1.5e3":                            "1500",
		"-25E-3":                           "-0.025",
		".5":                               "0.5",
		"-0.00":                            "0.00",
		"0":                                "0",
		"79228162514264337593543950336.50": "79228162514264337593543950336.50",
	} {
		b, err := ParseBigDecimal(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, b.String(), in)
	}
	assert.Equal(t, BigDecimal{}, MustParseBigDecimal("0"), "the zero value is 0")
	assert.Equal(t, BigDecimal{}, MustParseBigDecimal("-0e9"))
	assert.Equal(t, "0", BigDecimal{}.String())
	assert.Equal(t, "1.50bigdec", MustParseBigDecimal("1.50").SurrealString())
	assert.Equal(t, "0bigdec", BigDecimal{}.SurrealString())

	for _, bad := range []string{"", " 1.5", "1,5", "NaN", "inf", "1e147455", "abc"} {
		_, err := ParseBigDecimal(bad)
		assert.Error(t, err, bad)
	}
	assert.Panics(t, func() { MustParseBigDecimal("x") })

	ordered := []string{
		"-79228162514264337593543950336.5", "-10", "-1.5", "-0.25", "0", "0.000001", "0.25", "1.5", "2", "10", "79228162514264337593543950336.5",
	}
	for i, a := range ordered {
		for j, b := range ordered {
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			assert.Equal(t, want, MustParseBigDecimal(a).Compare(MustParseBigDecimal(b)), "%s vs %s", a, b)
		}
	}
	assert.True(t, MustParseBigDecimal("1.50").Equal(MustParseBigDecimal("1.5")))
	assert.NotEqual(t, MustParseBigDecimal("1.50"), MustParseBigDecimal("1.5"), "== compares the digits as written")
	assert.True(t, MustParseBigDecimal("0.00").Equal(BigDecimal{}))
	assert.True(t, MustParseBigDecimal("-0.0").Equal(MustParseBigDecimal("0")))

	assert.True(t, MustParseBigDecimal("79228162514264337593543950335").FitsDecimal())
	assert.False(t, MustParseBigDecimal("79228162514264337593543950336").FitsDecimal())
	assert.True(t, MustParseBigDecimal("1e28").FitsDecimal())
	assert.False(t, MustParseBigDecimal("9e28").FitsDecimal())
	assert.True(t, MustParseBigDecimal("0."+strings.Repeat("1", 28)).FitsDecimal())
	assert.False(t, MustParseBigDecimal("0."+strings.Repeat("1", 29)).FitsDecimal())
	assert.True(t, BigDecimal{}.FitsDecimal())
}

func TestRegex(t *testing.T) {
	assert.Equal(t, "/a+/", Regex{Pattern: "a+"}.String())
	assert.Equal(t, `/a\/b/`, Regex{Pattern: "a/b"}.String())
	assert.Equal(t, "/a+/", Regex{Pattern: "a+"}.SurrealString())
	assert.Equal(t, "//", Regex{}.String())
	assert.Equal(t, "<regex> ''", Regex{}.SurrealString(), "// starts a comment in SurrealQL")

	// SurrealString escapes only a slash the pattern does not escape, as the
	// server's lexer skips an escape and the byte after it.
	for pattern, want := range map[string]string{
		"a/b":    `/a\/b/`,
		`a\/b`:   `/a\/b/`,
		`^\/api`: `/^\/api/`,
		`\\/`:    `/\\\//`,
		`\d+/\w`: `/\d+\/\w/`,
		"é/x":    `/é\/x/`,
	} {
		assert.Equal(t, want, Regex{Pattern: pattern}.SurrealString(), pattern)
	}

	for _, pattern := range []string{"", "a/b", `^\d+$`, `a\/b`, `\`, `\\/`} {
		var back Regex
		text, err := Regex{Pattern: pattern}.MarshalText()
		require.NoError(t, err)
		require.NoError(t, back.UnmarshalText(text), pattern)
		assert.Equal(t, pattern, back.Pattern)
	}
	var r Regex
	assert.Error(t, r.UnmarshalText([]byte("a+")))
	assert.Error(t, r.UnmarshalText([]byte("/")))
}

// The kinds write the server's JSON form: their Display text.
func TestKinds_JSON(t *testing.T) {
	type row struct {
		Date      PlainDate  `json:"date"`
		Time      PlainTime  `json:"time"`
		Timestamp Timestamp  `json:"timestamp"`
		Interval  Interval   `json:"interval"`
		Decimal   BigDecimal `json:"decimal"`
		Regex     Regex      `json:"regex"`
	}
	in := row{
		Date:      MustParsePlainDate("2024-02-29"),
		Time:      MustParsePlainTime("12:34:56.789"),
		Timestamp: TimestampInfinity,
		Interval:  Interval{Months: 1, Microseconds: -1500000},
		Decimal:   MustParseBigDecimal("1.50"),
		Regex:     Regex{Pattern: "a/b"},
	}
	data, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"date":"2024-02-29","time":"12:34:56.789","timestamp":"infinity",`+
		`"interval":"P1MT-1.5S","decimal":"1.50","regex":"/a\\/b/"}`, string(data))
	var out row
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, in, out)
}
