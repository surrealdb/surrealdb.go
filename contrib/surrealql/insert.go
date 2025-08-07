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
	baseQuery
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
		baseQuery:      newBaseQuery(),
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
	var builder strings.Builder

	q.buildInsertClause(&builder)
	q.buildDataOrValues(&builder)
	q.buildReturnClause(&builder)

	return builder.String(), q.vars
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
func (q *InsertQuery) buildDataOrValues(builder *strings.Builder) {
	if q.valueQuery != nil {
		q.buildValueQuery(builder)
	} else if q.value != nil {
		q.buildValueParam(builder)
	} else if len(q.fields) > 0 && len(q.values) > 0 {
		q.buildFieldsValues(builder)
		q.buildOnDuplicate(builder)
	}
}

// buildValueQuery builds the value query part
func (q *InsertQuery) buildValueQuery(builder *strings.Builder) {
	builder.WriteString(" (")
	sql, vars := q.valueQuery.Build()
	builder.WriteString(sql)
	builder.WriteString(")")

	// Merge parameters from the value query
	for k, v := range vars {
		q.addParam(k, v)
	}
}

// buildValueParam builds the value parameter
func (q *InsertQuery) buildValueParam(builder *strings.Builder) {
	paramName := q.generateParamName("insert_data")
	builder.WriteString(" $")
	builder.WriteString(paramName)
	q.addParam(paramName, q.value)
}

// buildFieldsValues builds the fields and values part
func (q *InsertQuery) buildFieldsValues(builder *strings.Builder) {
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
			paramName := q.generateParamName(fmt.Sprintf("insert_%d_%d", i, j))
			builder.WriteString("$")
			builder.WriteString(paramName)
			q.addParam(paramName, value)
		}
		builder.WriteString(")")
	}
}

// buildOnDuplicate builds the ON DUPLICATE KEY UPDATE part
func (q *InsertQuery) buildOnDuplicate(builder *strings.Builder) {
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

		paramName := q.generateParamName("dup_" + field)
		builder.WriteString("$")
		builder.WriteString(paramName)
		q.addParam(paramName, value)
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

// InsertBuilder provides a fluent interface for building complex insert data
type InsertBuilder struct {
	data map[string]any
}

// NewRelationData creates a new insert data builder
func NewRelationData() *InsertBuilder {
	return &InsertBuilder{
		data: make(map[string]any),
	}
}

// Set adds a field-value pair to the insert data
func (b *InsertBuilder) Set(field string, value any) *InsertBuilder {
	b.data[field] = value
	return b
}

// SetIn sets the 'in' field for relation inserts
func (b *InsertBuilder) SetIn(record string) *InsertBuilder {
	b.data["in"] = record
	return b
}

// SetOut sets the 'out' field for relation inserts
func (b *InsertBuilder) SetOut(record string) *InsertBuilder {
	b.data["out"] = record
	return b
}

// SetID sets the 'id' field for relation inserts
func (b *InsertBuilder) SetID(id string) *InsertBuilder {
	b.data["id"] = id
	return b
}

// Build returns the built data map
func (b *InsertBuilder) Build() map[string]any {
	return b.data
}
