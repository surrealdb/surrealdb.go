package surrealql_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleBegin() {
	// Create a simple transaction
	tx := surrealql.Begin().
		Let("transfer_amount", 300.00).
		Raw("UPDATE account:one SET balance += $transfer_amount").
		Raw("UPDATE account:two SET balance -= $transfer_amount")

	sql, _ := tx.Build()
	fmt.Println(sql)
	// Output:
	// BEGIN TRANSACTION;
	// LET $transfer_amount = 300;
	// UPDATE account:one SET balance += $transfer_amount;
	// UPDATE account:two SET balance -= $transfer_amount;
	// COMMIT TRANSACTION;
}
