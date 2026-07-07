package surrealql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

type smallIntConfig struct {
	MaxSteps int `json:"max_steps"`
	Priority int `json:"priority"`
}

func TestIntegration_SmallIntsDecodeToIntFields(t *testing.T) {
	db, cleanup := testenv.SetupVersionTest(t, "v3.0.4")
	defer cleanup()

	ctx := context.Background()

	_, err := surrealdb.Query[[]smallIntConfig](ctx, db,
		"CREATE config:smallint SET max_steps = 0, priority = 1", nil)
	require.NoError(t, err)

	results, err := surrealdb.Query[[]smallIntConfig](ctx, db, "SELECT * FROM config:smallint", nil)
	require.NoError(t, err)
	require.Len(t, (*results)[0].Result, 1)

	cfg := (*results)[0].Result[0]
	require.Equal(t, 0, cfg.MaxSteps)
	require.Equal(t, 1, cfg.Priority)
}
