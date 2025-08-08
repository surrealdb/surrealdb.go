package surrealql

// DeleteQuery represents a DELETE query
type DeleteQuery struct {
	baseQuery
	targets      []string
	whereClause  *whereBuilder
	returnClause string
}

// Delete starts a DELETE query
func Delete[T mutationTarget](target T, targets ...T) *DeleteQuery {
	q := &DeleteQuery{
		baseQuery: newBaseQuery(),
		targets:   nil,
	}
	deleteAddTarget(q, target)
	for _, t := range targets {
		deleteAddTarget(q, t)
	}
	return q
}

func deleteAddTarget[MT mutationTarget](q *DeleteQuery, target MT) *DeleteQuery {
	sql, vars := buildTargetExpr(target)
	q.targets = append(q.targets, sql)
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// Where adds a WHERE condition
func (q *DeleteQuery) Where(condition string, args ...any) *DeleteQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *DeleteQuery) Return(clause string) *DeleteQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *DeleteQuery) ReturnNone() *DeleteQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *DeleteQuery) Build() (sql string, params map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *DeleteQuery) String() string {
	sql := "DELETE "

	for i, target := range q.targets {
		if i > 0 {
			sql += ", "
		}
		sql += target
	}

	if q.whereClause != nil && q.whereClause.hasConditions() {
		sql += " WHERE " + q.whereClause.build()
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}
