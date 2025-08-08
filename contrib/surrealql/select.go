package surrealql

import (
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// SelectQuery represents a SELECT query builder
type SelectQuery struct {
	baseQuery
	fields      []string
	omits       []string
	from        string
	whereClause *whereBuilder
	orderBy     []orderByClause
	limitVal    *int
	startVal    *int
	fetchFields []string
	groupBy     []string
	splitFields []string
	parallel    bool
	explain     bool
	// indicates if this query is a SELECT VALUE query
	value        bool
	returnClause string
}

// orderByClause represents an ORDER BY clause
type orderByClause struct {
	field   string
	desc    bool
	collate bool
	numeric bool
}

// SelectValue creates a `SELECT VALUE field FROM ...` query.
// It is used to select a single value per each record.
func SelectValue[T selectField](field T) *SelectQuery {
	bq := newBaseQuery()
	fs := make([]string, 0, 1)

	sql, vars := F(field).Build()
	fs = append(fs, sql)
	for k, v := range vars {
		bq.addParam(k, v)
	}

	return &SelectQuery{
		baseQuery: bq,
		fields:    fs,
		value:     true,
	}
}

// Select creates a new SELECT query builder.
func Select[T selectField](field T, fields ...T) *SelectQuery {
	bq := newBaseQuery()
	fs := make([]string, 0, len(fields)+1)

	sql, vars := F(field).Build()
	fs = append(fs, sql)
	for k, v := range vars {
		bq.addParam(k, v)
	}

	for _, field := range fields {
		sql, vars := F(field).Build()
		fs = append(fs, sql)
		for k, v := range vars {
			bq.addParam(k, v)
		}
	}

	return &SelectQuery{
		baseQuery: bq,
		fields:    fs,
	}
}

// Field adds a field to the SELECT query.
func (q *SelectQuery) Field(field *field) *SelectQuery {
	sql, vars := field.Build()
	// If fields only contains "*", replace it
	if len(q.fields) == 1 && q.fields[0] == "*" {
		q.fields = []string{sql}
	} else {
		q.fields = append(q.fields, sql)
	}
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// FieldName adds a field to the SELECT query.
func (q *SelectQuery) FieldName(field string) *SelectQuery {
	// If fields only contains "*", replace it
	if len(q.fields) == 1 && q.fields[0] == "*" {
		q.fields = []string{escapeIdent(field)}
	} else {
		q.fields = append(q.fields, escapeIdent(field))
	}
	return q
}

// FieldNameAs adds a field with an alias to the SELECT query.
func (q *SelectQuery) FieldNameAs(field, alias string) *SelectQuery {
	// If fields only contains "*", replace it
	if len(q.fields) == 1 && q.fields[0] == "*" {
		q.fields = []string{fmt.Sprintf("%s AS %s", escapeIdent(field), escapeIdent(alias))}
	} else {
		q.fields = append(q.fields, fmt.Sprintf("%s AS %s", escapeIdent(field), escapeIdent(alias)))
	}
	return q
}

// AddQuery adds another SelectQuery as a field to the current query.
func (q *SelectQuery) FieldQueryAs(query *SelectQuery, alias string) *SelectQuery {
	sql, vars := F(query).As(alias).Build()
	// If fields only contains "*", replace it
	if len(q.fields) == 1 && q.fields[0] == "*" {
		q.fields = []string{sql}
	} else {
		q.fields = append(q.fields, sql)
	}
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// FieldFunCallAs adds a function call as a field to the SELECT query.
func (q *SelectQuery) FieldFunCallAs(fun *FunCall, alias string) *SelectQuery {
	sql, vars := F(fun).As(alias).Build()
	// If fields only contains "*", replace it
	if len(q.fields) == 1 && q.fields[0] == "*" {
		q.fields = []string{sql}
	} else {
		q.fields = append(q.fields, sql)
	}
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// FieldRaw adds a raw field to the SELECT query without escaping.
// This is useful for fields that should not be escaped, such as function calls.
func (q *SelectQuery) FieldRaw(field string) *SelectQuery {
	// If fields only contains "*", replace it
	if len(q.fields) == 1 && q.fields[0] == "*" {
		q.fields = []string{field}
	} else {
		q.fields = append(q.fields, field)
	}
	return q
}

// Omit removes a field from the SELECT query by specifying OMIT clause.
// This is useful for excluding specific fields from the result set.
// Valid only when `SELECT *` is used.
func (q *SelectQuery) Omit(field string) *SelectQuery {
	// Omit a field from the SELECT query
	q.omits = append(q.omits, escapeIdent(field))
	return q
}

// OmitRaw adds a raw OMIT clause to the SELECT query.
// This allows specifying fields to omit without escaping.
// This is useful for using destructuring syntax described in
// https://surrealdb.com/docs/surrealql/statements/select#skip-certain-fields-using-the-omit-clause
func (q *SelectQuery) OmitRaw(field string) *SelectQuery {
	// Add raw OMIT clause without escaping
	q.omits = append(q.omits, field)
	return q
}

// FromTable sets the FROM clause of the query
// The from parameter can be:
// - A table name: "users"
// - A specific record: "users:123"
// - A RecordID string representation
func (q *SelectQuery) FromTable(table string) *SelectQuery {
	q.from = table
	return q
}

// From sets the FROM clause using a target expression.
// The target can be a table, or a specific record ID.
func (q *SelectQuery) From(thing *target) *SelectQuery {
	sql, vars := buildTargetExpr(thing)
	q.from = sql
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// FromQuery sets the FROM clause using another SelectQuery.
// This allows using the result of another query as the source for this query.
func (q *SelectQuery) FromQuery(query *SelectQuery) *SelectQuery {
	sql, vars := query.Build()
	q.from = fmt.Sprintf("(%s)", sql)
	for k, v := range vars {
		q.addParam(k, v)
	}
	return q
}

// FromRecordID sets the FROM clause using a RecordID
func (q *SelectQuery) FromRecordID(recordID models.RecordID) *SelectQuery {
	q.from = recordID.String()
	return q
}

// Where adds a WHERE condition to the query.
//
// All values are automatically parameterized to prevent injection:
//
//	query := surrealql.Select().From("users").
//	    Where("age > ? AND status = ?", 18, "active")
//	// Generates: SELECT * FROM users WHERE age > $param_1 AND status = $param_2
func (q *SelectQuery) Where(condition string, args ...any) *SelectQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args, &q.baseQuery)
	return q
}

// WhereEq adds a WHERE equality condition
func (q *SelectQuery) WhereEq(field string, value any) *SelectQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	paramName := q.generateParamName(field)
	condition := fmt.Sprintf("%s = $%s", escapeIdent(field), paramName)
	q.addParam(paramName, value)
	q.whereClause.addRawCondition(condition)
	return q
}

// WhereIn adds a WHERE IN condition
func (q *SelectQuery) WhereIn(field string, values ...any) *SelectQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	if len(values) == 0 {
		return q
	}

	paramName := q.generateParamName(field + "_in")
	condition := fmt.Sprintf("%s IN $%s", escapeIdent(field), paramName)
	q.addParam(paramName, values)
	q.whereClause.addRawCondition(condition)
	return q
}

// WhereNotNull adds a WHERE IS NOT NULL condition
func (q *SelectQuery) WhereNotNull(field string) *SelectQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	condition := fmt.Sprintf("%s IS NOT NULL", escapeIdent(field))
	q.whereClause.addRawCondition(condition)
	return q
}

