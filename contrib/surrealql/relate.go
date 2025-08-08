package surrealql

import "fmt"

// RelateQuery represents a RELATE query
type RelateQuery struct {
	baseQuery
	setsBuilder
	from         string
	edge         string
	to           string
	content      map[string]any
	useContent   bool
	returnClause string
}

// Relate starts a RELATE query
func Relate(from, edge, to string) *RelateQuery {
	return &RelateQuery{
		baseQuery:   newBaseQuery(),
		setsBuilder: newSetsBuilder(),
		from:        from,
		edge:        edge,
		to:          to,
		content:     make(map[string]any),
	}
}

// Set adds a field or expression to the relation
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *RelateQuery) Set(expr string, args ...any) *RelateQuery {
	q.addSet(expr, args, &q.baseQuery, "param")
	return q
}

// Content sets the entire content for the relation
func (q *RelateQuery) Content(content map[string]any) *RelateQuery {
	q.content = content
	q.useContent = true
	return q
}

// SetMap sets multiple fields from a map
func (q *RelateQuery) SetMap(fields map[string]any) *RelateQuery {
	q.addSetMap(fields)
	return q
}

// Return sets the RETURN clause
func (q *RelateQuery) Return(clause string) *RelateQuery {
	q.returnClause = clause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *RelateQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *RelateQuery) String() string {
	// Don't escape record IDs with colons, only escape the edge table name
	sql := fmt.Sprintf("RELATE %s->%s->%s",
		q.from,
		escapeIdent(q.edge),
		q.to)

	if q.useContent && len(q.content) > 0 {
		paramName := q.generateParamName("content")
		q.addParam(paramName, q.content)
		sql += fmt.Sprintf(" CONTENT $%s", paramName)
	} else if setClause := q.buildSetClause(&q.baseQuery, ""); setClause != "" {
		sql += " SET " + setClause
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}
