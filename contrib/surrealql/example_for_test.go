package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

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
	// };
	// Var param_1: [Tobie Jaime]
}