// WhereNull adds a WHERE IS NULL condition
func (q *SelectQuery) WhereNull(field string) *SelectQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	condition := fmt.Sprintf("%s IS NULL", escapeIdent(field))
	q.whereClause.addRawCondition(condition)
	return q
}

// OrderBy adds an ORDER BY clause
func (q *SelectQuery) OrderBy(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, desc: false})
	return q
}

// OrderByCollate adds an ORDER BY COLLATE clause
func (q *SelectQuery) OrderByCollate(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, collate: true})
	return q
}

// OrderByNumeric adds an ORDER BY NUMERIC clause
func (q *SelectQuery) OrderByNumeric(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, numeric: true})
	return q
}

// OrderByCollateNumeric adds an ORDER BY COLLATE NUMERIC clause
func (q *SelectQuery) OrderByCollateNumeric(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, collate: true, numeric: true})
	return q
}

// OrderByDesc adds an ORDER BY DESC clause
func (q *SelectQuery) OrderByDesc(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, desc: true})
	return q
}

// OrderByCollateDesc adds an ORDER BY COLLATE DESC clause
func (q *SelectQuery) OrderByCollateDesc(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, desc: true, collate: true})
	return q
}

// OrderByNumericDesc adds an ORDER BY NUMERIC DESC clause
func (q *SelectQuery) OrderByNumericDesc(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, desc: true, numeric: true})
	return q
}

