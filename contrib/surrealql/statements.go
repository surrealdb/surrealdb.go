package surrealql

import (
	"fmt"
	"strings"
)

// LetStatement represents a LET statement within a transaction
type LetStatement struct {
	variable string
	dataType string
	value    any
}

// IfStatement represents an IF statement within a transaction
type IfStatement struct {
	condition string
	thenBlock []Statement
	elseBlock []Statement
}

// ThrowStatement represents a THROW statement within a transaction
type ThrowStatement struct {
	err any
}

// ReturnStatement represents a RETURN statement within a transaction
type ReturnStatement struct {
	expr string
	args []any
}

// IfBuilder helps build IF statements
type IfBuilder[T any] struct {
	transaction *T
	ifStatement *IfStatement
}

// Then adds statements to the THEN block
func (ib *IfBuilder[T]) Then(fn func(*ThenBuilder)) *IfBuilder[T] {
	tb := &ThenBuilder{}
	tb.StatementsBuilder = &StatementsBuilder[ThenBuilder]{
		self: tb,
	}
	fn(tb)
	ib.ifStatement.thenBlock = tb.statements
	return ib
}

// Else adds statements to the ELSE block
func (ib *IfBuilder[T]) Else(fn func(*ElseBuilder)) *IfBuilder[T] {
	eb := &ElseBuilder{}
	eb.StatementsBuilder = &StatementsBuilder[ElseBuilder]{
		self: eb,
	}
	fn(eb)
	ib.ifStatement.elseBlock = eb.statements
	return ib
}

// End completes the IF statement without an ELSE block
func (ib *IfBuilder[T]) End() *T {
	return ib.transaction
}

// ThenBuilder helps build the THEN block of an IF statement
type ThenBuilder struct {
	*StatementsBuilder[ThenBuilder]
}

// ElseBuilder helps build the ELSE block of an IF statement
type ElseBuilder struct {
	*StatementsBuilder[ElseBuilder]
}

// Implementation of build methods for each statement type

func (l *LetStatement) build(c *queryBuildContext, builder *strings.Builder) {
	builder.WriteString("LET ")
	builder.WriteString(l.variable)

	if l.dataType != "" {
		builder.WriteString(": ")
		builder.WriteString(l.dataType)
	}

	builder.WriteString(" = ")

	switch v := l.value.(type) {
	case string:
		fmt.Fprintf(builder, "%q", v)
	case Query:
		builder.WriteString("(")
		v.build(c, builder)
		builder.WriteString(")")
	default:
		fmt.Fprintf(builder, "%v", v)
	}
}

func (i *IfStatement) build(c *queryBuildContext, builder *strings.Builder) {
	builder.WriteString("IF ")
	builder.WriteString(i.condition)
	builder.WriteString(" {\n")

	for _, stmt := range i.thenBlock {
		builder.WriteString("    ")
		stmt.build(c, builder)
		builder.WriteString(";\n")
	}

	builder.WriteString("}")

	if len(i.elseBlock) > 0 {
		builder.WriteString(" ELSE {\n")
		for _, stmt := range i.elseBlock {
			builder.WriteString("    ")
			stmt.build(c, builder)
			builder.WriteString(";\n")
		}
		builder.WriteString("}")
	}
}

func (t *ThrowStatement) build(c *queryBuildContext, b *strings.Builder) {
	b.WriteString("THROW ")
	switch v := t.err.(type) {
	case string:
		fmt.Fprintf(b, "%q", v)
	default:
		fmt.Fprintf(b, "%v", v)
	}
}

func (r *ReturnStatement) build(c *queryBuildContext, b *strings.Builder) {
	b.WriteString("RETURN ")

	if len(r.args) == 0 {
		// No placeholders, just return the raw expression
		b.WriteString(r.expr)
		return
	}

	// Process placeholders
	var startIndex int
	for _, arg := range r.args {
		placeholder := strings.Index(r.expr[startIndex:], "?")
		if placeholder < 0 {
			break
		}
		placeholder += startIndex
		b.WriteString(r.expr[startIndex:placeholder])
		// Check if arg is a Var (variable reference)
		if varRef, ok := arg.(Var); ok {
			// Replace the first ? with the variable reference
			b.WriteString(varRef.String())
		} else {
			// Regular value, create a parameter
			paramName := c.generateAndAddParam("return_param", arg)
			b.WriteString("$")
			b.WriteString(paramName)
		}
		// Update the start index
		startIndex = placeholder + 1
	}
	b.WriteString(r.expr[startIndex:])
}
