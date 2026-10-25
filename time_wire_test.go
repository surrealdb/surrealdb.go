package surrealdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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
