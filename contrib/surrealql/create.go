package surrealql

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// CreateQuery represents a CREATE query
type CreateQuery struct {
	baseQuery
	thing        string
	sets         map[string]any
	setsRaw      []string
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
func Create[T mutationTarget](thing T) *CreateQuery {
	bq := newBaseQuery()
	sql, vars := buildTargetExpr(thing)
	for k, v := range vars {
		bq.addParam(k, v)
	}
	return &CreateQuery{
		baseQuery: bq,
		thing:     sql,
		sets:      make(map[string]any),
		content:   make(map[string]any),
	}
}

// Set adds a field or expression to set in the CREATE query
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *CreateQuery) Set(expr string, args ...any) *CreateQuery {
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

// SetMap sets multiple fields from a map
func (q *CreateQuery) SetMap(fields map[string]any) *CreateQuery {
	maps.Copy(q.sets, fields)
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
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *CreateQuery) String() string {
	sql := fmt.Sprintf("CREATE %s", q.thing)

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
