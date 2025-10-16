package surrealql

import (
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Expr creates an expression with optional placeholders for use in various queries.
//
// Expr can be used for fields and targets for various queries including SELECT, CREATE, UPSERT, and so on.
//
// The returned value supports the As() method for aliasing.
//
// It can be used in multiple ways:
//   - Expr("count(orders)").As("total") - simple function call with alias
//   - Expr("math::mean([?,?,?])", 1, 2, 3).As("avg") - function with value arguments
//   - Expr("math::sum(?)", subQuery) - function with subquery
//   - Expr("? + ?", V("a"), 10).As("calc") - expression with variable and value
func Expr[T exprLike](raw T, args ...any) *expr {
	if len(args) == 0 {
		// No placeholders, just return the raw expression
		return &expr{expr: raw, alias: ""}
	}

	if v, ok := any(raw).(*expr); ok {
		return v
	}

	// Create a parameterized field
	return &expr{
		expr:     raw,
		alias:    "",
		args:     args,
		isRawSQL: true,
	}
}

// exprLike is an interface for types that can be
// seen as expressions in SurrealQL.
//
// This should be used solely for type safety in
// query targets and fields.
type exprLike interface {
	*expr | string | *SelectQuery | models.Table | *models.RecordID | models.RecordID
}

// buildExprLike builds the SQL expression for a select field.
// The ex MUST be any of the types defined in fieldType.
//
// It panics if the type is unsupported, but it should happen only when
// this library has a bug, because this function is private to prevent misuse.
func buildExprLike(c *queryBuildContext, b *strings.Builder, ex any, args []any) error {
	if ex == nil {
		// Default to selecting all fields
		b.WriteString("*")
		return nil
	}

	switch v := ex.(type) {
	case string:
		if len(args) > 0 {
			b.WriteString("<invalid expr with string>")
			return fmt.Errorf("invalid expr with string %q: <args> not allowed", v)
		}
		b.WriteString(v)
		return nil
	case *SelectQuery:
		if len(args) > 0 {
			b.WriteString("<invalid expr with SelectQuery>")
			return fmt.Errorf("invalid expr with SelectQuery: <args> not allowed")
		}
		b.WriteString("(")
		v.build(c, b)
		b.WriteString(")")
		return nil
	case *expr:
		if len(args) > 0 {
			b.WriteString("<invalid expr with expr>")
			return fmt.Errorf("invalid expr with expr: <args> not allowed")
		}
		v.build(c, b)
		return nil
	case models.Table:
		name := c.generateAndAddParam("table", v)
		b.WriteString("$")
		b.WriteString(name)
		return nil
	case *models.RecordID:
		name := c.generateAndAddParam("id", v)
		b.WriteString("$")
		b.WriteString(name)
		return nil
	case models.RecordID:
		name := c.generateAndAddParam("id", v)
		b.WriteString("$")
		b.WriteString(name)
		return nil
	default:
		panic(fmt.Sprintf("unsupported select field type: %T", ex))
	}
}

// expr represents a expr in a SELECT query.
// It can be a simple expr, a function call, or an expression, with an optional alias.
type expr struct {
	expr     any
	alias    string
	args     []any
	isRawSQL bool
	paramIdx int // Used for generating unique parameter names
}

// As adds an alias to the function call.
// This is useful when using the function in a SELECT query.
// It returns an Aliased[*FunCall] which can be used in SELECT queries.
func (f *expr) As(alias string) *expr {
	return &expr{
		expr:     f.expr,
		alias:    alias,
		args:     f.args,
		isRawSQL: f.isRawSQL,
		paramIdx: f.paramIdx,
	}
}

// Build returns the SurrealQL expression and any associated vars
// that can be used with the surrealdb.Query function.
func (f *expr) Build() (sql string, vars map[string]any) {
	c := newQueryBuildContext()
	var b strings.Builder
	f.build(&c, &b)
	return b.String(), c.vars
}

func buildQueryStringWithPlaceholders(c *queryBuildContext, b *strings.Builder, prefix, snippet string, args []any) {
	var startIndex int
	for _, arg := range args {
		placeholder := strings.Index(snippet[startIndex:], "?")
		if placeholder < 0 {
			break
		}
		placeholder += startIndex
		b.WriteString(snippet[startIndex:placeholder])

		switch v := arg.(type) {
		case Var:
			// Variable reference - replace ? with the variable
			b.WriteString(v.String())
		case Query:
			// Subquery - replace ? with the subquery
			b.WriteString("(")
			v.build(c, b)
			b.WriteString(")")
		default:
			// Regular value - create a parameter
			paramName := c.generateAndAddParam(prefix, v)
			b.WriteString("$")
			b.WriteString(paramName)
		}

		// Update the start index
		startIndex = placeholder + 1
	}

	b.WriteString(snippet[startIndex:])
}

// build returns the SurrealQL expression for the field and any associated vars.
func (f *expr) build(c *queryBuildContext, b *strings.Builder) {
	if f.isRawSQL {
		// Handle raw SQL with placeholders
		queryWithPlaceholders := f.expr.(string)

		buildQueryStringWithPlaceholders(c, b, "param", queryWithPlaceholders, f.args)
	} else {
		if validationErr := buildExprLike(c, b, f.expr, f.args); validationErr != nil {
			panic(validationErr)
		}
	}

	if f.alias != "" {
		b.WriteString(" AS ")
		b.WriteString(escapeIdent(f.alias))
	}
}

// String returns the SurrealQL string
func (q *expr) String() string {
	sql, _ := q.Build()
	return sql
}
