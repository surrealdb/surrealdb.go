package surrealql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestSelectFrom(t *testing.T) {
	t.Run("from_table", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("users").Build()
		assert.Equal(t, "SELECT * FROM users", sql)
		assert.Empty(t, vars)
	})

	t.Run("from_record", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("users:123").Build()
		assert.Equal(t, "SELECT * FROM users:123", sql)
		assert.Empty(t, vars)
	})

	t.Run("from_target", func(t *testing.T) {
		target := surrealql.Thing("users", 123)
		sql, vars := surrealql.SelectFrom(target).Build()
		assert.Equal(t, "SELECT * FROM $id_1", sql)
		assert.Contains(t, vars, "id_1")
		assert.IsType(t, models.RecordID{}, vars["id_1"])
	})

	t.Run("from_models_table", func(t *testing.T) {
		table := models.Table("users")
		sql, vars := surrealql.SelectFrom(table).Build()
		assert.Equal(t, "SELECT * FROM $table_1", sql)
		assert.Contains(t, vars, "table_1")
		assert.IsType(t, models.Table(""), vars["table_1"])
		assert.Equal(t, models.Table("users"), vars["table_1"])
	})

	t.Run("from_models_record_id", func(t *testing.T) {
		recordID := models.NewRecordID("users", "john")
		sql, vars := surrealql.SelectFrom(recordID).Build()
		assert.Equal(t, "SELECT * FROM $record_id_1", sql)
		assert.Contains(t, vars, "record_id_1")
		assert.IsType(t, models.RecordID{}, vars["record_id_1"])
		assert.Equal(t, models.NewRecordID("users", "john"), vars["record_id_1"])
	})

	t.Run("from_models_record_id_pointer", func(t *testing.T) {
		recordID := models.NewRecordID("users", 123)
		sql, vars := surrealql.SelectFrom(&recordID).Build()
		assert.Equal(t, "SELECT * FROM $record_id_1", sql)
		assert.Contains(t, vars, "record_id_1")
		assert.IsType(t, models.RecordID{}, vars["record_id_1"])
		assert.Equal(t, models.NewRecordID("users", 123), vars["record_id_1"])
	})

	t.Run("from_array", func(t *testing.T) {
		arr := []any{1, 2, 3}
		sql, vars := surrealql.SelectFrom("?", arr).Build()
		assert.Equal(t, "SELECT * FROM $from_param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, []any{1, 2, 3}, vars["from_param_1"])
	})

	t.Run("from_object", func(t *testing.T) {
		obj := map[string]any{"a": 1}
		sql, vars := surrealql.SelectFrom("?", obj).Build()
		assert.Equal(t, "SELECT * FROM $from_param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, map[string]any{"a": 1}, vars["from_param_1"])
	})

	t.Run("from_subquery", func(t *testing.T) {
		subquery := surrealql.Select("name").FromTable("users")
		sql, vars := surrealql.SelectFrom(subquery).Build()
		assert.Equal(t, "SELECT * FROM (SELECT name FROM users)", sql)
		assert.Empty(t, vars)
	})

	t.Run("with_fields", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("users").
			FieldName("name").
			FieldName("email").
			Build()
		assert.Equal(t, "SELECT name, email FROM users", sql)
		assert.Empty(t, vars)
	})

	t.Run("with_raw_expression", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("products").
			FieldRaw("price * 1.1 AS price_with_tax").
			Build()
		assert.Equal(t, "SELECT price * 1.1 AS price_with_tax FROM products", sql)
		assert.Empty(t, vars)
	})

	t.Run("with_where_clause", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("users").
			FieldName("name").
			Where("age > ?", 18).
			Build()
		assert.Equal(t, "SELECT name FROM users WHERE age > $param_1", sql)
		assert.Contains(t, vars, "param_1")
		assert.Equal(t, 18, vars["param_1"])
	})

	t.Run("graph_traversal", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("users:john->knows->users").Build()
		assert.Equal(t, "SELECT * FROM users:john->knows->users", sql)
		assert.Empty(t, vars)
	})

	t.Run("with_placeholder", func(t *testing.T) {
		recordID := models.NewRecordID("users", "john")
		sql, vars := surrealql.SelectFrom("?->knows->users", recordID).Build()
		assert.Equal(t, "SELECT * FROM $from_param_1->knows->users", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.IsType(t, models.RecordID{}, vars["from_param_1"])
	})

	t.Run("multiple_placeholders", func(t *testing.T) {
		fromRecord := models.NewRecordID("users", "john")
		toTable := "users"
		sql, vars := surrealql.SelectFrom("?->knows->?", fromRecord, toTable).Build()
		// Both placeholders become parameters
		assert.Equal(t, "SELECT * FROM $from_param_1->knows->$from_param_2", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.IsType(t, models.RecordID{}, vars["from_param_1"])
		assert.Contains(t, vars, "from_param_2")
		assert.Equal(t, "users", vars["from_param_2"])
	})

	t.Run("placeholder_with_table", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("?", "products").Build()
		assert.Equal(t, "SELECT * FROM $from_param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "products", vars["from_param_1"])
	})

	t.Run("placeholder_with_fields_and_where", func(t *testing.T) {
		sql, vars := surrealql.SelectFrom("?", "products").
			FieldName("name").
			FieldName("price").
			Where("price > ?", 100).
			Build()
		assert.Equal(t, "SELECT name, price FROM $from_param_1 WHERE price > $param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "products", vars["from_param_1"])
		assert.Contains(t, vars, "param_1")
		assert.Equal(t, 100, vars["param_1"])
	})

	t.Run("mixed_placeholder_types", func(t *testing.T) {
		// Test with different argument types
		// All placeholders become parameters
		sql, vars := surrealql.SelectFrom("?->?->?",
			models.NewRecordID("users", "john"),
			"knows",
			models.NewRecordID("users", "jane")).Build()
		assert.Equal(t, "SELECT * FROM $from_param_1->$from_param_2->$from_param_3", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.IsType(t, models.RecordID{}, vars["from_param_1"])
		assert.Contains(t, vars, "from_param_2")
		assert.Equal(t, "knows", vars["from_param_2"])
		assert.Contains(t, vars, "from_param_3")
		assert.IsType(t, models.RecordID{}, vars["from_param_3"])
	})

	t.Run("string_at_first_position", func(t *testing.T) {
		// When a string is at the first position, it becomes a parameter
		sql, vars := surrealql.SelectFrom("?->follows->users", "users:alice").Build()
		assert.Equal(t, "SELECT * FROM $from_param_1->follows->users", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "users:alice", vars["from_param_1"])
	})

	t.Run("table_name_with_special_chars", func(t *testing.T) {
		// String at first position becomes a parameter (not escaped)
		sql, vars := surrealql.SelectFrom("?", "user-data").Build()
		assert.Equal(t, "SELECT * FROM $from_param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "user-data", vars["from_param_1"])
	})

	t.Run("table_name_with_reserved_word", func(t *testing.T) {
		// String at first position becomes a parameter (not escaped)
		sql, vars := surrealql.SelectFrom("?", "select").Build()
		assert.Equal(t, "SELECT * FROM $from_param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "select", vars["from_param_1"])
	})

	t.Run("complex_traversal_string", func(t *testing.T) {
		// Complex traversals - all placeholders are parameterized
		// This is an INVALID SurrealQL query- In SurrealQL, you can say `$var->likes->products` but
		// cannot say `$var->$rel->$target`.
		// However, surrealql cannot complain on it because it does not parse `->` expressions.
		// It's SurrealQL's limitation that you cannot place random variables in the middle of a path.
		// It's the surrealql libary's limitation that it does not validate this.
		// So surrealql will produce an invalid query, without panic or error.
		sql, vars := surrealql.SelectFrom("?->?->?",
			"users:admin",
			"manages",
			"projects").Build()
		assert.Equal(t, "SELECT * FROM $from_param_1->$from_param_2->$from_param_3", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "users:admin", vars["from_param_1"])
		assert.Contains(t, vars, "from_param_2")
		assert.Equal(t, "manages", vars["from_param_2"])
		assert.Contains(t, vars, "from_param_3")
		assert.Equal(t, "projects", vars["from_param_3"])
	})

	t.Run("models_table_with_where", func(t *testing.T) {
		table := models.Table("products")
		sql, vars := surrealql.SelectFrom(table).
			FieldName("name").
			FieldName("price").
			Where("price > ?", 100).
			OrderBy("price").
			Build()
		assert.Equal(t, "SELECT name, price FROM $table_1 WHERE price > $param_1 ORDER BY price", sql)
		assert.Contains(t, vars, "table_1")
		assert.Equal(t, models.Table("products"), vars["table_1"])
		assert.Contains(t, vars, "param_1")
		assert.Equal(t, 100, vars["param_1"])
	})
}
