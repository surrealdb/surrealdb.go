package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// See https://github.com/surrealdb/surrealdb.go/issues/291
func ExampleQuery_issue291() {
	db := testenv.MustNew("surrealdbexamples", "query_issue291", "t")

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

	// With fxamacker/cbor: returns zero values (0, "")
	// With surrealcbor: returns nil
	// We need to handle both cases
	if got.I == nil || *got.I == 0 {
		fmt.Printf("I: <nil or zero>\n")
	} else {
		fmt.Printf("I: %+v\n", *got.I)
	}

	if got.J == nil || *got.J == "" {
		fmt.Printf("J: <nil or zero>\n")
	} else {
		fmt.Printf("J: %q\n", *got.J)
	}

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
	// I: <nil or zero>
	// J: <nil or zero>
	// I: <nil>
	// J: <nil>
}
