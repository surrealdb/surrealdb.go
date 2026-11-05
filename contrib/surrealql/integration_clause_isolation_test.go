package surrealql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/v2/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

// Caller text in one clause cannot change what the statement does. A comment
// in a condition used to drop the clauses after it, an OR in one condition to
// take in the others, a comment in a SET expression to drop the WHERE, and a
// field name to run a statement of its own or to be read as a value.
func TestIntegrationClausesKeepCallerTextInPlace(t *testing.T) {
	db := testenv.MustNew("surrealqlexamples", "clause_isolation", "item", "pwned")
	ctx := context.Background()
	_, err := surrealdb.Query[any](ctx, db, `
		REMOVE TABLE IF EXISTS item; DEFINE TABLE item SCHEMALESS;
		REMOVE TABLE IF EXISTS pwned; DEFINE TABLE pwned SCHEMALESS;
		CREATE item:a1 SET n = 1, tenant = "a", status = "x", none = "a field";
		CREATE item:a2 SET n = 2, tenant = "a", status = "y", none = "a field";
		CREATE item:b1 SET n = 3, tenant = "b", status = "x", none = "a field";
	`, nil)
	require.NoError(t, err)

	ids := func(q surrealql.Query) []models.RecordID {
		t.Helper()
		sql, vars := q.Build()
		res, qerr := surrealdb.Query[[]struct {
			ID models.RecordID `json:"id"`
		}](ctx, db, sql, vars)
		require.NoError(t, qerr)
		var out []models.RecordID
		for _, r := range (*res)[0].Result {
			out = append(out, r.ID)
		}
		return out
	}
	item := func(id string) models.RecordID { return models.RecordID{Table: "item", ID: id} }

	assert.Equal(t, []models.RecordID{item("a1")},
		ids(surrealql.Select("item").Where("n >= ? -- at least", 1).Where("tenant = ?", "a").OrderBy("n").Limit(1)),
		"the tenant condition, ORDER BY and LIMIT still apply")

	sql, vars := surrealql.Select("item").FieldName("none").Where("n = ?", 1).Build()
	fields, err := surrealdb.Query[[]map[string]any](ctx, db, sql, vars)
	require.NoError(t, err)
	assert.Equal(t, []map[string]any{{"none": "a field"}}, (*fields)[0].Result, "the field, not the value NONE")

	sql, vars = surrealql.Select("item").OrderBy("n;CREATE/**/pwned").Build()
	_, _ = surrealdb.Query[any](ctx, db, sql, vars)
	assert.Empty(t, ids(surrealql.Select("pwned")), "the field name ran no statement")

	assert.Equal(t, []models.RecordID{item("a1")},
		ids(surrealql.Delete("item").Where("status = ? OR n = ?", "x", 99).Where("tenant = ?", "a").ReturnBefore()),
		"only tenant a's record was deleted")

	assert.Equal(t, []models.RecordID{item("b1")},
		ids(surrealql.Update("item").Set("status = 'z' -- mark").Where("tenant = ?", "b")),
		"only tenant b's record was updated")
}
