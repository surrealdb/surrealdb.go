package surrealql

import "strings"

// Var represents a variable reference in SurrealQL
type Var struct {
	name string
}

// NewVar creates a new variable reference
// The name can be provided with or without the $ prefix
func NewVar(name string) Var {
	if !strings.HasPrefix(name, "$") {
		name = "$" + name
	}
	return Var{name: name}
}

// Var is a convenience function for creating variable references
// Example: Var("name") creates a reference to $name
func V(name string) Var {
	return NewVar(name)
}

// String returns the variable reference as a string
func (v Var) String() string {
	return v.name
}

// Name returns the variable name without the $ prefix
func (v Var) Name() string {
	return strings.TrimPrefix(v.name, "$")
}
