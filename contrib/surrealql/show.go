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
// SurrealDB reads SINCE in the same units as the versionstamps that SHOW CHANGES
// returns, so this method sends the versionstamp unchanged. You can use a
// versionstamp from a previous query result directly. SINCE is inclusive: the
// result starts with the change at that versionstamp, if there is one.
//
//	// First query
//	changes1 := db.Query("SHOW CHANGES FOR TABLE users")
//	maxVs := getMaxVersionstamp(changes1)
//
//	// Continue from the last change you received (it is returned again)
//	q := surrealql.ShowChangesForTable("users").SinceVersionstamp(maxVs)
//
// See: https://surrealdb.com/docs/surrealql/statements/show
func (q *ShowChangesForTableQuery) SinceVersionstamp(versionstamp uint64) *ShowChangesForTableQuery {
	q.since = fmt.Sprintf("%d", versionstamp)
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
		fmt.Fprintf(&builder, " LIMIT %d", q.limit)
	}

	return builder.String(), c.vars
}

// String returns the SurrealQL string for the query
func (q *ShowChangesForTableQuery) String() string {
	sql, _ := q.Build()
	return sql
}
