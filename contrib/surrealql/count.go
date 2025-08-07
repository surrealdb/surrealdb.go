package surrealql

import "fmt"

// CountQuery represents a COUNT query builder
type CountQuery struct {
	*SelectQuery
}

// Count creates a new COUNT query builder
// If no field is specified, it counts all records (COUNT()).
// If a field is specified, it counts non-null values in that field (COUNT(field)).
// Note that there is no `COUNT(*)` in SurrealQL, so you should use `COUNT()` for counting all.
//
// Deprecated: Use `Select()` instead, e.g. `Select("name", "count()")`.
// This function is here to demonstrate that this isn't a right abstaction,
// because you cannot specify non-aggregated fields in the SELECT clause
// when using COUNT.
func Count[T selectField](fields ...T) *CountQuery {
	if len(fields) == 0 {
		return &CountQuery{
			SelectQuery: Select("count()"),
		}
	}

	q := &CountQuery{
		SelectQuery: Select(fields[0], fields[1:]...),
	}
	for i, field := range fields {
		f := F(field)
		f.expr = fmt.Sprintf("count(%s)", f.expr)

		f = f.As(fmt.Sprintf("count_%d", i))

		q.Field(f)
	}

	return q
}

// As adds an alias to the count result
func (q *CountQuery) As(alias string) *CountQuery {
	if len(q.fields) > 0 {
		q.fields[0] = q.fields[0] + " AS " + alias
	}
	return q
}

// Additional helper methods for common count patterns

// CountGroupBy creates a count query with grouping
//
// Deprecated: Use `Select()` with `GroupBy()` instead,
// e.g. `Select("category", "count() AS count").FromTable("products").GroupBy("category")`.
// This function is here to demonstrate that this isn't a right abstraction,
// because you cannot specify non-aggregated fields in the SELECT clause
// when using COUNT with GROUP BY.
// It is recommended to use `Select()` for more flexibility.
func CountGroupBy(groupField string, groupFields ...string) *CountQuery {
	// Build the field list: group fields + count()
	fields := make([]string, len(groupFields)+2)
	fields[0] = groupField
	copy(fields[1:], groupFields)
	fields[len(fields)-1] = "count() AS count"

	query := &CountQuery{
		SelectQuery: Select(fields[0], fields[1:]...),
	}
	query.GroupBy(append([]string{groupField}, groupFields...)...)

	return query
}
