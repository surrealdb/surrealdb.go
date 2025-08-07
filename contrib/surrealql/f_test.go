package surrealql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestF_string(t *testing.T) {
	sql, vars := F("price").Build()
	assert.Equal(t, "price", sql)
	assert.Empty(t, vars)
}

func TestF_stringAlias(t *testing.T) {
	sql, vars := F("price").As("cost").Build()
	assert.Equal(t, "price AS cost", sql)
	assert.Empty(t, vars)
}

func TestF_selectQuery(t *testing.T) {
	sql, vars := F(Select("*").FromTable("products").Where("category = ?", "electronics")).Build()
	assert.Equal(t, "(SELECT * FROM products WHERE category = $param_1)", sql)
	assert.Equal(t, map[string]any{"param_1": "electronics"}, vars)
}

func TestF_selectQueryAlias(t *testing.T) {
	sql, vars := F(Select("*").FromTable("products").Where("category = ?", "electronics")).As("product_list").Build()
	assert.Equal(t, "(SELECT * FROM products WHERE category = $param_1) AS product_list", sql)
	assert.Equal(t, map[string]any{"param_1": "electronics"}, vars)
}

func TestF_fnArgFromField(t *testing.T) {
	sql, vars := F(Fn("math::sum").ArgFromField("amount")).Build()
	assert.Equal(t, "math::sum(amount)", sql)
	assert.Empty(t, vars)
}

func TestF_fnArgFromFieldAlias(t *testing.T) {
	sql, vars := F(Fn("math::sum").ArgFromField("amount")).As("total_amount").Build()
	assert.Equal(t, "math::sum(amount) AS total_amount", sql)
	assert.Empty(t, vars)
}

func TestF_fnArgFromValue(t *testing.T) {
	sql, vars := F(Fn("math::sum").ArgFromValue([]int{100})).Build()
	assert.Equal(t, "math::sum($fn_math_sum_0)", sql)
	assert.Equal(t, map[string]any{"fn_math_sum_0": []int{100}}, vars)
}

func TestF_fnArgFromValueAlias(t *testing.T) {
	sql, vars := F(Fn("math::sum").ArgFromValue([]int{100})).As("total_amount").Build()
	assert.Equal(t, "math::sum($fn_math_sum_0) AS total_amount", sql)
	assert.Equal(t, map[string]any{"fn_math_sum_0": []int{100}}, vars)
}
