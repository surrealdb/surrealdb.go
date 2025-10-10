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
	// CREATE users:123 SET name = $param_1;
	// UPDATE users:123 SET email = $param_2;
	// COMMIT TRANSACTION;
	// Var param_1: Alice
	// Var param_2: alice@example.com
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
	updateAccount1 := surrealql.Raw("UPDATE account:one SET balance += 300.00")
	updateAccount2 := surrealql.Raw("UPDATE account:two SET balance -= 300.00")

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

	sql, vars := tx.Build()
	fmt.Println(sql)

	keys := sort.StringSlice(slices.Collect(maps.Keys(vars)))
	sort.Stable(keys)
	for _, key := range keys {
		fmt.Printf("Var %s: %v\n", key, vars[key])
	}
	// Output:
	// BEGIN TRANSACTION;
	// CREATE account:one SET balance = $param_1;
	// CREATE account:two SET balance = $param_2;
	// IF !account:two.wants_to_send_money {
	//     THROW "Customer doesn't want to send any money!";
	// };
	// UPDATE account:one SET balance += 300.00;
	// UPDATE account:two SET balance -= 300.00;
	// COMMIT TRANSACTION;
	// Var param_1: 135605.16
	// Var param_2: 91031.31
}

func ExampleTransactionQuery_LetTyped() {
	// Create a transaction with typed LET statements
	tx := surrealql.Begin().
		LetTyped("num", "int | string", "9").
		LetTyped("vals", "array<bool>", surrealql.Raw("some:record.vals.map(|$val| <bool>$val)")).
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

// ExampleTransactionQuery_Return demonstrates how to set the result of a transaction using RETURN.
// This is based on https://surrealdb.com/docs/surrealql/statements/return#transaction-return-value
func ExampleTransactionQuery_Return() {
	// Create a transaction that returns a specific value
	tx := surrealql.Begin().
		Let("name", "Alice").
		Let("email", "alice@example.com").
		Query(surrealql.Create("person").
			Set("name", surrealql.Var("name")).
			Set("email", surrealql.Var("email"))).
		Return("$name")

	sql, _ := tx.Build()
	fmt.Println(sql)
	// Output:
	// BEGIN TRANSACTION;
	// LET $name = "Alice";
	// LET $email = "alice@example.com";
	// CREATE person SET name = $name, email = $email;
	// RETURN $name;
	// COMMIT TRANSACTION;
}

// ExampleTransactionQuery_Return_withPlaceholders demonstrates using RETURN with placeholders
func ExampleTransactionQuery_Return_withPlaceholders() {
	// Create a transaction that returns a computed value
	tx := surrealql.Begin().
		Let("a", 10).
		Let("b", 20).
		Query(surrealql.Create(surrealql.Table("test")).Set("v", "V")).
		Return("? + ? + ?", surrealql.Var("a"), surrealql.Var("b"), 5)

	sql, vars := tx.Build()
	fmt.Println(sql)
	dumpVars(vars)
	// Output:
	// BEGIN TRANSACTION;
	// LET $a = 10;
	// LET $b = 20;
	// CREATE $table_1 SET v = $param_1;
	// RETURN $a + $b + $return_param_1;
	// COMMIT TRANSACTION;
	// Vars:
	//   param_1: V
	//   return_param_1: 5
	//   table_1: test
}
