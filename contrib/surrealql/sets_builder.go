package surrealql

import (
	"strings"
)

// setsBuilder provides common functionality for building SET clauses
type setsBuilder struct {
	sets []*expr
}

// newSetsBuilder creates a new setsBuilder
func newSetsBuilder() setsBuilder {
	return setsBuilder{
		sets: make([]*expr, 0),
	}
}

func (sb *setsBuilder) hasSets() bool {
	return len(sb.sets) > 0
}

// addSet adds a field or expression to the SET clause
// Can be used for simple assignment: addSet("name = ?", "value")
// Or for compound operations: addSet("count += ?", 1)
func (sb *setsBuilder) addSet(expr string, args []any) {
	if !strings.Contains(expr, "?") && len(args) == 1 {
		expr = strings.TrimSpace(expr) + " = ?"
	}

	sb.sets = append(sb.sets, Expr(expr, args...))
}

// buildSetClause builds the SET clause and adds parameters to the base query
func (sb *setsBuilder) buildSetClause(base *queryBuildContext, b *strings.Builder) {
	if len(sb.sets) == 0 {
		return
	}

	b.WriteString("SET ")

	for i, setExpr := range sb.sets {
		if i > 0 {
			b.WriteString(", ")
		}
		setExpr.build(base, b)
	}
}
