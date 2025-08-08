package surrealql

import (
	"fmt"
	"strings"
)

// PatchOp represents a JSON Patch operation
//
// See https://jsonpatch.com/ for details on JSON Patch operations.
type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// UpsertQuery represents the initial UPSERT query that can be converted to specific types
type UpsertQuery struct {
	baseQuery
	only    bool
	targets []string
}

// upsertCommon contains common functionality for all UPSERT query types
type upsertCommon struct {
	*UpsertQuery
	whereClause  *whereBuilder
	returnClause string
	timeout      string
	parallel     bool
	explain      string
}

// UpsertSetQuery represents an UPSERT query with SET/UNSET
type UpsertSetQuery struct {
	upsertCommon
	setsBuilder
	unsets []string
}

// UpsertContentQuery represents an UPSERT query with CONTENT
type UpsertContentQuery struct {
	upsertCommon
	content map[string]any
}

// UpsertMergeQuery represents an UPSERT query with MERGE
type UpsertMergeQuery struct {
	upsertCommon
	merge map[string]any
}

// UpsertPatchQuery represents an UPSERT query with PATCH
type UpsertPatchQuery struct {
	upsertCommon
	patch []PatchOp
}

// UpsertReplaceQuery represents an UPSERT query with REPLACE
type UpsertReplaceQuery struct {
	upsertCommon
	replace map[string]any
}

// Upsert starts an UPSERT query
func Upsert[T mutationTarget](target T, targets ...T) *UpsertQuery {
	q := &UpsertQuery{
		baseQuery: newBaseQuery(),
	}

	upsertAddTarget(q, target)
	for _, t := range targets {
		upsertAddTarget(q, t)
	}

	return q
}

// UpsertOnly starts an UPSERT ONLY query (returns single record)
func UpsertOnly[T mutationTarget](target T) *UpsertQuery {
	q := &UpsertQuery{
		baseQuery: newBaseQuery(),
		only:      true,
	}
	upsertAddTarget(q, target)
	return q
}

func upsertAddTarget[MT mutationTarget](q *UpsertQuery, target MT) {
	sql, vars := buildTargetExpr(target)
	q.targets = append(q.targets, sql)
	for k, v := range vars {
		q.addParam(k, v)
	}
}

// Set converts to UpsertSetQuery and adds a field or expression
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *UpsertQuery) Set(expr string, args ...any) *UpsertSetQuery {
	setQuery := &UpsertSetQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		setsBuilder:  newSetsBuilder(),
	}
	return setQuery.Set(expr, args...)
}

// SetMap converts to UpsertSetQuery and sets multiple fields
func (q *UpsertQuery) SetMap(fields map[string]any) *UpsertSetQuery {
	setQuery := &UpsertSetQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		setsBuilder:  newSetsBuilder(),
	}
	setQuery.addSetMap(fields)
	return setQuery
}

// SetRaw converts to UpsertSetQuery and adds a raw SET expression
// Deprecated: Use Set() instead, which now supports raw expressions
func (q *UpsertQuery) SetRaw(expr string) *UpsertSetQuery {
	setQuery := &UpsertSetQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		setsBuilder:  newSetsBuilder(),
	}
	setQuery.setsRaw = append(setQuery.setsRaw, expr)
	return setQuery
}

// Content converts to UpsertContentQuery
func (q *UpsertQuery) Content(content map[string]any) *UpsertContentQuery {
	return &UpsertContentQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		content:      content,
	}
}

// Merge converts to UpsertMergeQuery
func (q *UpsertQuery) Merge(data map[string]any) *UpsertMergeQuery {
	return &UpsertMergeQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		merge:        data,
	}
}

// Patch converts to UpsertPatchQuery
func (q *UpsertQuery) Patch(ops []PatchOp) *UpsertPatchQuery {
	return &UpsertPatchQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		patch:        ops,
	}
}

// Replace converts to UpsertReplaceQuery
func (q *UpsertQuery) Replace(data map[string]any) *UpsertReplaceQuery {
	return &UpsertReplaceQuery{
		upsertCommon: upsertCommon{UpsertQuery: q},
		replace:      data,
	}
}

