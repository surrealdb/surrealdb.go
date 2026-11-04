package surrealdb_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/v2/pkg/connection"
	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// requireKinds skips the test unless the server knows SurrealDB 4.0's date,
// time, timestamp, interval, bigdecimal and regex kinds and their literals,
// as 4.0.0 does: a server without them refuses to parse pd”. Its answer is a
// string, so a failure to decode a kind cannot pass for a skip. Over HTTP it
// also makes the connection speak cbor.2 if the server does, as the first
// request is plain CBOR.
func requireKinds(ctx context.Context, t *testing.T, db *surrealdb.DB) {
	t.Helper()
	res, err := surrealdb.Query[string](ctx, db, "RETURN type::of(pd'2024-01-15')", nil)
	var serverErr surrealdb.ServerError
	if errors.As(err, &serverErr) {
		t.Skipf("the server does not know the SurrealDB 4.0 kinds: %v", err)
	}
	require.NoError(t, err)
	require.Equal(t, "date", (*res)[0].Result)
}

// kindValues has one value of each kind, and kindLiterals the same values as
// SurrealQL literals.
func kindValues() []any {
	return []any{
		models.MustParsePlainDate("2024-01-15"),
		models.MustParsePlainTime("12:34:56.5"),
		models.MustParseTimestamp("2024-01-15T12:34:56.123456789"),
		models.Interval{Months: 14, Days: 3, Microseconds: 3_723_500_000},
		models.MustParseBigDecimal("12345678901234567890123456789012345.5"),
		models.Regex{Pattern: "^a+$"},
	}
}

const kindLiterals = "[pd'2024-01-15', pt'12:34:56.5', ts'2024-01-15T12:34:56.123456789', " +
	"ivl'P1Y2M3DT1H2M3.5S', 12345678901234567890123456789012345.5bigdec, /^a+$/]"

func usesHTTP() bool {
	return strings.HasPrefix(testenv.GetSurrealDBURL(), "http")
}

// TestCBOR2Kinds round-trips SurrealDB 4.0's date, time, timestamp,
// interval, bigdecimal and regex kinds over cbor.2.
func TestCBOR2Kinds(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("cbor2", "kinds", "t")
	defer db.Close(ctx)
	requireKinds(ctx, t, db)
	if db.CBORVersion() != connection.CBORv2 {
		t.Skip("the server did not negotiate cbor.2")
	}

	t.Run("from SurrealQL literals", func(t *testing.T) {
		res, err := surrealdb.Query[[]any](ctx, db, "RETURN "+kindLiterals, nil)
		require.NoError(t, err)
		assert.Equal(t, kindValues(), (*res)[0].Result)
	})

	t.Run("through a parameter, keeping their kinds", func(t *testing.T) {
		res, err := surrealdb.Query[any](ctx, db, "RETURN $v.map(|$x| type::of($x)); RETURN $v",
			map[string]any{"v": kindValues()})
		require.NoError(t, err)
		assert.Equal(t, []any{"date", "time", "timestamp", "interval", "bigdecimal", "regex"}, (*res)[0].Result)
		assert.Equal(t, kindValues(), (*res)[1].Result)
	})

	t.Run("through a variable set with Let", func(t *testing.T) {
		defer func() { _ = db.Unset(ctx, "kind") }()
		for _, v := range kindValues() {
			err := db.Let(ctx, "kind", v)
			if _, isRegex := v.(models.Regex); isRegex && usesHTTP() {
				// An HTTP connection keeps a plain CBOR copy of each variable,
				// which a regex has no form for.
				require.ErrorIs(t, err, constants.ErrNeedsCBOR2)
				continue
			}
			require.NoError(t, err)
			res, err := surrealdb.Query[any](ctx, db, "RETURN $kind", nil)
			require.NoError(t, err)
			assert.Equal(t, v, (*res)[0].Result)
		}
	})

	t.Run("stored in a record", func(t *testing.T) {
		type row struct {
			ID       *models.RecordID  `json:"id,omitempty"`
			Due      models.PlainDate  `json:"due"`
			Opens    models.PlainTime  `json:"opens"`
			At       models.Timestamp  `json:"at"`
			Every    models.Interval   `json:"every"`
			Price    models.BigDecimal `json:"price"`
			Pattern  models.Regex      `json:"pattern"`
			Optional *models.PlainDate `json:"optional,omitempty"`
		}
		values := kindValues()
		in := row{
			Due:     values[0].(models.PlainDate),
			Opens:   values[1].(models.PlainTime),
			At:      values[2].(models.Timestamp),
			Every:   values[3].(models.Interval),
			Price:   values[4].(models.BigDecimal),
			Pattern: values[5].(models.Regex),
		}
		created, err := surrealdb.Create[row](ctx, db, models.NewRecordID("t", "kinds"), in)
		require.NoError(t, err)
		require.NotNil(t, created.ID)
		got, err := surrealdb.Select[row](ctx, db, *created.ID)
		require.NoError(t, err)
		in.ID = created.ID
		assert.Equal(t, in, *got)

		res, err := surrealdb.Query[[]any](ctx, db,
			"SELECT VALUE [type::of(due), type::of(opens), type::of(at), type::of(every), type::of(price), type::of(pattern)] FROM ONLY $id",
			map[string]any{"id": *created.ID})
		require.NoError(t, err)
		assert.Equal(t, []any{"date", "time", "timestamp", "interval", "bigdecimal", "regex"}, (*res)[0].Result)
	})

	t.Run("infinities", func(t *testing.T) {
		res, err := surrealdb.Query[[]any](ctx, db, "RETURN [pd'infinity', pd'-infinity', ts'infinity', ts'-infinity']", nil)
		require.NoError(t, err)
		assert.Equal(t, []any{
			models.PlainDateInfinity, models.PlainDateNegInfinity, models.TimestampInfinity, models.TimestampNegInfinity,
		}, (*res)[0].Result)
	})

	t.Run("into Go types, as over plain CBOR", func(t *testing.T) {
		type row struct {
			Due   time.Time     `json:"due"`
			Opens time.Duration `json:"opens"`
			Every string        `json:"every"`
			Price float64       `json:"price"`
		}
		res, err := surrealdb.Query[row](ctx, db, "RETURN {due: pd'2024-01-15', opens: pt'09:30', every: ivl'P1D', price: 1.5bigdec}", nil)
		require.NoError(t, err)
		assert.Equal(t, row{
			Due:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Opens: 9*time.Hour + 30*time.Minute,
			Every: "P1D",
			Price: 1.5,
		}, (*res)[0].Result)
	})

	t.Run("a bigdecimal that fits a decimal stays a bigdecimal", func(t *testing.T) {
		// SurrealDB 4.0.0 sends it as tag 192, where protocol v0.15.0 says tag 10.
		res, err := surrealdb.Query[any](ctx, db, "RETURN 1.50bigdec", nil)
		require.NoError(t, err)
		assert.Equal(t, models.MustParseBigDecimal("1.50"), (*res)[0].Result)
	})

	t.Run("intervals compare as the server compares them", func(t *testing.T) {
		month, days := models.Interval{Months: 1}, models.Interval{Days: 30}
		res, err := surrealdb.Query[bool](ctx, db, "RETURN $a = $b", map[string]any{"a": month, "b": days})
		require.NoError(t, err)
		assert.True(t, (*res)[0].Result)
		assert.True(t, month.Equal(days))
	})
}

