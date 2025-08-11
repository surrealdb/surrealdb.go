package surrealql

// DeleteQuery represents a DELETE query
type DeleteQuery struct {
	targets      []*expr
	whereClause  *whereBuilder
	returnClause string
}

// Delete starts a DELETE query
func Delete[T exprLike](target T, targets ...T) *DeleteQuery {
	q := &DeleteQuery{
		targets: nil,
	}
	q.targets = append(q.targets, Expr(target))
	for _, t := range targets {
		q.targets = append(q.targets, Expr(t))
	}
	return q
}

// Where adds a WHERE condition
func (q *DeleteQuery) Where(condition string, args ...any) *DeleteQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args)
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
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *DeleteQuery) build(c *queryBuildContext) (sql string) {
	sql = "DELETE "

	for i, target := range q.targets {
		if i > 0 {
			sql += ", "
		}
		tSQL := target.Build(c)
		sql += tSQL
	}

	if q.whereClause != nil && q.whereClause.hasConditions() {
		sql += " WHERE " + q.whereClause.build(c)
	}

	if q.returnClause != "" {
		sql += " RETURN " + q.returnClause
	}

	return sql
}

// String returns the SurrealQL string
func (q *DeleteQuery) String() string {
	sql, _ := q.Build()
	return sql
}
