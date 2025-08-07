// Package surrealql provides a query builder for SurrealQL queries.
// It allows you to construct SurrealQL queries programmatically with type safety.
package surrealql

import (
	"fmt"
	"strings"
)

// Constants for common return clauses
const (
	ReturnNoneClause   = "NONE"
	ReturnDiffClause   = "DIFF"
	ReturnBeforeClause = "BEFORE"
	ReturnAfterClause  = "AFTER"
	StatusOK           = "OK"
)

// Query represents a SurrealQL query that can be built and executed.
type Query interface {
	// Build returns the SurrealQL string and parameters for the query
	Build() (string, map[string]any)
	// String returns the SurrealQL string for the query
	String() string
}

// baseQuery contains common fields for all query types
type baseQuery struct {
	vars map[string]any
}

// newBaseQuery creates a new base query
func newBaseQuery() baseQuery {
	return baseQuery{
		vars: make(map[string]any),
	}
}

// addParam adds a parameter to the query
func (q *baseQuery) addParam(name string, value any) {
	q.vars[name] = value
}

// generateParamName generates a unique parameter name
func (q *baseQuery) generateParamName(prefix string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s_%d", prefix, i)
		if _, exists := q.vars[name]; !exists {
			return name
		}
	}
}

// escapeIdent escapes an identifier for use in SurrealQL
func escapeIdent(ident string) string {
	// If the identifier contains special characters, wrap it in backticks
	if strings.ContainsAny(ident, " -:`") || isReservedWord(ident) {
		return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
	}
	return ident
}

// isReservedWord checks if a word is a SurrealQL reserved word
func isReservedWord(word string) bool {
	// This is a simplified check - in a real implementation,
	// you'd have a complete list of reserved words
	reserved := []string{
		"SELECT", "FROM", "WHERE", "ORDER", "BY", "LIMIT", "START",
		"FETCH", "GROUP", "SPLIT", "RETURN", "PARALLEL", "EXPLAIN",
		"CREATE", "UPDATE", "DELETE", "RELATE", "INSERT", "DEFINE",
		"REMOVE", "INFO", "USE", "BEGIN", "CANCEL", "COMMIT",
		"IF", "ELSE", "THEN", "END", "BREAK", "CONTINUE",
		"FUNCTION", "PARAM", "FIELD", "TYPE", "DEFAULT",
		"ASSERT", "PERMISSIONS", "DURATION", "FLEXIBLE",
	}

	upperWord := strings.ToUpper(word)
	for _, r := range reserved {
		if upperWord == r {
			return true
		}
	}
	return false
}
