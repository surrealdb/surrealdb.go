package surrealql

import (
	"strings"
)

// TransactionQuery represents a transaction query with BEGIN/COMMIT statements
type TransactionQuery struct {
	*StatementsBuilder[TransactionQuery]
}

// Build returns the SurrealQL string and parameters for the transaction
func (t *TransactionQuery) Build() (sql string, vars map[string]any) {
	c := newQueryBuildContext()

	var builder strings.Builder

	builder.WriteString("BEGIN TRANSACTION;\n")

	t.build(&c, &builder)

	builder.WriteString("COMMIT TRANSACTION;")

	return builder.String(), c.vars
}

// String returns the SurrealQL string for the transaction
func (t *TransactionQuery) String() string {
	sql, _ := t.Build()
	return sql
}