// Build returns the SurrealQL string and parameters for an UPSERT without data modification
// This is valid in SurrealDB and will create the record if it doesn't exist
func (q *UpsertQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string for an UPSERT without data modification
func (q *UpsertQuery) String() string {
	var sql strings.Builder

	sql.WriteString("UPSERT")

	if q.only {
		sql.WriteString(" ONLY")
	}

	sql.WriteString(" ")
	for i, t := range q.targets {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(t)
	}

	return sql.String()
}

// Methods for UpsertSetQuery

// Set adds another field or expression to upsert
// Can be used for simple assignment: Set("name", "value")
// Or for compound operations: Set("count += ?", 1)
func (q *UpsertSetQuery) Set(expr string, args ...any) *UpsertSetQuery {
	q.addSet(expr, args, &q.baseQuery, "upsert_param")
	return q
}

// SetMap adds multiple fields from a map
func (q *UpsertSetQuery) SetMap(fields map[string]any) *UpsertSetQuery {
	q.addSetMap(fields)
	return q
}

// SetRaw adds a raw SET expression
// Deprecated: Use Set() instead, which now supports raw expressions
func (q *UpsertSetQuery) SetRaw(expr string) *UpsertSetQuery {
	q.setsRaw = append(q.setsRaw, expr)
	return q
}

// Unset removes fields
func (q *UpsertSetQuery) Unset(fields ...string) *UpsertSetQuery {
	q.unsets = append(q.unsets, fields...)
	return q
}

// Where adds a WHERE condition
func (q *UpsertSetQuery) Where(condition string, args ...any) *UpsertSetQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *UpsertSetQuery) Return(clause string) *UpsertSetQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *UpsertSetQuery) ReturnNone() *UpsertSetQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *UpsertSetQuery) ReturnBefore() *UpsertSetQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *UpsertSetQuery) ReturnAfter() *UpsertSetQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *UpsertSetQuery) ReturnDiff() *UpsertSetQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// ReturnValue sets RETURN VALUE for a specific field
//
// Although the original RETURN VALUE clause supports
// any expression, including a SELECT query within it,
// this implementation only supports a single field name.
// This is because SurrealDB's RETURN VALUE clause
// is typically used to return a specific field value
// after an UPSERT operation, rather than a complex expression.
// If you need to return a complex expression, consider using
// the RETURN clause with a full SELECT query instead.
// For example, use `Return("VALUE (SELECT * FROM product WHERE parent = $parent.id)")`
func (q *UpsertSetQuery) ReturnValue(field string) *UpsertSetQuery {
	q.returnClause = "VALUE " + escapeIdent(field)
	return q
}

// Timeout sets the timeout duration
func (q *UpsertSetQuery) Timeout(duration string) *UpsertSetQuery {
	q.timeout = duration
	return q
}

// Parallel enables parallel execution
func (q *UpsertSetQuery) Parallel() *UpsertSetQuery {
	q.parallel = true
	return q
}

// Explain adds EXPLAIN clause
func (q *UpsertSetQuery) Explain() *UpsertSetQuery {
	q.explain = ExplainClause
	return q
}

// ExplainFull adds EXPLAIN FULL clause
func (q *UpsertSetQuery) ExplainFull() *UpsertSetQuery {
	q.explain = ExplainFullClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *UpsertSetQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *UpsertSetQuery) String() string {
	var sql strings.Builder

	if q.explain != "" {
		sql.WriteString(q.explain)
		sql.WriteString(" ")
	}

	q.buildPrefix(&sql)

	// Build SET clause using common setsBuilder
	if setClause := q.buildSetClause(&q.baseQuery, "upsert"); setClause != "" || len(q.unsets) > 0 {
		sql.WriteString(" SET ")

		var setParts []string

		// Add the built SET clause from setsBuilder
		if setClause != "" {
			setParts = append(setParts, setClause)
		}

		// Handle UNSET fields - single UNSET followed by comma-separated fields
		if len(q.unsets) > 0 {
			unsetFields := make([]string, len(q.unsets))
			for i, field := range q.unsets {
				unsetFields[i] = escapeIdent(field)
			}
			setParts = append(setParts, "UNSET "+strings.Join(unsetFields, ", "))
		}

		sql.WriteString(strings.Join(setParts, ", "))
	}

	q.buildSuffix(&sql)
	return sql.String()
}

// Methods for UpsertContentQuery

