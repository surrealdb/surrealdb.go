package surrealql

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// setsBuilder provides common functionality for building SET clauses
type setsBuilder struct {
	sets    map[string]any
	setsRaw []string
}

// newSetsBuilder creates a new setsBuilder
func newSetsBuilder() setsBuilder {
	return setsBuilder{
		sets: make(map[string]any),
	}
}

// addSet adds a field or expression to the SET clause
// Can be used for simple assignment: addSet("name", "value")
// Or for compound operations: addSet("count += ?", 1)
func (sb *setsBuilder) addSet(expr string, args []any, base *baseQuery, paramPrefix string) {
	// Check if this is a simple field assignment or an expression
	if len(args) == 1 && !strings.ContainsAny(expr, "?+=<>!-*/") {
		// Simple field assignment
		sb.sets[expr] = args[0]
	} else if len(args) > 0 {
		// Expression with placeholders
		processedExpr := expr
		for _, arg := range args {
			paramName := base.generateParamName(paramPrefix)
			processedExpr = strings.Replace(processedExpr, "?", "$"+paramName, 1)
			base.addParam(paramName, arg)
		}
		sb.setsRaw = append(sb.setsRaw, processedExpr)
	} else {
		// Raw expression without placeholders
		sb.setsRaw = append(sb.setsRaw, expr)
	}
}

// addSetMap adds multiple fields from a map
func (sb *setsBuilder) addSetMap(fields map[string]any) {
	maps.Copy(sb.sets, fields)
}

// buildSetClause builds the SET clause and adds parameters to the base query
func (sb *setsBuilder) buildSetClause(base *baseQuery, paramPrefix string) string {
	if len(sb.sets) == 0 && len(sb.setsRaw) == 0 {
		return ""
	}

	var setParts []string

	// Handle SET fields
	if len(sb.sets) > 0 {
		setsKeys := sort.StringSlice(slices.Collect(maps.Keys(sb.sets)))
		sort.Stable(setsKeys)

		for _, field := range setsKeys {
			value := sb.sets[field]
			var paramName string
			if paramPrefix != "" {
				paramName = base.generateParamName(paramPrefix + "_" + field)
			} else {
				paramName = base.generateParamName(field)
			}
			base.addParam(paramName, value)
			setParts = append(setParts, fmt.Sprintf("%s = $%s", escapeIdent(field), paramName))
		}
	}

	// Handle raw SET expressions
	setParts = append(setParts, sb.setsRaw...)

	return strings.Join(setParts, ", ")
}
