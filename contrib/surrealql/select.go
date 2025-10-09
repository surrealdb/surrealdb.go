package surrealql

import (
	"fmt"
	"strings"
)

// SelectQuery represents a SELECT query builder
type SelectQuery struct {
	fields      []*expr
	omits       []string
	from        []*expr
	only        bool
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

// Select creates a new SELECT query starting with the FROM clause.
// This allows for more natural query building where you first specify what to select from,
// then optionally add fields. If no fields are specified, it defaults to SELECT *.
//
// The target can be:
//   - A string for raw expressions: "users", "users:123", or any SurrealQL expression WITHOUT "?" placeholders
//   - A models.Table for safe table name specification
//   - A *models.RecordID for specific records
//   - A *SelectQuery for subqueries
//   - A *expr for expressions
//     A *expr can be created using surrealql.Expr which supports placeholders with "?" that will be replaced by args
//   - For arrays and objects, use Select(Expr("?", myArray)) or Select(Expr("?", myMap))
//
// Examples:
//
//	// Select all from a table
//	Select("users")  // SELECT * FROM users
//
//	// Select from a specific record
//	Select("users:123")  // SELECT * FROM users:123
//
//	// Select from a parameterized graph traversal
//	Select(Expr("?->knows->users", models.NewRecordID("users", "john")))
//	// SELECT * FROM $from_param_1->knows->users
//
//	// Select from a subquery
//	subquery := Select("users").Fields("name")
//	Select(subquery)  // SELECT * FROM (SELECT name FROM users)
//
//	// Select a record using models.RecordID
//	Select(models.NewRecordID("users", 123)) // SELECT * from $from_id_1
//
//	// Select from a table using models.Table for type safety
//	Select(models.Table("users"))  // SELECT * FROM $table_1
//
//	// Select from an array or object using placeholders
//	Select(Expr("?"), []any{1, 2, 3})  // SELECT * FROM $from_param_1
//	Select(Expr("?"), map[string]any{"a": 1})  // SELECT * FROM $from_param_1
func Select[T exprLike](targets ...T) *SelectQuery {
	if len(targets) > 0 {
		if str, ok := any(targets[0]).(string); ok && strings.Contains(str, "?") {
			var args []any
			for _, target := range targets[1:] {
				args = append(args, target)
			}
			return &SelectQuery{
				from: []*expr{Expr(targets[0], args...)},
			}
		}
	}

	var ts []*expr
	for _, target := range targets {
		ts = append(ts, Expr(target))
	}
	return &SelectQuery{
		from: ts,
	}
}

// SelectOnly creates a new SELECT ONLY query starting with the FROM clause.
//
// See [Select] for details on the target parameter, general usage, and examples.
func SelectOnly[T exprLike](targets ...T) *SelectQuery {
	q := Select(targets...)
	q.only = true
	return q
}

// orderByClause represents an ORDER BY clause
type orderByClause struct {
	field   string
	desc    bool
	collate bool
	numeric bool
}

// Value turns this query into a `SELECT VALUE field FROM ...` query.
// It is used to select a single value per each record.
func (q *SelectQuery) Value(field any, args ...any) *SelectQuery {
	q2 := q.Field(field, args...)
	q2.value = true
	return q2
}

func (q *SelectQuery) Fields(rawFieldExprs ...any) *SelectQuery {
	for _, r := range rawFieldExprs {
		switch v := r.(type) {
		case string:
			q.fields = append(q.fields, Expr(v))
		case *SelectQuery:
			q.fields = append(q.fields, Expr(v))
		case *expr:
			q.fields = append(q.fields, v)
		default:
			// Unsupported field type
			panic(fmt.Sprintf("unsupported field type: %T", v))
		}
	}
	return q
}

// Alias adds an aliased field to the SELECT query.
//
// An alternative to calling this `Alias(field, args...)` is to call:
//
//	q.Field(Expr(field, args...).As(alias))
//
// See [Expr] for details on the field parameter.
func (q *SelectQuery) Alias(alias string, field any, args ...any) *SelectQuery {
	return q.Field(fieldExpr(field, args...).As(alias))
}

// Field adds a field to the SELECT query.
//
// This is a more general version of [FieldName] and [FieldNameAs].
// It can accept various types of field expressions such as strings, subqueries, and expressions.
//
// An alternative to calling this `Field(field, args...)` is to call:
//
//	q.Field(Expr(field, args...))
//
// See [Expr] for details on the field parameter.
//
// See [Alias] for adding aliased fields.
func (q *SelectQuery) Field(field any, args ...any) *SelectQuery {
	q.fields = append(q.fields, fieldExpr(field, args...))
	return q
}

func fieldExpr(field any, args ...any) *expr {
	switch v := field.(type) {
	case string:
		return Expr(v, args...)
	case *SelectQuery:
		return Expr(v)
	case *expr:
		return v
	default:
		// Unsupported field type
		panic(fmt.Sprintf("unsupported field type: %T", v))
	}
}

// FieldName adds a field to the SELECT query.
func (q *SelectQuery) FieldName(field string) *SelectQuery {
	escaped := escapeIdent(field)
	ex := Expr(escaped)
	q.fields = append(q.fields, ex)
	return q
}

// FieldNameAs adds a field with an alias to the SELECT query.
func (q *SelectQuery) FieldNameAs(field, alias string) *SelectQuery {
	escaped := escapeIdent(field)
	ex := Expr(escaped)
	aliased := ex.As(alias)
	q.fields = append(q.fields, aliased)
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

// Where adds a WHERE condition to the query.
//
// All values are automatically parameterized to prevent injection:
//
//	query := surrealql.Select("users").
//	    Where("age > ? AND status = ?", 18, "active")
//	// Generates: SELECT * FROM users WHERE age > $param_1 AND status = $param_2
func (q *SelectQuery) Where(condition string, args ...any) *SelectQuery {
	if q.whereClause == nil {
		q.whereClause = &whereBuilder{}
	}
	q.whereClause.addCondition(condition, args)
	return q
}

// WhereEq adds a WHERE equality condition
func (q *SelectQuery) WhereEq(field string, value any) *SelectQuery {
	return q.Where("type::field(?) = ?", field, value)
}

// WhereIn adds a WHERE IN condition
func (q *SelectQuery) WhereIn(field string, values any) *SelectQuery {
	return q.Where("type::field(?) IN ?", field, values)
}

// WhereNotNull adds a WHERE IS NOT NULL condition
func (q *SelectQuery) WhereNotNull(field string) *SelectQuery {
	return q.Where("type::field(?) IS NOT NULL", field)
}

// WhereNull adds a WHERE IS NULL condition
func (q *SelectQuery) WhereNull(field string) *SelectQuery {
	return q.Where("type::field(?) IS NULL", field)
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
	c := newQueryBuildContext()
	return q.build(&c), c.vars
}

func (q *SelectQuery) build(c *queryBuildContext) (sql string) {
	var parts []string

	// Add EXPLAIN if enabled
	if q.explain {
		parts = append(parts, "EXPLAIN")
	}

	// SELECT clause
	parts = append(parts, q.buildSelectClause(c))

	// FROM clause
	if len(q.from) > 0 {
		fromClauses := make([]string, len(q.from))
		for i, f := range q.from {
			fromClauses[i] = f.build(c.in("from"))
		}
		from := "FROM "
		if q.only {
			from += "ONLY "
		}
		parts = append(parts, from+strings.Join(fromClauses, ", "))
	}

	// WHERE clause
	if q.whereClause != nil && q.whereClause.hasConditions() {
		parts = append(parts, "WHERE "+q.whereClause.build(c))
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

// buildSelectClause builds the SELECT clause
func (q *SelectQuery) buildSelectClause(c *queryBuildContext) string {
	var b strings.Builder

	b.WriteString("SELECT ")

	if q.value {
		b.WriteString("VALUE ")
	}

	if len(q.fields) == 0 {
		b.WriteString("*")
	} else {
		for i, field := range q.fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(field.build(c))
		}
	}

	for i, omit := range q.omits {
		if i == 0 {
			b.WriteString(" OMIT ")
		} else {
			b.WriteString(", ")
		}
		b.WriteString(omit)
	}

	return b.String()
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
	sql, _ := q.Build()
	return sql
}

// whereBuilder helps build WHERE clauses
type whereBuilder struct {
	conditions []whereCondition
}

// whereCondition is an unprocessed WHERE condition
// which is processed when build(*queryBuildContext) is called.
type whereCondition struct {
	condition string
	args      []any
}

func (w *whereBuilder) addCondition(condition string, args []any) {
	w.conditions = append(w.conditions, whereCondition{
		condition: condition,
		args:      args,
	})
}

func (w *whereBuilder) hasConditions() bool {
	return len(w.conditions) > 0
}

func (w *whereBuilder) build(c *queryBuildContext) string {
	if len(w.conditions) == 0 {
		return ""
	}

	var parts []string
	for i, cond := range w.conditions {
		// Replace ? placeholders with named parameters
		processedCondition := cond.condition
		for _, arg := range cond.args {
			paramName := c.generateAndAddParam("param", arg)
			processedCondition = strings.Replace(processedCondition, "?", "$"+paramName, 1)
		}

		if i == 0 {
			parts = append(parts, processedCondition)
		} else {
			parts = append(parts, "AND "+processedCondition)
		}
	}

	return strings.Join(parts, " ")
}