// OrderByCollateNumericDesc adds an ORDER BY COLLATE NUMERIC DESC clause
func (q *SelectQuery) OrderByCollateNumericDesc(field string) *SelectQuery {
	q.orderBy = append(q.orderBy, orderByClause{field: field, desc: true, collate: true, numeric: true})
	return q
}

// Limit sets the LIMIT clause
func (q *SelectQuery) Limit(limit int) *SelectQuery {
	q.limitVal = &limit
	return q
}

// Start sets the START clause
func (q *SelectQuery) Start(start int) *SelectQuery {
	q.startVal = &start
	return q
}

// Fetch adds fields to fetch relationships
func (q *SelectQuery) Fetch(fields ...string) *SelectQuery {
	q.fetchFields = append(q.fetchFields, fields...)
	return q
}

// GroupBy adds GROUP BY fields
func (q *SelectQuery) GroupBy(fields ...string) *SelectQuery {
	q.groupBy = append(q.groupBy, fields...)
	return q
}

// GroupAll adds GROUP ALL clause for table-wide aggregation
func (q *SelectQuery) GroupAll() *SelectQuery {
	q.groupBy = []string{"ALL"}
	return q
}

// Split adds SPLIT AT fields
func (q *SelectQuery) Split(fields ...string) *SelectQuery {
	q.splitFields = append(q.splitFields, fields...)
	return q
}

// Parallel enables PARALLEL execution
func (q *SelectQuery) Parallel() *SelectQuery {
	q.parallel = true
	return q
}

// Explain enables EXPLAIN mode
func (q *SelectQuery) Explain() *SelectQuery {
	q.explain = true
	return q
}

// Return sets the RETURN clause
func (q *SelectQuery) Return(clause string) *SelectQuery {
	q.returnClause = clause
	return q
}

// ReturnNone sets RETURN NONE
func (q *SelectQuery) ReturnNone() *SelectQuery {
	q.returnClause = ReturnNoneClause
	return q
}

// ReturnDiff sets RETURN DIFF
func (q *SelectQuery) ReturnDiff() *SelectQuery {
	q.returnClause = ReturnDiffClause
	return q
}

// ReturnBefore sets RETURN BEFORE
func (q *SelectQuery) ReturnBefore() *SelectQuery {
	q.returnClause = ReturnBeforeClause
	return q
}

// ReturnAfter sets RETURN AFTER
func (q *SelectQuery) ReturnAfter() *SelectQuery {
	q.returnClause = ReturnAfterClause
	return q
}

// Build returns the SurrealQL string and parameters for the query
func (q *SelectQuery) Build() (sql string, vars map[string]any) {
	return q.String(), q.vars
}

// buildSelectClause builds the SELECT clause
func (q *SelectQuery) buildSelectClause() string {
	if len(q.fields) == 0 {
		return "SELECT *"
	}

	fields := make([]string, len(q.fields))
	for i, field := range q.fields {
		// Don't escape if it's *, contains parentheses (function call), or AS (alias)
		if field == "*" || strings.Contains(field, "(") || strings.Contains(field, " AS ") {
			fields[i] = field
		} else {
			fields[i] = escapeIdent(field)
		}
	}

	base := "SELECT "

	if q.value {
		base += "VALUE "
	}

	base += strings.Join(fields, ", ")

	if len(q.omits) > 0 {
		return base + " OMIT " + strings.Join(q.omits, ", ")
	}

	return base
}

// buildGroupClause builds the GROUP BY clause
func (q *SelectQuery) buildGroupClause() string {
	if len(q.groupBy) == 0 {
		return ""
	}

	if len(q.groupBy) == 1 && q.groupBy[0] == "ALL" {
		return "GROUP ALL"
	}

	groupFields := make([]string, len(q.groupBy))
	for i, field := range q.groupBy {
		groupFields[i] = escapeIdent(field)
	}
	return "GROUP BY " + strings.Join(groupFields, ", ")
}