// Where adds a WHERE condition
func (q *UpsertContentQuery) Where(condition string, args ...any) *UpsertContentQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *UpsertContentQuery) Return(clause string) *UpsertContentQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *UpsertContentQuery) ReturnNone() *UpsertContentQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *UpsertContentQuery) ReturnBefore() *UpsertContentQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *UpsertContentQuery) ReturnAfter() *UpsertContentQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *UpsertContentQuery) ReturnDiff() *UpsertContentQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// ReturnValue sets RETURN VALUE for a specific field
func (q *UpsertContentQuery) ReturnValue(field string) *UpsertContentQuery {
	q.returnClause = "VALUE " + escapeIdent(field)
	return q
}

// Timeout sets the timeout duration
func (q *UpsertContentQuery) Timeout(duration string) *UpsertContentQuery {
	q.timeout = duration
	return q
}

// Parallel enables parallel execution
func (q *UpsertContentQuery) Parallel() *UpsertContentQuery {
	q.parallel = true
	return q
}

// Explain adds EXPLAIN clause
func (q *UpsertContentQuery) Explain() *UpsertContentQuery {
	q.explain = ExplainClause
	return q
}

// ExplainFull adds EXPLAIN FULL clause
func (q *UpsertContentQuery) ExplainFull() *UpsertContentQuery {
	q.explain = ExplainFullClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *UpsertContentQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *UpsertContentQuery) String() string {
	var sql strings.Builder

	if q.explain != "" {
		sql.WriteString(q.explain)
		sql.WriteString(" ")
	}

	q.buildPrefix(&sql)

	if len(q.content) > 0 {
		paramName := q.generateParamName("upsert_content")
		q.addParam(paramName, q.content)
		sql.WriteString(fmt.Sprintf(" CONTENT $%s", paramName))
	}

	q.buildSuffix(&sql)
	return sql.String()
}

// Methods for UpsertMergeQuery

// Where adds a WHERE condition
func (q *UpsertMergeQuery) Where(condition string, args ...any) *UpsertMergeQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *UpsertMergeQuery) Return(clause string) *UpsertMergeQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *UpsertMergeQuery) ReturnNone() *UpsertMergeQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *UpsertMergeQuery) ReturnBefore() *UpsertMergeQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *UpsertMergeQuery) ReturnAfter() *UpsertMergeQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *UpsertMergeQuery) ReturnDiff() *UpsertMergeQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// ReturnValue sets RETURN VALUE for a specific field
func (q *UpsertMergeQuery) ReturnValue(field string) *UpsertMergeQuery {
	q.returnClause = "VALUE " + escapeIdent(field)
	return q
}

// Timeout sets the timeout duration
func (q *UpsertMergeQuery) Timeout(duration string) *UpsertMergeQuery {
	q.timeout = duration
	return q
}

// Parallel enables parallel execution
func (q *UpsertMergeQuery) Parallel() *UpsertMergeQuery {
	q.parallel = true
	return q
}

// Explain adds EXPLAIN clause
func (q *UpsertMergeQuery) Explain() *UpsertMergeQuery {
	q.explain = ExplainClause
	return q
}

// ExplainFull adds EXPLAIN FULL clause
func (q *UpsertMergeQuery) ExplainFull() *UpsertMergeQuery {
	q.explain = ExplainFullClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *UpsertMergeQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *UpsertMergeQuery) String() string {
	var sql strings.Builder

	if q.explain != "" {
		sql.WriteString(q.explain)
		sql.WriteString(" ")
	}

	q.buildPrefix(&sql)

	if len(q.merge) > 0 {
		paramName := q.generateParamName("upsert_merge")
		q.addParam(paramName, q.merge)
		sql.WriteString(fmt.Sprintf(" MERGE $%s", paramName))
	}

	q.buildSuffix(&sql)
	return sql.String()
}

// Methods for UpsertPatchQuery

// Where adds a WHERE condition
func (q *UpsertPatchQuery) Where(condition string, args ...any) *UpsertPatchQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *UpsertPatchQuery) Return(clause string) *UpsertPatchQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *UpsertPatchQuery) ReturnNone() *UpsertPatchQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *UpsertPatchQuery) ReturnBefore() *UpsertPatchQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *UpsertPatchQuery) ReturnAfter() *UpsertPatchQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *UpsertPatchQuery) ReturnDiff() *UpsertPatchQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// ReturnValue sets RETURN VALUE for a specific field
func (q *UpsertPatchQuery) ReturnValue(field string) *UpsertPatchQuery {
	q.returnClause = "VALUE " + escapeIdent(field)
	return q
}

