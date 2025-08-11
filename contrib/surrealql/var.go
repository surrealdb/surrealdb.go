package surrealql

import "strings"

// Var represents a variable reference in SurrealQL
type Var string

// String returns the variable reference as a string
func (v Var) String() string {
	if strings.HasPrefix(string(v), "$") {
		return string(v)
	}
	return "$" + string(v)
}

// Name returns the variable name without the $ prefix
func (v Var) Name() string {
	return strings.TrimPrefix(string(v), "$")
}
