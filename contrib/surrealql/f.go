package surrealql

import (
	"fmt"
)

func F[T selectField](f T) *field {
	switch v := any(f).(type) {
	case *field:
		return v
	default:
		return &field{expr: f, alias: ""}
	}
}

// selectField is an interface for fields in SELECT queries.
// This should be used solely for type safety in the SelectQuery.
type selectField interface {
	*field | fieldType
}

// fieldType is an interface for fields in SELECT queries.
//
// Select can be done with fields, or aliased fields,
// where each field can be a simple escaped field,
// a function call, or another SelectQuery.
type fieldType interface {
	string | *SelectQuery | *FunCall
}

// buildSelectFieldExpr builds the SQL expression for a select field.
// The f MUST be any of the types defined in fieldType.
//
// It panics if the type is unsupported, but it should happen only when
// this library has a bug, because this function is private to prevent misuse.
func buildSelectFieldExpr(f any) (sql string, vars map[string]any) {
	if f == nil {
		return "*", nil // Default to selecting all fields
	}

	switch v := f.(type) {
	case string:
		return v, nil
	case *SelectQuery:
		sql, vars := v.Build()
		return fmt.Sprintf("(%s)", sql), vars
	case *FunCall:
		return v.Build()
	default:
		panic(fmt.Sprintf("unsupported select field type: %T", f))
	}
}

// field represents a field in a SELECT query.
// It can be a simple field, a function call, or an expression, with an optional alias.
type field struct {
	expr  any
	alias string
}

// As adds an alias to the function call.
// This is useful when using the function in a SELECT query.
// It returns an Aliased[*FunCall] which can be used in SELECT queries.
func (f *field) As(alias string) *field {
	return &field{expr: f.expr, alias: alias}
}

// Build returns the SurrealQL expression for the field and any associated vars.
func (f *field) Build() (sql string, vars map[string]any) {
	innerQL, innerVars := buildSelectFieldExpr(f.expr)

	if f.alias != "" {
		return fmt.Sprintf("%s AS %s", innerQL, escapeIdent(f.alias)), innerVars
	}
	return innerQL, innerVars
}
