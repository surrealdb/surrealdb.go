package surrealql_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegrationDefineTable(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "events")

	ctx := context.Background()

	t.Run("DefineTableWithChangefeed", func(t *testing.T) {
		// Define a table with changefeed
		defineQuery := surrealql.DefineTable("events").
			Schemafull().
			Changefeed("1h")

		ql, vars := defineQuery.Build()
		t.Logf("DEFINE TABLE SurrealQL: %s", ql)

		_, err := surrealdb.Query[any](ctx, db, ql, vars)
		if err != nil {
			t.Fatalf("DEFINE TABLE failed: %v", err)
		}

		// Define fields
		fieldQuery := surrealql.DefineField("timestamp", "events").
			Type("datetime").
			Default("time::now()")

		ql, vars = fieldQuery.Build()
		_, err = surrealdb.Query[any](ctx, db, ql, vars)
		if err != nil {
			t.Fatalf("DEFINE FIELD failed: %v", err)
		}

		// Create some events using query
		type Event struct {
			ID        models.RecordID       `json:"id,omitempty"`
			Timestamp models.CustomDateTime `json:"timestamp"`
			Action    string                `json:"action"`
		}

		createEvent := surrealql.Create("events").
			Set("action", "user_login")
		ql, vars = createEvent.Build()
		_, err = surrealdb.Query[[]Event](ctx, db, ql, vars)
		if err != nil {
			t.Fatalf("Failed to create event: %v", err)
		}

		// Show changes (this will only work if enough time has passed)
		showQuery := surrealql.ShowChangesForTable("events").
			Since("0").
			Limit(10)

		ql, vars = showQuery.Build()
		t.Logf("SHOW CHANGES SurrealQL: %s", ql)

		// The query should be valid even if no changes are returned yet
		_, err = surrealdb.Query[any](ctx, db, ql, vars)
		if err != nil {
			// Some versions of SurrealDB might not support SHOW CHANGES yet
			t.Logf("SHOW CHANGES query failed (might not be supported): %v", err)
		}
	})
}
