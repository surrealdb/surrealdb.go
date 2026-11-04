package surrealdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// TestRecordIDAsVariable_WorksAcrossVersions verifies that using models.RecordID
// as a query variable works, avoiding the need for type::record().
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

	// The SDK supports SurrealDB 3.3 and later.
	require.True(t, v.Major >= 3, "Expected major version >= 3, got %d", v.Major)
}
