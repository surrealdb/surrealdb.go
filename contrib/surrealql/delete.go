package surrealql

import "strings"

// DeleteQuery represents a DELETE query
type DeleteQuery struct {
	targets      []*expr
	whereClause  *whereBuilder
	returnClause string
	only         bool
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

// DeleteOnly starts a DELETE ONLY query that deletes and returns only one record
//
// Note that DELETE ONLY requires either ReturnBefore or ReturnAfter by its nature.
// The standard DELETE returns an empty array by default so adding ONLY to it always fails
// because there is no single record to return.
//
// Refer to [SurrealDB documentation] for details.
//
// [SurrealDB documentation]: https://surrealdb.com/docs/surrealql/statements/delete#basic-usage
func DeleteOnly[T exprLike](target T) *DeleteQuery {
	q := Delete(target)
	q.only = true
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

// ReturnBefore sets RETURN BEFORE
func (q *DeleteQuery) ReturnBefore() *DeleteQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *DeleteQuery) ReturnAfter() *DeleteQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *DeleteQuery) ReturnDiff() *DeleteQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *DeleteQuery) Build() (sql string, params map[string]any) {
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *DeleteQuery) build(c *queryBuildContext) (sql string) {
	var b strings.Builder

	b.WriteString("DELETE ")

	if q.only {
		b.WriteString("ONLY ")
	}

	for i, target := range q.targets {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(target.Build(c))
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
func (q *DeleteQuery) String() string {
	sql, _ := q.Build()
	return sql
}
