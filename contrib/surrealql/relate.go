package surrealql

import "fmt"

// RelateQuery represents a RELATE query
type RelateQuery struct {
	baseQuery
	from         string
	edge         string
	to           string
	content      map[string]any
	returnClause string
}

// Relate starts a RELATE query
func Relate(from, edge, to string) *RelateQuery {
	return &RelateQuery{
		baseQuery: newBaseQuery(),
		from:      from,
		edge:      edge,
		to:        to,
		content:   make(map[string]any),
	}
}

// Set adds a field to the relation
func (q *RelateQuery) Set(field string, value any) *RelateQuery {
	q.content[field] = value
	return q
}

// Content sets the entire content for the relation
func (q *RelateQuery) Content(content map[string]any) *RelateQuery {
	q.content = content
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

	if len(q.content) > 0 {
		paramName := q.generateParamName("content")
		q.addParam(paramName, q.content)
		sql += fmt.Sprintf(" CONTENT $%s", paramName)
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}
