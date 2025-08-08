package surrealql

import "fmt"

// UpdateQuery represents an UPDATE query
type UpdateQuery struct {
	baseQuery
	setsBuilder
	targets      []string
	whereClause  *whereBuilder
	returnClause string
}

// Update starts an UPDATE query
func Update[T mutationTarget](target T, targets ...T) *UpdateQuery {
	q := &UpdateQuery{
		baseQuery:   newBaseQuery(),
		setsBuilder: newSetsBuilder(),
		targets:     nil,
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
	q.addSet(expr, args, &q.baseQuery, "param")
	return q
}

// SetMap sets multiple fields from a map
func (q *UpdateQuery) SetMap(fields map[string]any) *UpdateQuery {
	q.addSetMap(fields)
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

	if setClause := q.buildSetClause(&q.baseQuery, ""); setClause != "" {
		sql += " SET " + setClause
	}

	if q.whereClause != nil && q.whereClause.hasConditions() {
		sql += " WHERE " + q.whereClause.build()
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}
