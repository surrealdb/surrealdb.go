package surrealql

import "fmt"

// UpdateQuery represents an UPDATE query
type UpdateQuery struct {
	setsBuilder
	targets      []*expr
	whereClause  *whereBuilder
	returnClause string
}

// Update starts an UPDATE query
func Update[T exprLike](targets ...T) *UpdateQuery {
	q := &UpdateQuery{
		setsBuilder: newSetsBuilder(),
		targets:     nil,
	}

	for _, t := range targets {
		q.targets = append(q.targets, Expr(t))
	}

	return q
}

// Set adds a field or expression to update
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *UpdateQuery) Set(expr string, args ...any) *UpdateQuery {
	q.addSet(expr, args)
	return q
}

// Where adds a WHERE condition
func (q *UpdateQuery) Where(condition string, args ...any) *UpdateQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args)
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
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *UpdateQuery) build(c *queryBuildContext) (sql string) {
	for _, t := range q.targets {
		if sql != "" {
			sql += ", "
		}

		tSQL := t.Build(c)

		sql += tSQL
	}

	sql = fmt.Sprintf("UPDATE %s", sql)

	if setClause := q.buildSetClause(c); setClause != "" {
		sql += " SET " + setClause
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
func (q *UpdateQuery) String() string {
	sql, _ := q.Build()
	return sql
}
