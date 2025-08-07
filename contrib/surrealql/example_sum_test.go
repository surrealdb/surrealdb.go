package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleSelect_aggregate() {
	// Using various aggregate functions

	// Total revenue
	sumQuery := surrealql.Select("math::sum(amount)").
		FromTable("orders").
		Where("status = ?", "completed")

	// Average rating
	avgQuery := surrealql.Select("math::mean(rating)").
		FromTable("reviews").
		Where("product_id = ?", "product:123")

	// Price range
	minQuery := surrealql.Select("math::min(price)").
		FromTable("products").
		Where("category = ?", "electronics")

	maxQuery := surrealql.Select("math::max(price)").
		FromTable("products")

	fmt.Println("Sum:", sumQuery.String())
	fmt.Println("Avg:", avgQuery.String())
	fmt.Println("Min:", minQuery.String())
	fmt.Println("Max:", maxQuery.String())

	// Output:
	// Sum: SELECT math::sum(amount) FROM orders WHERE status = $param_1
	// Avg: SELECT math::mean(rating) FROM reviews WHERE product_id = $param_1
	// Min: SELECT math::min(price) FROM products WHERE category = $param_1
	// Max: SELECT math::max(price) FROM products
}
