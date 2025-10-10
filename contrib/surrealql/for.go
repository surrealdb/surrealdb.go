package surrealql

import "strings"

// For creates a new FOR statement, which iterates over an array or the results of a subquery.
// The item parameter is the loop variable name, and iterable can be an array or a subquery.
// Additional arguments can be provided for parameterized subqueries.
func For[T exprLike](item string, iterableExpr T, iterableArgs ...any) *ForStatement {
	s := &ForStatement{
		item:     strings.TrimPrefix(item, "$"),
		iterable: Expr(iterableExpr, iterableArgs...),
	}

	s.StatementsBuilder = &StatementsBuilder[ForStatement]{
		self: s,
	}

	return s
}

type ForStatement struct {
	item     string
	iterable *expr

	*StatementsBuilder[ForStatement]
}

func (f *ForStatement) Build() (sql string, vars map[string]any) {
	c := newQueryBuildContext()

	var builder strings.Builder

	f.build(&c, &builder)

	return builder.String(), c.vars
}

func (f *ForStatement) build(c *queryBuildContext, builder *strings.Builder) {
	builder.WriteString("FOR $")
	builder.WriteString(f.item)
	builder.WriteString(" IN ")
	f.iterable.build(c, builder)
	builder.WriteString(" {\n")
	f.StatementsBuilder.build(c, builder)
	builder.WriteString("}")
}

// String returns the SurrealQL string
func (q *ForStatement) String() string {
	sql, _ := q.Build()
	return sql
}
