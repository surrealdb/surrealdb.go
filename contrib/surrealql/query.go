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
	ExplainClause      = "EXPLAIN"
	ExplainFullClause  = "EXPLAIN FULL"
)

// Query represents a SurrealQL query that can be built and executed.
type Query interface {
	// Build returns the SurrealQL string and parameters for the query
	Build() (string, map[string]any)

	// build generates the SurrealQL string in the provided build context.
	// The build mutates the context, and the context is propagated across
	// multiple sub queries so that variables are unique.
	build(c *queryBuildContext, b *strings.Builder)

	// String returns the SurrealQL string for the query
	String() string
}

// queryBuildContext holds the context for building queries.
// It enables building a query with unique variable names.
type queryBuildContext struct {
	vars map[string]any

	ctx        string
	underlying *queryBuildContext
}

// newQueryBuildContext creates a new base query
func newQueryBuildContext() queryBuildContext {
	return queryBuildContext{
		vars: make(map[string]any),
	}
}

func (q *queryBuildContext) in(ctx string) *queryBuildContext {
	return &queryBuildContext{
		ctx:        ctx,
		underlying: q,
	}
}

// generateParamName generates a unique parameter name
func (q *queryBuildContext) generateParamName(prefix string) string {
	if q.underlying != nil {
		panic("unreachable")
	}

	for i := 1; ; i++ {
		name := fmt.Sprintf("%s_%d", prefix, i)
		if _, exists := q.vars[name]; !exists {
			return name
		}
	}
}

// generateAndAddParam generates a unique parameter name and adds it to the query context
func (q *queryBuildContext) generateAndAddParam(prefix string, value any) string {
	if q.underlying != nil {
		return q.underlying.generateAndAddParam(q.ctx+"_"+prefix, value)
	}

	name := q.generateParamName(prefix)
	q.vars[name] = value
	return name
}

// escapeIdent escapes an identifier for use in SurrealQL
func escapeIdent(ident string) string {
	// If the identifier contains special characters, wrap it in backticks
	if strings.ContainsAny(ident, " -:`") || isReservedWord(ident) {
		// Escape any backticks in the identifier with backslash
		return "`" + strings.ReplaceAll(ident, "`", "\\`") + "`"
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
