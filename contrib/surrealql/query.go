// Package surrealql provides a query builder for SurrealQL queries.
// It allows you to construct SurrealQL queries programmatically with type safety.
package surrealql

import (
	"fmt"
	"regexp"
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

// plainIdent matches an identifier which SurrealQL reads as the name it is,
// without quoting it, unless it is a reserved word.
var plainIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// identEscaper escapes what a backtick-quoted identifier gives a meaning to:
// a backslash, and the backtick that would end it.
var identEscaper = strings.NewReplacer(`\`, `\\`, "`", "\\`")

// escapeIdent writes a name, such as a table's, for use in SurrealQL. Unless it
// is a plain identifier which is not a reserved word, it is quoted with
// backticks, so that it is read as the name it is, whatever it holds: a ";", a
// comment or a keyword in it cannot change the statement.
func escapeIdent(ident string) string {
	if plainIdent.MatchString(ident) && !isReservedWord(ident) {
		return ident
	}
	return "`" + identEscaper.Replace(ident) + "`"
}

// escapeFieldPath writes a field path, such as "address.city", for use in
// SurrealQL: each segment as escapeIdent writes it, but for a "*" segment, as
// in "tags.*".
func escapeFieldPath(path string) string {
	segments := strings.Split(path, ".")
	for i, segment := range segments {
		if segment != "*" {
			segments[i] = escapeIdent(segment)
		}
	}
	return strings.Join(segments, ".")
}

// reservedWords are quoted where they name something. SurrealQL reads NONE,
// NULL, TRUE and FALSE as those values, so a field of that name would silently
// be replaced by the value, and it refuses statement keywords such as LET.
var reservedWords = map[string]bool{
	"NONE": true, "NULL": true, "TRUE": true, "FALSE": true,
	"SELECT": true, "FROM": true, "WHERE": true, "ORDER": true, "BY": true, "LIMIT": true, "START": true,
	"FETCH": true, "GROUP": true, "SPLIT": true, "RETURN": true, "PARALLEL": true, "EXPLAIN": true,
	"CREATE": true, "UPDATE": true, "UPSERT": true, "DELETE": true, "RELATE": true, "INSERT": true,
	"DEFINE": true, "REMOVE": true, "ALTER": true, "INFO": true, "USE": true, "LET": true,
	"BEGIN": true, "CANCEL": true, "COMMIT": true, "LIVE": true, "KILL": true, "SHOW": true,
	"IF": true, "ELSE": true, "THEN": true, "END": true, "FOR": true, "BREAK": true, "CONTINUE": true,
	"THROW": true, "SLEEP": true, "OPTION": true,
	"AND": true, "OR": true, "NOT": true, "IS": true, "IN": true, "AS": true, "ONLY": true, "VALUE": true,
	"BEFORE": true, "AFTER": true, "DIFF": true,
	"FUNCTION": true, "PARAM": true, "FIELD": true, "TYPE": true, "DEFAULT": true,
	"ASSERT": true, "PERMISSIONS": true, "DURATION": true, "FLEXIBLE": true,
}

// isReservedWord checks if a word is a SurrealQL reserved word
func isReservedWord(word string) bool {
	return reservedWords[strings.ToUpper(word)]
}

// hasLineComment reports whether SurrealQL text may hold a comment which runs
// to the end of the line. It may be in a string instead: then a line break
// after the text is still only whitespace.
func hasLineComment(sql string) bool {
	return strings.Contains(sql, "--") || strings.Contains(sql, "//") || strings.Contains(sql, "#")
}