// Timeout sets the timeout duration
func (q *UpsertPatchQuery) Timeout(duration string) *UpsertPatchQuery {
	q.timeout = duration
	return q
}

// Parallel enables parallel execution
func (q *UpsertPatchQuery) Parallel() *UpsertPatchQuery {
	q.parallel = true
	return q
}

// Explain adds EXPLAIN clause
func (q *UpsertPatchQuery) Explain() *UpsertPatchQuery {
	q.explain = ExplainClause
	return q
}

// ExplainFull adds EXPLAIN FULL clause
func (q *UpsertPatchQuery) ExplainFull() *UpsertPatchQuery {
	q.explain = ExplainFullClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *UpsertPatchQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *UpsertPatchQuery) String() string {
	var sql strings.Builder

	if q.explain != "" {
		sql.WriteString(q.explain)
		sql.WriteString(" ")
	}

	q.buildPrefix(&sql)

	if len(q.patch) > 0 {
		paramName := q.generateParamName("upsert_patch")
		q.addParam(paramName, q.patch)
		sql.WriteString(fmt.Sprintf(" PATCH $%s", paramName))
	}

	q.buildSuffix(&sql)
	return sql.String()
}

// Methods for UpsertReplaceQuery

// Where adds a WHERE condition
func (q *UpsertReplaceQuery) Where(condition string, args ...any) *UpsertReplaceQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// Return sets the RETURN clause
func (q *UpsertReplaceQuery) Return(clause string) *UpsertReplaceQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *UpsertReplaceQuery) ReturnNone() *UpsertReplaceQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *UpsertReplaceQuery) ReturnBefore() *UpsertReplaceQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *UpsertReplaceQuery) ReturnAfter() *UpsertReplaceQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *UpsertReplaceQuery) ReturnDiff() *UpsertReplaceQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// ReturnValue sets RETURN VALUE for a specific field
func (q *UpsertReplaceQuery) ReturnValue(field string) *UpsertReplaceQuery {
	q.returnClause = "VALUE " + escapeIdent(field)
	return q
}

// Timeout sets the timeout duration
func (q *UpsertReplaceQuery) Timeout(duration string) *UpsertReplaceQuery {
	q.timeout = duration
	return q
}

// Parallel enables parallel execution
func (q *UpsertReplaceQuery) Parallel() *UpsertReplaceQuery {
	q.parallel = true
	return q
}

// Explain adds EXPLAIN clause
func (q *UpsertReplaceQuery) Explain() *UpsertReplaceQuery {
	q.explain = ExplainClause
	return q
}

// ExplainFull adds EXPLAIN FULL clause
func (q *UpsertReplaceQuery) ExplainFull() *UpsertReplaceQuery {
	q.explain = ExplainFullClause
	return q
}

// Build returns the SurrealQL string and parameters
func (q *UpsertReplaceQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// String returns the SurrealQL string
func (q *UpsertReplaceQuery) String() string {
	var sql strings.Builder

	if q.explain != "" {
		sql.WriteString(q.explain)
		sql.WriteString(" ")
	}

	q.buildPrefix(&sql)

	if len(q.replace) > 0 {
		paramName := q.generateParamName("upsert_replace")
		q.addParam(paramName, q.replace)
		sql.WriteString(fmt.Sprintf(" REPLACE $%s", paramName))
	}

	q.buildSuffix(&sql)
	return sql.String()
}

// Helper methods for upsertCommon

func (c *upsertCommon) buildPrefix(sql *strings.Builder) {
	sql.WriteString("UPSERT")

	if c.only {
		sql.WriteString(" ONLY")
	}

	sql.WriteString(" ")
	for i, t := range c.targets {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(t)
	}
}

func (c *upsertCommon) buildSuffix(sql *strings.Builder) {
	if c.whereClause != nil && c.whereClause.hasConditions() {
		sql.WriteString(" WHERE ")
		sql.WriteString(c.whereClause.build())
	}

	if c.returnClause != "" {
		sql.WriteString(" RETURN ")
		sql.WriteString(c.returnClause)
	}

	if c.timeout != "" {
		sql.WriteString(" TIMEOUT ")
		sql.WriteString(c.timeout)
	}

	if c.parallel {
		sql.WriteString(" PARALLEL")
	}
}
