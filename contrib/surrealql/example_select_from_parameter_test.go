package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelectFrom_parameterRules demonstrates SurrealDB's parameter rules in FROM clauses
func ExampleSelectFrom_parameterRules() {
	// Rule 1: Parameters ARE allowed at the START of FROM expressions
	user := models.NewRecordID("users", "john")
	sql1, vars1 := surrealql.SelectFrom("?->knows->users", user).Build()
	fmt.Println("Start position:")
	fmt.Println(sql1)
	fmt.Printf("Has parameter: %v\n", len(vars1) > 0)

	// Rule 2: Placeholders in middle/end also generate parameters
	// SurrealDB will reject these if it doesn't allow them there
	sql2, vars2 := surrealql.SelectFrom("users->?->?", "knows", "users").Build()
	fmt.Println("\nMiddle/end positions:")
	fmt.Println(sql2)
	fmt.Printf("Has parameter: %v\n", len(vars2) > 0)

	// Rule 3: First placeholder can be parameter, subsequent ones are inlined
	sql3, vars3 := surrealql.SelectFrom("?->?->?",
		models.NewRecordID("users", "alice"),
		"follows",
		"posts").Build()
	fmt.Println("\nMixed positions:")
	fmt.Println(sql3)
	fmt.Printf("Parameter count: %d\n", len(vars3))

	// Output: Start position:
	// SELECT * FROM $from_param_1->knows->users
	// Has parameter: true
	//
	// Middle/end positions:
	// SELECT * FROM users->$from_param_1->$from_param_2
	// Has parameter: true
	//
	// Mixed positions:
	// SELECT * FROM $from_param_1->$from_param_2->$from_param_3
	// Parameter count: 3
}
