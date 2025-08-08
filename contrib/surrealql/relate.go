package surrealql

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// RelateQuery represents a RELATE query
type RelateQuery struct {
	baseQuery
	from         string
	edge         string
	to           string
	sets         map[string]any
	setsRaw      []string
	content      map[string]any
	useContent   bool
	returnClause string
}

// Relate starts a RELATE query
func Relate(from, edge, to string) *RelateQuery {
	return &RelateQuery{
		baseQuery: newBaseQuery(),
		from:      from,
		edge:      edge,
		to:        to,
		sets:      make(map[string]any),
		content:   make(map[string]any),
	}
}

// Set adds a field or expression to the relation
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *RelateQuery) Set(expr string, args ...any) *RelateQuery {
	// Check if this is a simple field assignment or an expression
	if len(args) == 1 && !strings.ContainsAny(expr, "?+=<>!-*/") {
		// Simple field assignment
		q.sets[expr] = args[0]
	} else if len(args) > 0 {
		// Expression with placeholders
		processedExpr := expr
		for _, arg := range args {
			paramName := q.generateParamName("param")
			processedExpr = strings.Replace(processedExpr, "?", "$"+paramName, 1)
			q.addParam(paramName, arg)
		}
		q.setsRaw = append(q.setsRaw, processedExpr)
	} else {
		// Raw expression without placeholders
		q.setsRaw = append(q.setsRaw, expr)
	}
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
	maps.Copy(q.sets, fields)
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
	} else if len(q.sets) > 0 || len(q.setsRaw) > 0 {
		var setParts []string

		// Handle SET fields
		if len(q.sets) > 0 {
			setsKeys := sort.StringSlice(slices.Collect(maps.Keys(q.sets)))
			sort.Stable(setsKeys)

			for _, field := range setsKeys {
				value := q.sets[field]
				paramName := q.generateParamName(field)
				q.addParam(paramName, value)
				setParts = append(setParts, fmt.Sprintf("%s = $%s", escapeIdent(field), paramName))
			}
		}

		// Handle raw SET expressions
		setParts = append(setParts, q.setsRaw...)

		sql += " SET " + strings.Join(setParts, ", ")
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}
