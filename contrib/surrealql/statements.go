package surrealql

import (
	"strings"
)

// StatementsBuilder provides common functionality for building a series of statements
//
// T is the type of the struct embedding this builder, allowing method chaining to return the correct type.
type StatementsBuilder[T any] struct {
	statements []TransactionStatement

	// self is a reference to the struct embedding this builder, allowing method chaining to return the correct type and avoid type assertions.
	// It must be set by the embedding struct after initialization.
	self *T
}

// Let adds a LET statement to the transaction
func (t *StatementsBuilder[T]) Let(variable string, value any) *T {
	if !strings.HasPrefix(variable, "$") {
		variable = "$" + variable
	}
	t.statements = append(t.statements, &LetStatement{
		variable: variable,
		value:    value,
	})
	return t.self
}

// LetTyped adds a typed LET statement to the transaction
func (t *StatementsBuilder[T]) LetTyped(variable, dataType string, value any) *T {
	if !strings.HasPrefix(variable, "$") {
		variable = "$" + variable
	}
	t.statements = append(t.statements, &LetStatement{
		variable: variable,
		dataType: dataType,
		value:    value,
	})
	return t.self
}

// If adds an IF statement to the transaction
func (t *StatementsBuilder[T]) If(condition string) *IfBuilder[T] {
	ifStmt := &IfStatement{
		condition: condition,
	}
	b := &IfBuilder[T]{
		transaction: t.self,
		ifStatement: ifStmt,
	}
	t.statements = append(t.statements, ifStmt)
	return b
}

// Throw adds a THROW statement to the transaction
func (t *StatementsBuilder[T]) Throw(err any) *T {
	t.statements = append(t.statements, &ThrowStatement{
		err: err,
	})
	return t.self
}

// Raw adds a raw SurrealQL statement to the transaction
func (t *StatementsBuilder[T]) Raw(sql string) *T {
	t.statements = append(t.statements, &RawStatement{
		sql: sql,
	})
	return t.self
}

// Query adds any Query to the transaction
func (t *StatementsBuilder[T]) Query(query Query) *T {
	t.statements = append(t.statements, &QueryStatement{
		query: query,
	})
	return t.self
}

// Return adds a RETURN statement to the transaction
// The expr parameter is raw SQL that can contain placeholders (?)
// Args are the values to substitute for the placeholders
func (t *StatementsBuilder[T]) Return(expr string, args ...any) *T {
	t.statements = append(t.statements, &ReturnStatement{
		expr: expr,
		args: args,
	})
	return t.self
}

// Build returns the SurrealQL string and parameters for the transaction
func (t *StatementsBuilder[T]) build(c *queryBuildContext, builder *strings.Builder) {
	for _, stmt := range t.statements {
		stmt.build(c, builder)
		builder.WriteString(";\n")
	}
}
