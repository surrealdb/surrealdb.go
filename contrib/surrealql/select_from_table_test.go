package surrealql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestSelect_fromTable(t *testing.T) {
	t.Run("normal_table_name", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("users")).Build()
		assert.Equal(t, "SELECT * FROM $from_table_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("users"), vars["from_table_1"])
	})

	t.Run("table_with_special_chars", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("user-data")).Build()
		assert.Equal(t, "SELECT * FROM $from_table_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("user-data"), vars["from_table_1"])
	})

	t.Run("reserved_word_table", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("select")).Build()
		assert.Equal(t, "SELECT * FROM $from_table_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("select"), vars["from_table_1"])
	})

	t.Run("with_fields", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("products")).
			FieldName("name").
			FieldName("price").
			Build()
		assert.Equal(t, "SELECT name, price FROM $from_table_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("products"), vars["from_table_1"])
	})

	t.Run("with_where_clause", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("users")).
			Where("age > ?", 18).
			Build()
		assert.Equal(t, "SELECT * FROM $from_table_1 WHERE age > $param_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("users"), vars["from_table_1"])
		assert.Contains(t, vars, "param_1")
		assert.Equal(t, 18, vars["param_1"])
	})

	t.Run("table_with_underscore", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("user_accounts")).Build()
		assert.Equal(t, "SELECT * FROM $from_table_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("user_accounts"), vars["from_table_1"])
	})

	t.Run("table_with_multiple_special_chars", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("user-data-2024")).Build()
		assert.Equal(t, "SELECT * FROM $from_table_1", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("user-data-2024"), vars["from_table_1"])
	})

	t.Run("complete_query", func(t *testing.T) {
		sql, vars := surrealql.Select(models.Table("user-profiles")).
			FieldName("name").
			FieldName("email").
			Where("active = ?", true).
			OrderBy("created_at").
			Limit(10).
			Build()
		assert.Equal(t, "SELECT name, email FROM $from_table_1 WHERE active = $param_1 ORDER BY created_at LIMIT 10", sql)
		assert.Contains(t, vars, "from_table_1")
		assert.Equal(t, models.Table("user-profiles"), vars["from_table_1"])
		assert.Contains(t, vars, "param_1")
		assert.Equal(t, true, vars["param_1"])
	})
}
