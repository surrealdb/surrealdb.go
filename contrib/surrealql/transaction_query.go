package surrealql

import (
	"fmt"
	"strings"
)

// TransactionQuery represents a transaction query with BEGIN/COMMIT statements
type TransactionQuery struct {
	baseQuery
	statements []TransactionStatement
}

// TransactionStatement represents a statement that can be executed within a transaction
type TransactionStatement interface {
	build() string
}

// LetStatement represents a LET statement within a transaction
type LetStatement struct {
	variable string
	dataType string
	value    any
}

// IfStatement represents an IF statement within a transaction
type IfStatement struct {
	condition string
	thenBlock []TransactionStatement
	elseBlock []TransactionStatement
}

// ThrowStatement represents a THROW statement within a transaction
type ThrowStatement struct {
	err any
}

// RawStatement represents a raw SurrealQL statement within a transaction
type RawStatement struct {
	sql string
}

// QueryStatement wraps any Query to be used within a transaction
type QueryStatement struct {
	query Query
}

// Let adds a LET statement to the transaction
func (t *TransactionQuery) Let(variable string, value any) *TransactionQuery {
	if !strings.HasPrefix(variable, "$") {
		variable = "$" + variable
	}
	t.statements = append(t.statements, &LetStatement{
		variable: variable,
		value:    value,
	})
	return t
}

// LetTyped adds a typed LET statement to the transaction
func (t *TransactionQuery) LetTyped(variable, dataType string, value any) *TransactionQuery {
	if !strings.HasPrefix(variable, "$") {
		variable = "$" + variable
	}
	t.statements = append(t.statements, &LetStatement{
		variable: variable,
		dataType: dataType,
		value:    value,
	})
	return t
}

// If adds an IF statement to the transaction
func (t *TransactionQuery) If(condition string) *IfBuilder {
	ifStmt := &IfStatement{
		condition: condition,
	}
	return &IfBuilder{
		transaction: t,
		ifStatement: ifStmt,
	}
}

// Throw adds a THROW statement to the transaction
func (t *TransactionQuery) Throw(err any) *TransactionQuery {
	t.statements = append(t.statements, &ThrowStatement{
		err: err,
	})
	return t
}

// Raw adds a raw SurrealQL statement to the transaction
func (t *TransactionQuery) Raw(sql string) *TransactionQuery {
	t.statements = append(t.statements, &RawStatement{
		sql: sql,
	})
	return t
}

// Query adds any Query to the transaction
func (t *TransactionQuery) Query(query Query) *TransactionQuery {
	t.statements = append(t.statements, &QueryStatement{
		query: query,
	})
	return t
}

// Build returns the SurrealQL string and parameters for the transaction
func (t *TransactionQuery) Build() (sql string, vars map[string]any) {
	var builder strings.Builder

	builder.WriteString("BEGIN TRANSACTION;\n")

	for _, stmt := range t.statements {
		builder.WriteString(stmt.build())
		builder.WriteString(";\n")

		// Merge parameters if this is a QueryStatement
		if qs, ok := stmt.(*QueryStatement); ok {
			_, vars := qs.query.Build()
			for k, v := range vars {
				t.vars[k] = v
			}
		}
	}

	builder.WriteString("COMMIT TRANSACTION;")

	return builder.String(), t.vars
}

// String returns the SurrealQL string for the transaction
func (t *TransactionQuery) String() string {
	sql, _ := t.Build()
	return sql
}

// IfBuilder helps build IF statements
type IfBuilder struct {
	transaction *TransactionQuery
	ifStatement *IfStatement
}

// Then adds statements to the THEN block
func (ib *IfBuilder) Then(fn func(*ThenBuilder)) *IfBuilder {
	tb := &ThenBuilder{
		statements: &ib.ifStatement.thenBlock,
	}
	fn(tb)
	return ib
}

// Else adds statements to the ELSE block
func (ib *IfBuilder) Else(fn func(*ElseBuilder)) *TransactionQuery {
	eb := &ElseBuilder{
		statements: &ib.ifStatement.elseBlock,
	}
	fn(eb)
	ib.transaction.statements = append(ib.transaction.statements, ib.ifStatement)
	return ib.transaction
}

// End completes the IF statement without an ELSE block
func (ib *IfBuilder) End() *TransactionQuery {
	ib.transaction.statements = append(ib.transaction.statements, ib.ifStatement)
	return ib.transaction
}

// ThenBuilder helps build the THEN block of an IF statement
type ThenBuilder struct {
	statements *[]TransactionStatement
}

// Throw adds a THROW statement to the THEN block
func (tb *ThenBuilder) Throw(err any) *ThenBuilder {
	*tb.statements = append(*tb.statements, &ThrowStatement{
		err: err,
	})
	return tb
}

// Raw adds a raw SurrealQL statement to the THEN block
func (tb *ThenBuilder) Raw(sql string) *ThenBuilder {
	*tb.statements = append(*tb.statements, &RawStatement{
		sql: sql,
	})
	return tb
}

// ElseBuilder helps build the ELSE block of an IF statement
type ElseBuilder struct {
	statements *[]TransactionStatement
}

// Throw adds a THROW statement to the ELSE block
func (eb *ElseBuilder) Throw(err any) *ElseBuilder {
	*eb.statements = append(*eb.statements, &ThrowStatement{
		err: err,
	})
	return eb
}

// Raw adds a raw SurrealQL statement to the ELSE block
func (eb *ElseBuilder) Raw(sql string) *ElseBuilder {
	*eb.statements = append(*eb.statements, &RawStatement{
		sql: sql,
	})
	return eb
}

// Implementation of build methods for each statement type

func (l *LetStatement) build() string {
	var builder strings.Builder
	builder.WriteString("LET ")
	builder.WriteString(l.variable)

	if l.dataType != "" {
		builder.WriteString(": ")
		builder.WriteString(l.dataType)
	}

	builder.WriteString(" = ")

	switch v := l.value.(type) {
	case string:
		builder.WriteString(fmt.Sprintf("%q", v))
	case Query:
		sql, _ := v.Build()
		builder.WriteString("(")
		builder.WriteString(sql)
		builder.WriteString(")")
	default:
		builder.WriteString(fmt.Sprintf("%v", v))
	}

	return builder.String()
}

func (i *IfStatement) build() string {
	var builder strings.Builder
	builder.WriteString("IF ")
	builder.WriteString(i.condition)
	builder.WriteString(" {\n")

	for _, stmt := range i.thenBlock {
		builder.WriteString("    ")
		builder.WriteString(stmt.build())
		builder.WriteString(";\n")
	}

	builder.WriteString("}")

	if len(i.elseBlock) > 0 {
		builder.WriteString(" ELSE {\n")
		for _, stmt := range i.elseBlock {
			builder.WriteString("    ")
			builder.WriteString(stmt.build())
			builder.WriteString(";\n")
		}
		builder.WriteString("}")
	}

	return builder.String()
}

func (t *ThrowStatement) build() string {
	switch v := t.err.(type) {
	case string:
		return fmt.Sprintf("THROW %q", v)
	default:
		return fmt.Sprintf("THROW %v", v)
	}
}

func (r *RawStatement) build() string {
	return strings.TrimRight(r.sql, ";")
}

func (q *QueryStatement) build() string {
	sql, _ := q.query.Build()
	return strings.TrimRight(sql, ";")
}
