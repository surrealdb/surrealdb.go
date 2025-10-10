package surrealql

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// InsertQuery represents an INSERT query
type InsertQuery struct {
	table      string
	isRelation bool
	ignore     bool

	// valueQuery is for INSERT INTO table (SELECT ...) queries
	valueQuery *SelectQuery

	// value is for INSERT INTO table $value queries
	value any

	fields            []string
	values            [][]any
	onDuplicateSet    map[string]any
	onDuplicateSetRaw []string
	returnClause      string
}

// Insert creates a new INSERT query
func Insert(table string) *InsertQuery {
	return &InsertQuery{
		table:          table,
		onDuplicateSet: make(map[string]any),
	}
}

// Ignore sets the IGNORE flag for the insert,
// which changes the syntax to INSERT IGNORE
func (q *InsertQuery) Ignore() *InsertQuery {
	q.ignore = true
	return q
}

// Relation sets the query as a relation insert,
// which changes the syntax to INSERT RELATION
func (q *InsertQuery) Relation() *InsertQuery {
	q.isRelation = true
	return q
}

// Value sets the data to insert (single record or array of records)
func (q *InsertQuery) Value(data any) *InsertQuery {
	q.value = data
	return q
}

// ValueQuery sets the query to use the result of the provided SELECT query
// as the data to insert
// This is used for INSERT INTO table (SELECT ...)
func (q *InsertQuery) ValueQuery(query *SelectQuery) *InsertQuery {
	q.valueQuery = query
	return q
}

// Fields sets the fields for VALUES insert
func (q *InsertQuery) Fields(fields ...string) *InsertQuery {
	q.fields = fields
	return q
}

// Values adds values for VALUES insert
func (q *InsertQuery) Values(values ...any) *InsertQuery {
	q.values = append(q.values, values)
	return q
}

// OnDuplicateKeyUpdateSet adds an ON DUPLICATE KEY UPDATE field = value clause
func (q *InsertQuery) OnDuplicateKeyUpdateSet(field string, value any) *InsertQuery {
	q.onDuplicateSet[field] = value
	return q
}

// OnDuplicateKeyUpdateRaw adds an ON DUPLICATE KEY UPDATE expression
func (q *InsertQuery) OnDuplicateKeyUpdateRaw(expr string) *InsertQuery {
	q.onDuplicateSetRaw = append(q.onDuplicateSetRaw, expr)
	return q
}

// Return sets the RETURN clause
func (q *InsertQuery) Return(clause string) *InsertQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *InsertQuery) ReturnNone() *InsertQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *InsertQuery) ReturnBefore() *InsertQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *InsertQuery) ReturnAfter() *InsertQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *InsertQuery) ReturnDiff() *InsertQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// Build returns the SurrealQL string and parameters for the query
func (q *InsertQuery) Build() (query string, vars map[string]any) {
	var b strings.Builder
	c := newQueryBuildContext()
	q.build(&c, &b)
	return b.String(), c.vars
}

func (q *InsertQuery) build(c *queryBuildContext, b *strings.Builder) {
	q.buildInsertClause(b)
	q.buildDataOrValues(c, b)
	q.buildReturnClause(b)
}

// buildInsertClause builds the INSERT clause part
func (q *InsertQuery) buildInsertClause(builder *strings.Builder) {
	builder.WriteString("INSERT")
	if q.ignore && !q.isRelation {
		builder.WriteString(" IGNORE")
	}
	if q.isRelation {
		builder.WriteString(" RELATION")
	}
	builder.WriteString(" INTO ")
	builder.WriteString(escapeIdent(q.table))
}

// buildDataOrValues builds the data or fields/values part
func (q *InsertQuery) buildDataOrValues(c *queryBuildContext, builder *strings.Builder) {
	if q.valueQuery != nil {
		q.buildValueQuery(c, builder)
	} else if q.value != nil {
		q.buildValueParam(c, builder)
	} else if len(q.fields) > 0 && len(q.values) > 0 {
		q.buildFieldsValues(c, builder)
		q.buildOnDuplicate(c, builder)
	}
}

// buildValueQuery builds the value query part
func (q *InsertQuery) buildValueQuery(c *queryBuildContext, builder *strings.Builder) {
	builder.WriteString(" (")
	q.valueQuery.build(c, builder)
	builder.WriteString(")")
}

// buildValueParam builds the value parameter
func (q *InsertQuery) buildValueParam(c *queryBuildContext, builder *strings.Builder) {
	paramName := c.generateAndAddParam("insert_data", q.value)
	builder.WriteString(" $")
	builder.WriteString(paramName)
}

// buildFieldsValues builds the fields and values part
func (q *InsertQuery) buildFieldsValues(c *queryBuildContext, builder *strings.Builder) {
	// Fields
	builder.WriteString(" (")
	for i, field := range q.fields {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(escapeIdent(field))
	}
	builder.WriteString(") VALUES")

	// Values
	for i, row := range q.values {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(" (")
		for j, value := range row {
			if j > 0 {
				builder.WriteString(", ")
			}
			paramName := c.generateAndAddParam(fmt.Sprintf("insert_%d_%d", i, j), value)
			builder.WriteString("$")
			builder.WriteString(paramName)
		}
		builder.WriteString(")")
	}
}

// buildOnDuplicate builds the ON DUPLICATE KEY UPDATE part
func (q *InsertQuery) buildOnDuplicate(c *queryBuildContext, builder *strings.Builder) {
	if len(q.onDuplicateSet) == 0 && len(q.onDuplicateSetRaw) == 0 {
		return
	}

	builder.WriteString(" ON DUPLICATE KEY UPDATE")
	first := true
	keys := sort.StringSlice(slices.Collect(maps.Keys(q.onDuplicateSet)))
	sort.Stable(keys)
	for _, field := range keys {
		if !first {
			builder.WriteString(",")
		} else {
			first = false
		}

		value := q.onDuplicateSet[field]

		builder.WriteString(" ")
		builder.WriteString(escapeIdent(field))
		builder.WriteString(" = ")

		paramName := c.generateAndAddParam("dup_"+field, value)
		builder.WriteString("$")
		builder.WriteString(paramName)
	}

	for _, expr := range q.onDuplicateSetRaw {
		if !first {
			builder.WriteString(",")
		} else {
			first = false
		}

		builder.WriteString(" ")
		builder.WriteString(expr)
	}
}

// buildReturnClause builds the RETURN clause
func (q *InsertQuery) buildReturnClause(builder *strings.Builder) {
	if q.returnClause != "" {
		builder.WriteString(" RETURN ")
		builder.WriteString(q.returnClause)
	}
}

// String returns the SurrealQL string for the query
func (q *InsertQuery) String() string {
	sql, _ := q.Build()
	return sql
}
