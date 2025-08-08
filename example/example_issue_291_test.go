package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// See https://github.com/surrealdb/surrealdb.go/issues/291
func ExampleQuery_issue291() {
	db := testenv.MustNew("example", "query_issue291", "t")

	_, err := surrealdb.Query[any](
		context.Background(),
		db,
		`DEFINE TABLE IF NOT EXISTS t SCHEMAFULL;
DEFINE FIELD IF NOT EXISTS i ON TABLE t TYPE option<int>;
DEFINE FIELD IF NOT EXISTS j ON TABLE t TYPE option<string>;
CREATE t:s;`,
		map[string]any{"name": "John Doe"},
	)
	if err != nil {
		panic(err)
	}

	type ReturnData struct {
		I *int    `json:"i"`
		J *string `json:"j"`
	}

	dataNones, err := surrealdb.Query[[]ReturnData](
		context.Background(),
		db,
		"SELECT i, j FROM t",
		nil)
	if err != nil {
		panic(err)
	}

	got := (*dataNones)[0].Result[0]

	fmt.Printf("I: %+v\n", *got.I)
	fmt.Printf("J: %q\n", *got.J)

	dataAll, err := surrealdb.Query[[]ReturnData](
		context.Background(),
		db,
		"SELECT * FROM t",
		nil)
	if err != nil {
		panic(err)
	}

	gotAll := (*dataAll)[0].Result[0]

	fmt.Printf("I: %+v\n", gotAll.I)
	fmt.Printf("J: %+v\n", gotAll.J)

	// Output:
	// I: 0
	// J: ""
	// I: <nil>
	// J: <nil>
}
