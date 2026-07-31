package surrealql_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegration_RecordRange_v2(t *testing.T) {
	runRecordRangeIntegration(t, "v2.3.7")
}

func TestIntegration_RecordRange_v3(t *testing.T) {
	runRecordRangeIntegration(t, "v3.0.4")
}

// runRecordRangeIntegration checks selecting records by id range against a live server.
func runRecordRangeIntegration(t *testing.T, version string) {
	t.Helper()

	db, cleanup := testenv.SetupVersionTest(t, version)
	defer cleanup()

	ctx := context.Background()

	ver, err := testenv.GetVersion(ctx, db)
	require.NoError(t, err)

	type Person struct {
		ID   models.RecordID `json:"id"`
		Name string          `json:"name"`
	}

	_, err = surrealdb.Query[any](ctx, db, `
		DEFINE TABLE person;
		DELETE person;
		CREATE person:1 SET name = 'one';
		CREATE person:2 SET name = 'two';
		CREATE person:3 SET name = 'three';
		CREATE person:10 SET name = 'ten';
	`, nil)
	require.NoError(t, err)

	t.Run("core Select with RecordRange", func(t *testing.T) {
		rr := models.NewRecordRange("person", models.Included(1), models.Included(3))
		got, err := surrealdb.Select[[]Person](ctx, db, rr)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Len(t, *got, 3)
		assert.Equal(t, "one", (*got)[0].Name)
		assert.Equal(t, "two", (*got)[1].Name)
		assert.Equal(t, "three", (*got)[2].Name)
	})

	t.Run("core Select with RecordID whose ID is a Range", func(t *testing.T) {
		// You can also put a Range in a RecordID; RecordRange is the clearer API.
		what := models.RecordID{
			Table: "person",
			ID: models.Range[int, models.BoundIncluded[int], models.BoundIncluded[int]]{
				Begin: models.Included(1),
				End:   models.Included(2),
			},
		}
		got, err := surrealdb.Select[[]Person](ctx, db, what)
		require.NoError(t, err)
		require.Len(t, *got, 2)
	})

	t.Run("surrealql Select with RecordRange", func(t *testing.T) {
		sql, vars := surrealql.Select(surrealql.RecordRange("person",
			models.Included(2),
			models.Included(3),
		)).Build()

		t.Logf("sql=%s vars=%v", sql, vars)
		results, err := surrealdb.Query[[]Person](ctx, db, sql, vars)
		require.NoError(t, err)
		require.NotNil(t, results)
		require.Len(t, *results, 1)
		if (*results)[0].Error != nil {
			t.Fatalf("query status=%s error=%v result=%v", (*results)[0].Status, (*results)[0].Error, (*results)[0].Result)
		}
		require.Len(t, (*results)[0].Result, 2)
		assert.Equal(t, "two", (*results)[0].Result[0].Name)
		assert.Equal(t, "three", (*results)[0].Result[1].Name)
	})

	t.Run("legacy type::thing / type::record workaround", func(t *testing.T) {
		// SurrealDB 2.x uses type::thing; 3.x uses type::record.
		r := models.Range[int, models.BoundIncluded[int], models.BoundIncluded[int]]{
			Begin: models.Included(1),
			End:   models.Included(1),
		}
		params := map[string]any{
			"tb":     models.Table("person"),
			"recrng": r,
		}
		sql := fmt.Sprintf("SELECT * FROM %s($tb, $recrng)", ver.ThingOrRecordFn())
		results, err := surrealdb.Query[[]Person](ctx, db, sql, params)
		require.NoError(t, err)
		require.Len(t, (*results)[0].Result, 1)
		assert.Equal(t, "one", (*results)[0].Result[0].Name)
	})

	t.Run("range over array-style record ids", func(t *testing.T) {
		_, err := surrealdb.Query[any](ctx, db, `
			DEFINE TABLE temp;
			DELETE temp;
			CREATE temp:['London', 1] SET city = 'London', n = 1;
			CREATE temp:['London', 2] SET city = 'London', n = 2;
			CREATE temp:['Paris', 1] SET city = 'Paris', n = 1;
		`, nil)
		require.NoError(t, err)

		type Temp struct {
			City string `json:"city"`
			N    int    `json:"n"`
		}

		rr := models.NewRecordRange("temp",
			models.IncludedMany[any]("London", models.None),
			models.IncludedMany[any]("London", models.OpenRange()),
		)

		got, err := surrealdb.Select[[]Temp](ctx, db, rr)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Len(t, *got, 2, "should return only London records")
		assert.Equal(t, "London", (*got)[0].City)
		assert.Equal(t, "London", (*got)[1].City)
	})
}
