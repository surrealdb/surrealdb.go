package surrealql

import (
	"fmt"
	"strings"
	"time"
)

// ShowChangesForTableQuery represents a SHOW CHANGES query
type ShowChangesForTableQuery struct {
	table string
	since string
	limit int
}

// ShowChangesForTable creates a new SHOW CHANGES query
func ShowChangesForTable(table string) *ShowChangesForTableQuery {
	return &ShowChangesForTableQuery{
		table: table,
	}
}

// Since sets the starting point for showing changes
// Can be either a timestamp (e.g., "d\"2023-09-07T01:23:52Z\"") or a version number (e.g., "0")
func (q *ShowChangesForTableQuery) Since(since string) *ShowChangesForTableQuery {
	q.since = since
	return q
}

// SinceVersionstamp sets the starting point for showing changes using a versionstamp.
//
// This method automatically handles the versionstamp adjustment required by SurrealDB.
// When you receive a versionstamp from a SHOW CHANGES query result, it includes extra
// bytes for FoundationDB ordering. This method automatically shifts the versionstamp
// right by 16 bits to extract the logical version needed for the SINCE clause.
//
// This allows you to directly use the versionstamp from a previous query result
// without manual adjustment:
//
//	// First query
//	changes1 := db.Query("SHOW CHANGES FOR TABLE users")
//	maxVs := getMaxVersionstamp(changes1)
//
//	// Continue from where you left off - no manual adjustment needed
//	q := surrealql.ShowChangesForTable("users").SinceVersionstamp(maxVs)
//
// See: https://surrealdb.com/docs/surrealql/statements/show
func (q *ShowChangesForTableQuery) SinceVersionstamp(versionstamp uint64) *ShowChangesForTableQuery {
	// SurrealDB versionstamps include extra bytes for FoundationDB ordering
	// When using SINCE, we need to shift right by 16 bits to get the logical version
	adjustedVs := versionstamp >> 16
	q.since = fmt.Sprintf("%d", adjustedVs)
	return q
}

// SinceTime sets the starting point for showing changes using a timestamp
func (q *ShowChangesForTableQuery) SinceTime(since *time.Time) *ShowChangesForTableQuery {
	q.since = fmt.Sprintf("d%q", since.Format(time.RFC3339))
	return q
}

// Limit sets the maximum number of changes to return
func (q *ShowChangesForTableQuery) Limit(limit int) *ShowChangesForTableQuery {
	q.limit = limit
	return q
}

// Build returns the SurrealQL string and parameters for the query
func (q *ShowChangesForTableQuery) Build() (sql string, vars map[string]any) {
	c := newQueryBuildContext()

	var builder strings.Builder

	builder.WriteString("SHOW CHANGES FOR TABLE ")
	builder.WriteString(escapeIdent(q.table))

	if q.since != "" {
		builder.WriteString(" SINCE ")
		builder.WriteString(q.since)
	}

	if q.limit > 0 {
		builder.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
	}

	return builder.String(), c.vars
}

// String returns the SurrealQL string for the query
func (q *ShowChangesForTableQuery) String() string {
	sql, _ := q.Build()
	return sql
}
