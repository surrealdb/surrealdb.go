package surrealdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestRecordIDAsVariable_WorksAcrossVersions verifies that using models.RecordID
// as a query variable works on both SurrealDB 2.x and 3.x, avoiding the need
// for type::thing() (2.x) or type::record() (3.x).
func TestRecordIDAsVariable_WorksAcrossVersions(t *testing.T) {
	db := testenv.MustNew("test_version_compat", "test_db", "persons")
	ctx := context.Background()

	recordID := models.NewRecordID("persons", "test123")

	// CREATE using RecordID variable
	_, err := surrealdb.Query[any](ctx, db,
		`CREATE $id CONTENT {name: "Test"}`,
		map[string]any{"id": recordID},
	)
	require.NoError(t, err)

	// SELECT using RecordID variable
	results, err := surrealdb.Query[[]map[string]any](ctx, db,
		`SELECT * FROM $id`,
		map[string]any{"id": recordID},
	)
	require.NoError(t, err)
	require.Len(t, *results, 1)
	require.Equal(t, "OK", (*results)[0].Status)
	require.Len(t, (*results)[0].Result, 1)
}

// TestVersionDetection verifies that version detection works correctly.
func TestVersionDetection(t *testing.T) {
	db := testenv.MustNew("test_version_compat", "test_db", "dummy")
	ctx := context.Background()

	v, err := testenv.GetVersion(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, v)

	// Version should be either 2.x or 3.x
	require.True(t, v.Major >= 2, "Expected major version >= 2, got %d", v.Major)

	// ThingOrRecordFn should return appropriate function name
	if v.Major >= 3 {
		require.Equal(t, "type::record", v.ThingOrRecordFn())
	} else {
		require.Equal(t, "type::thing", v.ThingOrRecordFn())
	}
}
