package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleFor_insideTransaction() {
	createUser := surrealql.Create("user").Set(`id = type::thing("user", $name)`).Set("name = $name")
	createUsers := surrealql.For("name", `["Tobie", "Jaime"]`).Query(createUser)
	tx := surrealql.Begin().Query(createUsers)
	sql, vars := tx.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// BEGIN TRANSACTION;
	// FOR $name IN ["Tobie", "Jaime"] {
	// CREATE user SET id = type::thing("user", $name), name = $name;
	// };
	// COMMIT TRANSACTION;
}

func ExampleForStatement_goSliceAsIterable() {
	createUser := surrealql.Create("type::thing('person', $name)").Set("name = $name")

	statement := surrealql.For("name", "?", []any{"Tobie", "Jaime"}).
		Query(createUser)

	sql, vars := statement.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// FOR $name IN $param_1 {
	// CREATE type::thing('person', $name) SET name = $name;
	// }
	// Var param_1: [Tobie Jaime]
}

func ExampleForStatement_subqueryAsIterable() {
	subquery := surrealql.Select("person").Value("id").Where("age >= ?", 18)

	createUser := surrealql.Update("$person").Set("can_vote = true")

	statement := surrealql.For("person", subquery).
		Query(createUser)

	sql, vars := statement.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}

	// Output:
	// FOR $person IN (SELECT VALUE id FROM person WHERE age >= $param_1) {
	// UPDATE $person SET can_vote = true;
	// }
	// Var param_1: 18
}
