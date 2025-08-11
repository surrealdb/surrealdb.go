package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

// ExampleVar demonstrates the difference between Var (variable reference) and string literals
func ExampleVar() {
	// Using V() for variable reference
	query1 := surrealql.Create("users").
		Set("name", surrealql.Var("name")). // References the variable $name
		Set("prefix", "$user")              // Literal string "$user"

	sql1, vars1 := query1.Build()
	fmt.Println("With Var:")
	fmt.Println(sql1)
	fmt.Printf("Vars: %v\n", vars1)

	// Output:
	// With Var:
	// CREATE users SET name = $name, prefix = $param_1
	// Vars: map[param_1:$user]
}
