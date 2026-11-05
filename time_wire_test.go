package surrealdb_test

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// TestQuery_valuesTravelInCompactForms sends a time.Time, a UUIDString, a
// CustomDurationString and a uint64 above math.MaxInt64 as query variables,
// and checks that the server reads each as the value it is. The SDK used to
// send the first three as text tags that SurrealDB 4.0 refuses, dropping a
// time.Time's fraction of a second, and the uint64 as an integer that 3.x
// wrapped to a negative number.
func TestQuery_valuesTravelInCompactForms(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("wire_forms", "compact", "dummy")
	t.Cleanup(func() { _ = db.Close(ctx) })

	at := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	res, err := surrealdb.Query[[]any](ctx, db, `RETURN [
		$at == d"2026-01-02T03:04:05.123456789Z",
		$id == u"0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b",
		$dur == 1h30m,
		$big > 0,
		<string> $big
	]`, map[string]any{
		"at":  at,
		"id":  models.UUIDString("0190b1a2-7c3e-7d1f-8a2b-3c4d5e6f7a8b"),
		"dur": models.CustomDurationString("1h30m"),
		"big": uint64(18446744073709551615),
	})
	require.NoError(t, err)
	require.Nil(t, (*res)[0].Error, "%+v", (*res)[0].Error)
	got := (*res)[0].Result
	require.Len(t, got, 5)
	assert.Equal(t, []any{true, true, true, true}, got[:4], "each value is read as what it is")
	assert.Contains(t, got[4], "18446744073709551615")

	// A time.Time comes back to the nanosecond.
	back, err := surrealdb.Query[models.CustomDateTime](ctx, db, "RETURN $at", map[string]any{"at": at})
	require.NoError(t, err)
	assert.True(t, at.Equal((*back)[0].Result.Time), "%v != %v", at, (*back)[0].Result.Time)
}

// TestQuery_longDurationsAndWideYearsTravel sends durations longer than a
// time.Duration can hold, about 292 years, and times outside the years 0000 to
// 9999. Such durations used to wrap without an error, and such times failed
// to marshal.
func TestQuery_longDurationsAndWideYearsTravel(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("wire_forms", "long", "dummy")
	t.Cleanup(func() { _ = db.Close(ctx) })

	for _, text := range []string{"300y", "600y", "1000y", "584942417355y"} {
		t.Run(text, func(t *testing.T) {
			vars := map[string]any{"d": models.CustomDurationString(text)}
			res, err := surrealdb.Query[string](ctx, db, "RETURN <string> $d", vars)
			require.NoError(t, err)
			assert.Equal(t, text, (*res)[0].Result, "the server reads the duration it was sent")

			if os.Getenv(testenv.EnvSurrealCBORImpl) == "fxamackercbor" {
				return // the deprecated decoder reads tag 14 only into time.Duration types
			}
			back, err := surrealdb.Query[models.CustomDurationString](ctx, db, "RETURN $d", vars)
			require.NoError(t, err)
			assert.Equal(t, models.CustomDurationString(text), (*back)[0].Result, "and it reads back the same")
		})
	}

	for _, at := range []time.Time{
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		t.Run(at.Format(time.RFC3339), func(t *testing.T) {
			res, err := surrealdb.Query[models.CustomDateTime](ctx, db, "RETURN $at", map[string]any{"at": at})
			require.NoError(t, err)
			assert.True(t, at.Equal((*res)[0].Result.Time), "%v != %v", at, (*res)[0].Result.Time)
		})
	}
}

// TestQuery_durationsTravelAsDurations sends a time.Duration, alone and inside
// a struct, and checks that the server reads it as a duration. The SDK used to
// send it as an integer number of nanoseconds, which the server stored as an
// int, though the SDK reads a duration back into a time.Duration.
func TestQuery_durationsTravelAsDurations(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("wire_forms", "durations", "dummy")
	t.Cleanup(func() { _ = db.Close(ctx) })

	type job struct {
		Every time.Duration `json:"every"`
		Delay time.Duration `json:"delay,omitempty"`
	}
	res, err := surrealdb.Query[[]any](ctx, db, `RETURN [
		type::of($d), $d == 1m30s,
		type::of($job.every), $job.every == 2h,
		type::of($job.delay)
	]`, map[string]any{
		"d":   90 * time.Second,
		"job": job{Every: 2 * time.Hour},
	})
	require.NoError(t, err)
	require.Nil(t, (*res)[0].Error, "%+v", (*res)[0].Error)
	assert.Equal(t, []any{"duration", true, "duration", true, "none"}, (*res)[0].Result,
		"a zero duration tagged omitempty is left out, as an integer one was")

	back, err := surrealdb.Query[models.CustomDuration](ctx, db, "RETURN $d", map[string]any{"d": 90 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, (*back)[0].Result.Duration)
}

// TestQuery_regexpIsRefused checks that a *regexp.Regexp, which the SDK used to
// send as an empty object, is refused before anything is sent.
func TestQuery_regexpIsRefused(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("wire_forms", "regexp", "dummy")
	t.Cleanup(func() { _ = db.Close(ctx) })

	_, err := surrealdb.Query[any](ctx, db, "RETURN $re", map[string]any{"re": regexp.MustCompile("a.b")})
	assert.ErrorContains(t, err, "models.Regex")
}
