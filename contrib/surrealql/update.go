package surrealql

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// UpdateQuery represents an UPDATE query
type UpdateQuery struct {
	baseQuery
	targets      []string
	sets         map[string]any
	setsRaw      []string
	whereClause  *whereBuilder
	returnClause string
}

// Update starts an UPDATE query
func Update[T mutationTarget](target T, targets ...T) *UpdateQuery {
	q := &UpdateQuery{
		baseQuery: newBaseQuery(),
		targets:   nil,
		sets:      make(map[string]any),
	}

	updateAddTarget(q, target)
	for _, t := range targets {
		updateAddTarget(q, t)
	}

	return q
}

func updateAddTarget[MT mutationTarget](q *UpdateQuery, target MT) *UpdateQuery {
	sql, vars := buildTargetExpr(target)
	q.targets = append(q.targets, sql)
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// Set adds a field or expression to update
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *UpdateQuery) Set(expr string, args ...any) *UpdateQuery {
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
func (q *UpdateQuery) SetMap(fields map[string]any) *UpdateQuery {
	maps.Copy(q.sets, fields)
	return q
}

// Where adds a WHERE condition
func (q *UpdateQuery) Where(condition string, args ...any) *UpdateQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *UpdateQuery) Return(clause string) *UpdateQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *UpdateQuery) ReturnNone() *UpdateQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *UpdateQuery) ReturnDiff() *UpdateQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *UpdateQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *UpdateQuery) String() string {
	sql := ""

	for _, t := range q.targets {
		if sql != "" {
			sql += ", "
		}
		sql += t
	}

	sql = fmt.Sprintf("UPDATE %s", sql)

	if len(q.sets) > 0 || len(q.setsRaw) > 0 {
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

	if q.whereClause != nil && q.whereClause.hasConditions() {
		sql += " WHERE " + q.whereClause.build()
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}
