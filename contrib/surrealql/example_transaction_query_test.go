package surrealql_test

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleTransactionQuery_Query() {
	// Create a transaction with multiple query builders
	createUser := surrealql.Create("users:123").Set("name", "Alice")
	updateUser := surrealql.Update("users:123").Set("email", "alice@example.com")

	tx := surrealql.Begin().
		Query(createUser).
		Query(updateUser)

	sql, vars := tx.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}
	// Output:
	// BEGIN TRANSACTION;
	// CREATE users:123 SET name = $name_1;
	// UPDATE users:123 SET email = $email_1;
	// COMMIT TRANSACTION;
	// Var email_1: alice@example.com
	// Var email_2: alice@example.com
	// Var name_1: Alice
	// Var name_2: Alice
}

func ExampleTransactionQuery_If() {
	// Create a transaction with conditional logic
	tx := surrealql.Begin().
		Let("transfer_amount", 300.00).
		Raw("UPDATE account:one SET dollars -= $transfer_amount").
		If("account:one.dollars < 0").
		Then(func(tb *surrealql.ThenBuilder) {
			// TODO: Fix this so that it becomes:
			//   THROW "Insufficient funds, would have $" + <string>account:one.dollars;
			// Instead of:
			//   THROW "Insufficient funds, would have $\" + <string>account:one.dollars";
			tb.Throw("Insufficient funds, would have $\" + <string>account:one.dollars")
		}).
		End()

	sql, _ := tx.Build()
	fmt.Println(sql)
	// Output:
	// BEGIN TRANSACTION;
	// LET $transfer_amount = 300;
	// UPDATE account:one SET dollars -= $transfer_amount;
	// IF account:one.dollars < 0 {
	//     THROW "Insufficient funds, would have $\" + <string>account:one.dollars";
	// };
	// COMMIT TRANSACTION;
}

// ExampleTransactionQuery_returningEarly demonstrates how to create a transaction with multiple query builders.
// Note that this example is derived from https://surrealdb.com/docs/surrealql/statements/begin#returning-early-from-a-transaction.
func ExampleTransactionQuery_returningEarly() {
	// Create a transaction using existing query builders
	createAccount1 := surrealql.Create("account:one").Set("balance", 135605.16)
	createAccount2 := surrealql.Create("account:two").Set("balance", 91031.31)
	updateAccount1 := surrealql.Raw("UPDATE account:one SET balance += 300.00", nil)
	updateAccount2 := surrealql.Raw("UPDATE account:two SET balance -= 300.00", nil)

	tx := surrealql.Begin().
		Query(createAccount1).
		Query(createAccount2).
		If("!account:two.wants_to_send_money").
		Then(func(tb *surrealql.ThenBuilder) {
			tb.Throw("Customer doesn't want to send any money!")
		}).
		End().
		Query(updateAccount1).
		Query(updateAccount2)

	sql, _ := tx.Build()
	fmt.Println(sql)
	// Output:
	// BEGIN TRANSACTION;
	// CREATE account:one SET balance = $balance_1;
	// CREATE account:two SET balance = $balance_1;
	// IF !account:two.wants_to_send_money {
	//     THROW "Customer doesn't want to send any money!";
	// };
	// UPDATE account:one SET balance += 300.00;
	// UPDATE account:two SET balance -= 300.00;
	// COMMIT TRANSACTION;
}

func ExampleTransactionQuery_LetTyped() {
	// Create a transaction with typed LET statements
	tx := surrealql.Begin().
		LetTyped("num", "int | string", "9").
		LetTyped("vals", "array<bool>", surrealql.Raw("some:record.vals.map(|$val| <bool>$val)", nil)).
		Raw("CREATE thing SET number = $num, values = $vals")

	sql, _ := tx.Build()
	fmt.Println(sql)
	// Output:
	// BEGIN TRANSACTION;
	// LET $num: int | string = "9";
	// LET $vals: array<bool> = (some:record.vals.map(|$val| <bool>$val));
	// CREATE thing SET number = $num, values = $vals;
	// COMMIT TRANSACTION;
}
