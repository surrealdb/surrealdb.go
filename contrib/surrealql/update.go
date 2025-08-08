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

// Set adds a field to update
func (q *UpdateQuery) Set(field string, value any) *UpdateQuery {
	q.sets[field] = value
	return q
}

// SetMap sets multiple fields from a map
func (q *UpdateQuery) SetMap(fields map[string]any) *UpdateQuery {
	for k, v := range fields {
		q.sets[k] = v
	}
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

	if len(q.sets) > 0 {
		setsKeys := sort.StringSlice(slices.Collect(maps.Keys(q.sets)))
		sort.Stable(setsKeys)

		var setParts []string
		for _, field := range setsKeys {
			value := q.sets[field]
			paramName := q.generateParamName(field)
			q.addParam(paramName, value)
			setParts = append(setParts, fmt.Sprintf("%s = $%s", escapeIdent(field), paramName))
		}
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
