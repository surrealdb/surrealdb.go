package surrealql

import (
	"fmt"
	"strings"
	"time"
)

// DefineTableQuery represents a DEFINE TABLE query
type DefineTableQuery struct {
	table           string
	changefeed      string
	includeOriginal bool
	schemafull      bool
	schemaless      bool
	permissions     []permission
	fields          []string
}

// permission holds permission type and value
type permission struct {
	perm  string
	value string
}

// DefineTable creates a new DEFINE TABLE query
func DefineTable(table string) *DefineTableQuery {
	return &DefineTableQuery{
		table:       table,
		permissions: make([]permission, 0),
		fields:      make([]string, 0),
	}
}

// Changefeed enables changefeed for the table with specified duration
func (q *DefineTableQuery) Changefeed(duration string) *DefineTableQuery {
	q.changefeed = duration
	return q
}

// ChangefeedDuration enables changefeed for the table with specified duration
func (q *DefineTableQuery) ChangefeedDuration(duration time.Duration) *DefineTableQuery {
	q.changefeed = duration.String()
	return q
}

// ChangefeedWithOriginal enables changefeed with INCLUDE ORIGINAL option
func (q *DefineTableQuery) ChangefeedWithOriginal(duration string) *DefineTableQuery {
	q.changefeed = duration
	q.includeOriginal = true
	return q
}

// ChangefeedDurationWithOriginal enables changefeed with INCLUDE ORIGINAL option
func (q *DefineTableQuery) ChangefeedDurationWithOriginal(duration time.Duration) *DefineTableQuery {
	q.changefeed = duration.String()
	q.includeOriginal = true
	return q
}

// Schemafull sets the table to SCHEMAFULL mode
func (q *DefineTableQuery) Schemafull() *DefineTableQuery {
	q.schemafull = true
	q.schemaless = false
	return q
}

// Schemaless sets the table to SCHEMALESS mode
func (q *DefineTableQuery) Schemaless() *DefineTableQuery {
	q.schemaless = true
	q.schemafull = false
	return q
}

// Permissions sets permissions for the table
func (q *DefineTableQuery) Permissions(perm, value string) *DefineTableQuery {
	q.permissions = append(q.permissions, permission{perm: perm, value: value})
	return q
}

// Build returns the SurrealQL string and parameters for the query
func (q *DefineTableQuery) Build() (query string, params map[string]any) {
	c := newQueryBuildContext()

	var builder strings.Builder

	builder.WriteString("DEFINE TABLE ")
	builder.WriteString(escapeIdent(q.table))

	// Add schema mode
	if q.schemafull {
		builder.WriteString(" SCHEMAFULL")
	} else if q.schemaless {
		builder.WriteString(" SCHEMALESS")
	}

	// Add changefeed if specified
	if q.changefeed != "" {
		builder.WriteString(" CHANGEFEED ")
		builder.WriteString(q.changefeed)
		if q.includeOriginal {
			builder.WriteString(" INCLUDE ORIGINAL")
		}
	}

	// Add permissions if specified
	if len(q.permissions) > 0 {
		builder.WriteString(" PERMISSIONS")
		for _, p := range q.permissions {
			builder.WriteString(fmt.Sprintf(" %s %s", strings.ToUpper(p.perm), p.value))
		}
	}

	return builder.String(), c.vars
}

// String returns the SurrealQL string for the query
func (q *DefineTableQuery) String() string {
	sql, _ := q.Build()
	return sql
}

// DefineFieldQuery represents a DEFINE FIELD query
type DefineFieldQuery struct {
	table    string
	field    string
	dataType string
	value    string
	assert   string
	default_ string
}

// DefineField creates a new DEFINE FIELD query
func DefineField(field, table string) *DefineFieldQuery {
	return &DefineFieldQuery{
		field: field,
		table: table,
	}
}

// Type sets the field type
func (q *DefineFieldQuery) Type(dataType string) *DefineFieldQuery {
	q.dataType = dataType
	return q
}

// Value sets the field value expression
func (q *DefineFieldQuery) Value(value string) *DefineFieldQuery {
	q.value = value
	return q
}

// Assert sets the field assertion
func (q *DefineFieldQuery) Assert(assert string) *DefineFieldQuery {
	q.assert = assert
	return q
}

// Default sets the field default value
func (q *DefineFieldQuery) Default(defaultValue string) *DefineFieldQuery {
	q.default_ = defaultValue
	return q
}

// Build returns the SurrealQL string and parameters for the query
func (q *DefineFieldQuery) Build() (query string, params map[string]any) {
	c := newQueryBuildContext()

	var builder strings.Builder

	builder.WriteString("DEFINE FIELD ")
	builder.WriteString(escapeIdent(q.field))
	builder.WriteString(" ON TABLE ")
	builder.WriteString(escapeIdent(q.table))

	if q.dataType != "" {
		builder.WriteString(" TYPE ")
		builder.WriteString(q.dataType)
	}

	if q.value != "" {
		builder.WriteString(" VALUE ")
		builder.WriteString(q.value)
	}

	if q.assert != "" {
		builder.WriteString(" ASSERT ")
		builder.WriteString(q.assert)
	}

	if q.default_ != "" {
		builder.WriteString(" DEFAULT ")
		builder.WriteString(q.default_)
	}

	return builder.String(), c.vars
}

// String returns the SurrealQL string for the query
func (q *DefineFieldQuery) String() string {
	sql, _ := q.Build()
	return sql
}
