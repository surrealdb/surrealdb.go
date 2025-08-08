package surrealql

import (
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// selectTarget is an interface for targets in SELECT queries.
// It supports tables, records, subqueries, and raw values.
type selectTarget interface {
	string | *target | models.RecordID | *models.RecordID | *SelectQuery | models.Table
}

// SelectFrom creates a new SELECT query starting with the FROM clause.
// This allows for more natural query building where you first specify what to select from,
// then optionally add fields. If no fields are specified, it defaults to SELECT *.
//
// The target can be:
// - A string for raw expressions: "users", "users:123", or any SurrealQL expression
//   - Supports placeholders with "?" that will be replaced by args
//   - For arrays and objects, use SelectFrom("?", myArray) or SelectFrom("?", myMap)
//
// - A models.Table for safe table name specification
// - A *target for programmatic table/record specification
// - A *models.RecordID for specific records
// - A *SelectQuery for subqueries
//
// Examples:
//
//	// Select all from a table
//	SelectFrom("users")  // SELECT * FROM users
//
//	// Select from a specific record
//	SelectFrom("users:123")  // SELECT * FROM users:123
//
//	// Select from a parameterized graph traversal
//	SelectFrom("?->knows->users", models.NewRecordID("users", "john"))
//	// SELECT * FROM $from_param_1->knows->users
//
//	// Select from a subquery
//	subquery := Select("name").FromTable("users")
//	SelectFrom(subquery)  // SELECT * FROM (SELECT name FROM users)
//
//	// Select from a table using models.Table for type safety
//	SelectFrom(models.Table("users"))  // SELECT * FROM $table_1
//
//	// Select from an array or object using placeholders
//	SelectFrom("?", []any{1, 2, 3})  // SELECT * FROM $from_param_1
//	SelectFrom("?", map[string]any{"a": 1})  // SELECT * FROM $from_param_1
func SelectFrom[T selectTarget](target T, args ...any) *SelectQuery {
	bq := newBaseQuery()
	fromExpr := buildSelectTargetExprWithArgs(target, args, &bq)

	return &SelectQuery{
		baseQuery: bq,
		fields:    []string{"*"}, // Default to SELECT *
		from:      fromExpr,
	}
}

// buildSelectTargetExprWithArgs builds a FROM expression for SELECT queries with placeholder support.
// It supports various source types including tables, records, subqueries, arrays, and objects.
// When the target is a string with placeholders (?), the args will be used to replace them.
func buildSelectTargetExprWithArgs(f any, args []any, bq *baseQuery) string {
	if f == nil {
		return ""
	}

	switch v := f.(type) {
	case string:
		return handleStringTarget(v, args, bq)
	case *target:
		return handleTargetStruct(v, bq)
	case models.RecordID:
		return handleRecordID(v, bq)
	case *models.RecordID:
		return handleRecordID(*v, bq)
	case *SelectQuery:
		return handleSubquery(v, bq)
	case models.Table:
		return handleTable(v, bq)
	default:
		panic(fmt.Sprintf("unsupported select target type: %T", f))
	}
}

// handleStringTarget processes string targets with optional placeholder support
func handleStringTarget(v string, args []any, bq *baseQuery) string {
	if len(args) > 0 && strings.Contains(v, "?") {
		// Replace placeholders with properly formatted values
		processedExpr := v
		for _, arg := range args {
			replacement := formatFromArgument(arg, bq)
			processedExpr = strings.Replace(processedExpr, "?", replacement, 1)
		}
		return processedExpr
	}
	// No placeholders or args, return as-is
	return v
}

// formatFromArgument formats an argument for use in a FROM clause
// Generates parameters for all placeholders - SurrealDB will validate if they're allowed
func formatFromArgument(arg any, bq *baseQuery) string {
	switch v := arg.(type) {
	case string:
		// String values always become parameters
		paramName := bq.generateParamName("from_param")
		bq.addParam(paramName, v)
		return "$" + paramName
	case models.RecordID:
		// RecordID always becomes a parameter
		paramName := bq.generateParamName("from_param")
		bq.addParam(paramName, v)
		return "$" + paramName
	case *models.RecordID:
		// RecordID pointer always becomes a parameter
		paramName := bq.generateParamName("from_param")
		bq.addParam(paramName, *v)
		return "$" + paramName
	case *target:
		// Target struct - build it and use its parameters
		sql, vars := v.Build()
		for k, val := range vars {
			bq.addParam(k, val)
		}
		return sql
	default:
		// For other types, always create a parameter
		paramName := bq.generateParamName("from_param")
		bq.addParam(paramName, arg)
		return "$" + paramName
	}
}

// handleTargetStruct processes target struct types
func handleTargetStruct(v *target, bq *baseQuery) string {
	sql, targetVars := v.Build()
	for k, val := range targetVars {
		bq.addParam(k, val)
	}
	return sql
}

// handleRecordID processes RecordID types
// models.RecordID already has the correct CBOR type, so we just parameterize it
func handleRecordID(v models.RecordID, bq *baseQuery) string {
	paramName := bq.generateParamName("record_id")
	bq.addParam(paramName, v)
	return fmt.Sprintf("$%s", paramName)
}

// handleSubquery processes subquery types
func handleSubquery(v *SelectQuery, bq *baseQuery) string {
	sql, subVars := v.Build()
	for k, val := range subVars {
		bq.addParam(k, val)
	}
	return fmt.Sprintf("(%s)", sql)
}

// handleTable processes models.Table types
// models.Table already has the correct CBOR type, so we just parameterize it
func handleTable(v models.Table, bq *baseQuery) string {
	paramName := bq.generateParamName("table")
	bq.addParam(paramName, v)
	return fmt.Sprintf("$%s", paramName)
}
