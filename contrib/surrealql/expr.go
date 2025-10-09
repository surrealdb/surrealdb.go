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
func buildExprLike(c *queryBuildContext, ex any, args []any) (sql string, validationErr error) {
	if ex == nil {
		return "*", nil // Default to selecting all fields
	}

	switch v := ex.(type) {
	case string:
		if len(args) > 0 {
			return "<invalid expr with string>", fmt.Errorf("invalid expr with string %q: <args> not allowed", v)
		}
		return v, nil
	case *SelectQuery:
		if len(args) > 0 {
			return "<invalid expr with SelectQuery>", fmt.Errorf("invalid expr with SelectQuery: <args> not allowed")
		}
		sql := v.build(c)
		return fmt.Sprintf("(%s)", sql), nil
	case *expr:
		if len(args) > 0 {
			return "<invalid expr with expr>", fmt.Errorf("invalid expr with expr: <args> not allowed")
		}

		sql := v.build(c)
		return sql, nil
	case models.Table:
		name := c.generateAndAddParam("table", v)
		return "$" + name, nil
	case *models.RecordID:
		name := c.generateAndAddParam("id", v)
		return "$" + name, nil
	case models.RecordID:
		name := c.generateAndAddParam("id", v)
		return "$" + name, nil
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
	return f.build(&c), c.vars
}

// build returns the SurrealQL expression for the field and any associated vars.
func (f *expr) build(c *queryBuildContext) (sql string) {
	if f.isRawSQL {
		// Handle raw SQL with placeholders
		processedExpr := f.expr.(string)

		for _, arg := range f.args {
			// Check the type of argument
			switch v := arg.(type) {
			case Var:
				// Variable reference - replace ? with the variable
				processedExpr = strings.Replace(processedExpr, "?", v.String(), 1)
			case Query:
				// Subquery - replace ? with the subquery
				subSQL := v.build(c)
				processedExpr = strings.Replace(processedExpr, "?", fmt.Sprintf("(%s)", subSQL), 1)
			default:
				// Regular value - create a parameter
				paramName := c.generateAndAddParam("param", v)
				processedExpr = strings.Replace(processedExpr, "?", "$"+paramName, 1)
			}
		}

		if f.alias != "" {
			return fmt.Sprintf("%s AS %s", processedExpr, escapeIdent(f.alias))
		}
		return processedExpr
	}

	innerQL, validationErr := buildExprLike(c, f.expr, f.args)
	if validationErr != nil {
		panic(validationErr)
	}

	if f.alias != "" {
		return fmt.Sprintf("%s AS %s", innerQL, escapeIdent(f.alias))
	}
	return innerQL
}

// String returns the SurrealQL string
func (q *expr) String() string {
	sql, _ := q.Build()
	return sql
}
