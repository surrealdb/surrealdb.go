package surrealql

import "strings"

// UpdateQuery represents an UPDATE query
type UpdateQuery struct {
	setsBuilder
	targets      []*expr
	whereClause  *whereBuilder
	returnClause string
	only         bool
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

// UpdateOnly starts an UPDATE ONLY query that updates and returns only one record
func UpdateOnly[T exprLike](target T) *UpdateQuery {
	q := Update(target)
	q.only = true

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
	var b strings.Builder

	b.WriteString("UPDATE ")

	if q.only {
		b.WriteString("ONLY ")
	}

	for i, t := range q.targets {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(t.build(c))
	}

	if setClause := q.buildSetClause(c); setClause != "" {
		b.WriteString(" SET ")
		b.WriteString(setClause)
	}

	if q.whereClause != nil && q.whereClause.hasConditions() {
		b.WriteString(" WHERE ")
		b.WriteString(q.whereClause.build(c))
	}

	if q.returnClause != "" {
		b.WriteString(" RETURN ")
		b.WriteString(q.returnClause)
	}

	return b.String()
}

// String returns the SurrealQL string
func (q *UpdateQuery) String() string {
	sql, _ := q.Build()
	return sql
}
