package surrealql

import "fmt"

// RelateQuery represents a RELATE query
type RelateQuery struct {
	setsBuilder
	from         *expr
	edge         string
	to           *expr
	content      map[string]any
	useContent   bool
	returnClause string
}

// Relate starts a RELATE query
func Relate[T exprLike](from T, edge string, to T) *RelateQuery {
	return &RelateQuery{
		setsBuilder: newSetsBuilder(),
		from:        Expr(from),
		edge:        edge,
		to:          Expr(to),
		content:     make(map[string]any),
	}
}

// Set adds a field or expression to the relation
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *RelateQuery) Set(expr string, args ...any) *RelateQuery {
	q.addSet(expr, args)
	return q
}

// Content sets the entire content for the relation
func (q *RelateQuery) Content(content map[string]any) *RelateQuery {
	q.content = content
	q.useContent = true
	return q
}

// Return sets the RETURN clause
func (q *RelateQuery) Return(clause string) *RelateQuery {
	q.returnClause = clause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *RelateQuery) Build() (sql string, vars map[string]any) {
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *RelateQuery) build(c *queryBuildContext) (sql string) {
	from := q.from.Build(c)
	to := q.to.Build(c)

	// Don't escape record IDs with colons, only escape the edge table name
	sql = fmt.Sprintf("RELATE %s->%s->%s",
		from,
		escapeIdent(q.edge),
		to)

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
func (q *RelateQuery) String() string {
	sql, _ := q.Build()
	return sql
}