// TestCBOR2Kinds_plainCBOR reads and sends the kinds over plain CBOR, where
// the server and the SDK widen them, and a regex cannot travel at all.
func TestCBOR2Kinds_plainCBOR(t *testing.T) {
	ctx := context.Background()
	config := testenv.MustNewConfig("cbor2", "kindsplain", "t")
	config.DisableCBORv2 = true
	db := config.MustNew()
	defer db.Close(ctx)
	requireKinds(ctx, t, db)
	require.Equal(t, connection.CBORv1, db.CBORVersion())

	t.Run("widened by the server", func(t *testing.T) {
		res, err := surrealdb.Query[[]any](ctx, db, "RETURN "+strings.Replace(kindLiterals, ", /^a+$/", ", 1.5bigdec", 1), nil)
		require.NoError(t, err)
		assert.Equal(t, []any{
			models.CustomDateTime{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
			models.CustomDuration{Duration: 12*time.Hour + 34*time.Minute + 56*time.Second + 500*time.Millisecond},
			models.CustomDateTime{Time: time.Date(2024, 1, 15, 12, 34, 56, 123456789, time.UTC)},
			"P1Y2M3DT1H2M3.5S",
			"12345678901234567890123456789012345.5",
			models.DecimalString("1.5"),
		}, (*res)[0].Result)
	})

	t.Run("read into the kinds from their widened forms", func(t *testing.T) {
		type row struct {
			Due   models.PlainDate  `json:"due"`
			Opens models.PlainTime  `json:"opens"`
			At    models.Timestamp  `json:"at"`
			Every models.Interval   `json:"every"`
			Price models.BigDecimal `json:"price"`
		}
		res, err := surrealdb.Query[row](ctx, db, "RETURN {due: pd'2024-01-15', opens: pt'12:34:56.5', "+
			"at: ts'2024-01-15T12:34:56.123456789', every: ivl'P1Y2M3DT1H2M3.5S', price: 12345678901234567890123456789012345.5bigdec}", nil)
		require.NoError(t, err)
		values := kindValues()
		assert.Equal(t, row{
			Due:   values[0].(models.PlainDate),
			Opens: values[1].(models.PlainTime),
			At:    values[2].(models.Timestamp),
			Every: values[3].(models.Interval),
			Price: values[4].(models.BigDecimal),
		}, (*res)[0].Result)
	})

	t.Run("the date infinity widened to the latest datetime", func(t *testing.T) {
		// SurrealDB 4.0.0 widens it to its latest datetime, where protocol
		// v0.15.0 says midnight of the last day.
		res, err := surrealdb.Query[any](ctx, db, "RETURN pd'infinity'", nil)
		require.NoError(t, err)
		assert.Equal(t, models.CustomDateTime{Time: time.Unix(8210266876799, 999999999).UTC()}, (*res)[0].Result)

		date, err := surrealdb.Query[models.PlainDate](ctx, db, "RETURN pd'infinity'", nil)
		require.NoError(t, err)
		assert.Equal(t, models.PlainDateInfinity, (*date)[0].Result)
	})

	t.Run("widened by the SDK", func(t *testing.T) {
		res, err := surrealdb.Query[[]any](ctx, db, "RETURN $v.map(|$x| type::of($x))", map[string]any{"v": kindValues()[:5]})
		require.NoError(t, err)
		assert.Equal(t, []any{"datetime", "duration", "datetime", "string", "string"}, (*res)[0].Result)
	})

	t.Run("a regex cannot travel", func(t *testing.T) {
		_, err := surrealdb.Query[any](ctx, db, "RETURN $v", map[string]any{"v": models.Regex{Pattern: "a"}})
		require.ErrorIs(t, err, constants.ErrNeedsCBOR2)
		_, err = surrealdb.Query[any](ctx, db, "RETURN /a/", nil)
		require.Error(t, err, "the server cannot send a regex over plain CBOR")
	})
}
