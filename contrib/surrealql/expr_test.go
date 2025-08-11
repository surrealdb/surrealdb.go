package surrealql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildExpr(expr *expr) (sql string, vars map[string]any) {
	c := newQueryBuildContext()
	sql = expr.Build(&c)
	return sql, c.vars
}

func TestExpr_string(t *testing.T) {
	sql, vars := buildExpr(Expr("price"))
	assert.Equal(t, "price", sql)
	assert.Empty(t, vars)
}

func TestExpr_stringAlias(t *testing.T) {
	sql, vars := buildExpr(Expr("price").As("cost"))
	assert.Equal(t, "price AS cost", sql)
	assert.Empty(t, vars)
}

func TestExpr_selectQuery(t *testing.T) {
	sql, vars := buildExpr(Expr(Select("products").Where("category = ?", "electronics")))
	assert.Equal(t, "(SELECT * FROM products WHERE category = $param_1)", sql)
	assert.Equal(t, map[string]any{"param_1": "electronics"}, vars)
}

func TestExpr_selectQueryAlias(t *testing.T) {
	sql, vars := buildExpr(Expr(Select("products").Where("category = ?", "electronics")).As("product_list"))
	assert.Equal(t, "(SELECT * FROM products WHERE category = $param_1) AS product_list", sql)
	assert.Equal(t, map[string]any{"param_1": "electronics"}, vars)
}

func TestExpr_fnArgFromField(t *testing.T) {
	sql, vars := buildExpr(Expr("math::sum(amount)"))
	assert.Equal(t, "math::sum(amount)", sql)
	assert.Empty(t, vars)
}

func TestExpr_fnArgFromFieldAlias(t *testing.T) {
	sql, vars := buildExpr(Expr("math::sum(amount)").As("total_amount"))
	assert.Equal(t, "math::sum(amount) AS total_amount", sql)
	assert.Empty(t, vars)
}

func TestExpr_fnArgFromValue(t *testing.T) {
	sql, vars := buildExpr(Expr("math::sum(?)", []int{100}))
	assert.Equal(t, "math::sum($fn_math_sum_1)", sql)
	assert.Equal(t, map[string]any{"fn_math_sum_1": []int{100}}, vars)
}

func TestExpr_fnArgFromValueAlias(t *testing.T) {
	sql, vars := buildExpr(Expr("math::sum(?)", []int{100}).As("total_amount"))
	assert.Equal(t, "math::sum($fn_math_sum_1) AS total_amount", sql)
	assert.Equal(t, map[string]any{"fn_math_sum_1": []int{100}}, vars)
}