// buildOrderClause builds the ORDER BY clause
func (q *SelectQuery) buildOrderClause() string {
	if len(q.orderBy) == 0 {
		return ""
	}

	orderClauses := make([]string, len(q.orderBy))
	for i, order := range q.orderBy {
		clause := escapeIdent(order.field)
		if order.desc {
			clause += " DESC"
		}
		orderClauses[i] = clause
	}
	return "ORDER BY " + strings.Join(orderClauses, ", ")
}

// buildSplitClause builds the SPLIT clause
func (q *SelectQuery) buildSplitClause() string {
	if len(q.splitFields) == 0 {
		return ""
	}

	splitFields := make([]string, len(q.splitFields))
	for i, field := range q.splitFields {
		splitFields[i] = escapeIdent(field)
	}
	return "SPLIT AT " + strings.Join(splitFields, ", ")
}

// buildFetchClause builds the FETCH clause
func (q *SelectQuery) buildFetchClause() string {
	if len(q.fetchFields) == 0 {
		return ""
	}

	fetchFields := make([]string, len(q.fetchFields))
	for i, field := range q.fetchFields {
		fetchFields[i] = escapeIdent(field)
	}
	return "FETCH " + strings.Join(fetchFields, ", ")
}

// String returns the SurrealQL string for the query
func (q *SelectQuery) String() string {
	var parts []string

	// Add EXPLAIN if enabled
	if q.explain {
		parts = append(parts, "EXPLAIN")
	}

	// SELECT clause
	parts = append(parts, q.buildSelectClause())

	// FROM clause
	if q.from != "" {
		parts = append(parts, "FROM "+q.from)
	}

	// WHERE clause
	if q.whereClause != nil && q.whereClause.hasConditions() {
		parts = append(parts, "WHERE "+q.whereClause.build())
	}

	// SPLIT clause
	if splitClause := q.buildSplitClause(); splitClause != "" {
		parts = append(parts, splitClause)
	}

	// GROUP BY clause
	if groupClause := q.buildGroupClause(); groupClause != "" {
		parts = append(parts, groupClause)
	}

	// ORDER BY clause
	if orderClause := q.buildOrderClause(); orderClause != "" {
		parts = append(parts, orderClause)
	}

	// LIMIT clause
	if q.limitVal != nil {
		parts = append(parts, fmt.Sprintf("LIMIT %d", *q.limitVal))
	}

	// START clause
	if q.startVal != nil {
		parts = append(parts, fmt.Sprintf("START %d", *q.startVal))
	}

	// FETCH clause
	if fetchClause := q.buildFetchClause(); fetchClause != "" {
		parts = append(parts, fetchClause)
	}

	// PARALLEL clause
	if q.parallel {
		parts = append(parts, "PARALLEL")
	}

	// RETURN clause
	if q.returnClause != "" {
		parts = append(parts, "RETURN "+q.returnClause)
	}

	return strings.Join(parts, " ")
}

// whereBuilder helps build WHERE clauses
type whereBuilder struct {
	conditions []whereCondition
}

type whereCondition struct {
	operator  string // AND or OR
	condition string
}

func (w *whereBuilder) addCondition(condition string, args []any, base *baseQuery) {
	// Replace ? placeholders with named parameters
	processedCondition := condition
	for _, arg := range args {
		paramName := base.generateParamName("param")
		processedCondition = strings.Replace(processedCondition, "?", "$"+paramName, 1)
		base.addParam(paramName, arg)
	}

	w.conditions = append(w.conditions, whereCondition{
		operator:  "AND",
		condition: processedCondition,
	})
}

func (w *whereBuilder) addRawCondition(condition string) {
	w.conditions = append(w.conditions, whereCondition{
		operator:  "AND",
		condition: condition,
	})
}

func (w *whereBuilder) hasConditions() bool {
	return len(w.conditions) > 0
}

func (w *whereBuilder) build() string {
	if len(w.conditions) == 0 {
		return ""
	}

	var parts []string
	for i, cond := range w.conditions {
		if i == 0 {
			parts = append(parts, cond.condition)
		} else {
			parts = append(parts, cond.operator+" "+cond.condition)
		}
	}

	return strings.Join(parts, " ")
}
