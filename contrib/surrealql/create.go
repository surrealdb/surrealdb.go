package surrealql

import (
	"fmt"
	"strings"
)

// CreateQuery represents a CREATE query
type CreateQuery struct {
	setsBuilder
	targets      []*expr
	content      map[string]any
	useContent   bool
	returnClause string
}

// Create starts a CREATE query
// The `thing` parameter is either a table name, or a record ID with a colon (e.g., "users:123").
// If you want to create a new record without specifying an ID, use just the table name (e.g., "users").
// If you want to create a new record with a specific ID, use the format "table:id" (e.g., "users:123").
// A special case to note for `table:id` is that when the `id` is a number-like string (e.g., "123")
// "table:123" will treat `123` as an integer ID,
// while "table:`123`" will treat `123` as a string ID.
func Create[T exprLike](target T, targets ...T) *CreateQuery {
	var ts []*expr
	ts = append(ts, Expr(target))
	for _, target := range targets {
		ts = append(ts, Expr(target))
	}
	return &CreateQuery{
		setsBuilder: newSetsBuilder(),
		targets:     ts,
		content:     make(map[string]any),
	}
}

// Set adds a field or expression to set in the CREATE query
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *CreateQuery) Set(expr string, args ...any) *CreateQuery {
	q.addSet(expr, args)
	return q
}

// Content sets the entire content for the CREATE query
func (q *CreateQuery) Content(content map[string]any) *CreateQuery {
	q.content = content
	q.useContent = true
	return q
}

// Return sets the RETURN clause
func (q *CreateQuery) Return(clause string) *CreateQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *CreateQuery) ReturnNone() *CreateQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *CreateQuery) Build() (sql string, vars map[string]any) {
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *CreateQuery) build(c *queryBuildContext) (sql string) {
	var targets []string

	for _, t := range q.targets {
		targets = append(targets, t.Build(c))
	}

	sql = fmt.Sprintf("CREATE %s", strings.Join(targets, ", "))

	if q.useContent && len(q.content) > 0 {
		paramName := c.generateAndAddParam("content", q.content)
		sql += fmt.Sprintf(" CONTENT $%s", paramName)
	} else if setClause := q.buildSetClause(c); setClause != "" {
		sql += " SET " + setClause
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}

// String returns the SurrealQL string
func (q *CreateQuery) String() string {
	sql, _ := q.Build()
	return sql
}
