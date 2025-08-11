package surrealql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelect(t *testing.T) {
	tests := []struct {
		name      string
		query     Query
		wantSurQL string
		wantArgs  map[string]any
	}{
		{
			name:      "simple select all",
			query:     Select("users"),
			wantSurQL: "SELECT * FROM users",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select specific fields",
			query:     Select("users").Fields("id", "name", "email"),
			wantSurQL: "SELECT id, name, email FROM users",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with where equals",
			query:     Select("users").WhereEq("active", true),
			wantSurQL: "SELECT * FROM users WHERE type::field($param_1) = $param_2",
			wantArgs:  map[string]any{"param_1": "active", "param_2": true},
		},
		{
			name:      "select with where in",
			query:     Select("users").WhereIn("status", []string{"active", "pending"}),
			wantSurQL: "SELECT * FROM users WHERE type::field($param_1) IN $param_2",
			wantArgs:  map[string]any{"param_1": "status", "param_2": []string{"active", "pending"}},
		},
		{
			name:      "select with order by",
			query:     Select("users").OrderBy("created_at"),
			wantSurQL: "SELECT * FROM users ORDER BY created_at",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with order by desc",
			query:     Select("users").OrderByDesc("created_at"),
			wantSurQL: "SELECT * FROM users ORDER BY created_at DESC",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with limit and start",
			query:     Select("users").Limit(10).Start(20),
			wantSurQL: "SELECT * FROM users LIMIT 10 START 20",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with return none",
			query:     Select("users").ReturnNone(),
			wantSurQL: "SELECT * FROM users RETURN NONE",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with group by",
			query:     Select("products").Fields("category", "count() AS total").GroupBy("category"),
			wantSurQL: "SELECT category, count() AS total FROM products GROUP BY category",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with multiple where conditions",
			query:     Select("users").WhereEq("active", true).WhereNotNull("email"),
			wantSurQL: "SELECT * FROM users WHERE type::field($param_1) = $param_2 AND type::field($param_3) IS NOT NULL",
			wantArgs:  map[string]any{"param_1": "active", "param_2": true, "param_3": "email"},
		},
		{
			name:      "select with fetch",
			query:     Select("posts").Fetch("author", "comments"),
			wantSurQL: "SELECT * FROM posts FETCH author, comments",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with parallel",
			query:     Select("users").Parallel(),
			wantSurQL: "SELECT * FROM users PARALLEL",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with explain",
			query:     Select("users").Explain(),
			wantSurQL: "EXPLAIN SELECT * FROM users",
			wantArgs:  map[string]any{},
		},
		{
			name:      "select with complex where",
			query:     Select("orders").Where("total > ? AND status = ?", 100, "pending"),
			wantSurQL: "SELECT * FROM orders WHERE total > $param_1 AND status = $param_2",
			wantArgs:  map[string]any{"param_1": 100, "param_2": "pending"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSurQL, gotArgs := tt.query.Build()

			if gotSurQL != tt.wantSurQL {
				t.Errorf("SurrealQL mismatch\ngot:  %q\nwant: %q", gotSurQL, tt.wantSurQL)
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("Args count mismatch\ngot:  %d\nwant: %d", len(gotArgs), len(tt.wantArgs))
			}

			for k, v := range tt.wantArgs {
				assert.Equal(t, v, gotArgs[k], "Arg %q mismatch", k)
			}
		})
	}
}
