package surrealql

import (
	"fmt"
	"strings"
)

// Helper function to create raw queries.
// This supports placeholder parameters.
func Raw(sql string, args ...any) Query {
	return &rawQuery{
		sql:  sql,
		args: args,
	}
}

type rawQuery struct {
	sql  string
	args []any
}

func (q *rawQuery) Build() (sql string, params map[string]any) {
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *rawQuery) build(c *queryBuildContext) (sql string) {
	// Replace ? placeholders with positional arguments using context
	sql = q.sql
	for _, arg := range q.args {
		paramName := c.generateAndAddParam("raw", arg)
		sql = strings.Replace(sql, "?", fmt.Sprintf("$%s", paramName), 1)
	}
	return sql
}

func (q *rawQuery) String() string {
	sql, _ := q.Build()
	return sql
}
