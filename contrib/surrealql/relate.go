package surrealql

import "strings"

// RelateQuery represents a RELATE query
type RelateQuery struct {
	setsBuilder
	from         *expr
	edge         string
	to           *expr
	content      map[string]any
	useContent   bool
	returnClause string
	only         bool
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

// RelateOnly starts a RELATE ONLY query that creates and returns only one relation
func RelateOnly[T exprLike](from T, edge string, to T) *RelateQuery {
	r := Relate(from, edge, to)
	r.only = true
	return r
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
	var b strings.Builder

	b.WriteString("RELATE ")

	if q.only {
		b.WriteString("ONLY ")
	}

	b.WriteString(q.from.build(c))
	b.WriteString("->")
	b.WriteString(escapeIdent(q.edge))
	b.WriteString("->")
	b.WriteString(q.to.build(c))

	if q.useContent && len(q.content) > 0 {
		paramName := c.generateAndAddParam("content", q.content)
		b.WriteString(" CONTENT $")
		b.WriteString(paramName)
	} else if setClause := q.buildSetClause(c); setClause != "" {
		b.WriteString(" SET ")
		b.WriteString(setClause)
	}

	if q.returnClause != "" {
		b.WriteString(" RETURN ")
		b.WriteString(q.returnClause)
	}

	return b.String()
}

// String returns the SurrealQL string
func (q *RelateQuery) String() string {
	sql, _ := q.Build()
	return sql
}
