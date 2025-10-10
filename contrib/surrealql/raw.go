package surrealql

import (
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
	var b strings.Builder
	q.build(&c, &b)
	return b.String(), c.vars
}

func (q *rawQuery) build(c *queryBuildContext, b *strings.Builder) {
	// Replace ? placeholders with positional arguments using context
	var start int
	for _, arg := range q.args {
		paramName := c.generateAndAddParam("raw", arg)
		placeholder := strings.Index(q.sql[start:], "?")
		if placeholder < 0 {
			break
		}
		placeholder += start
		b.WriteString(q.sql[start:placeholder])
		b.WriteString("$")
		b.WriteString(paramName)
		start = placeholder + 1
	}
	b.WriteString(q.sql[start:])
}

func (q *rawQuery) String() string {
	sql, _ := q.Build()
	return sql
}
