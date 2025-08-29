package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleQuery_transactionRollback demonstrates that mutations within a rolled back transaction don't persist.
// The CANCEL statement rolls back all changes made within the transaction.
func ExampleQuery_transactionRollback() {
	config := testenv.MustNewConfig("surrealdbexamples", "transaction_rollback", "accounts")
	config.Endpoint = testenv.GetSurrealDBURL()

	db := config.MustNew()

	type Account struct {
		ID      *models.RecordID `json:"id,omitempty"`
		Name    string           `json:"name"`
		Balance float64          `json:"balance"`
	}

	ctx := context.Background()

	// First, create an initial account outside of any transaction
	initialAccount, err := surrealdb.Create[Account](ctx, db, "accounts", map[string]any{
		"name":    "Savings Account",
		"balance": 1000.00,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create initial account: %v", err))
	}

	fmt.Printf("Initial account created: %s with balance %.2f\n", initialAccount.Name, initialAccount.Balance)

	// Now start a transaction that will be rolled back
	transactionQuery := `
		BEGIN TRANSACTION;

		-- Create a new account within the transaction
		CREATE accounts SET name = $checkingName, balance = $checkingBalance;

		-- Update the existing account within the transaction
		UPDATE $accountID SET balance = $newBalance;

		-- Create another account
		CREATE accounts SET name = $investmentName, balance = $investmentBalance;

		-- Roll back all changes made in this transaction
		CANCEL TRANSACTION;
	`

	_, err = surrealdb.Query[any](ctx, db, transactionQuery, map[string]any{
		"accountID":         initialAccount.ID,
		"checkingName":      "Checking Account",
		"checkingBalance":   500.00,
		"newBalance":        2000.00,
		"investmentName":    "Investment Account",
		"investmentBalance": 5000.00,
	})
	// When a transaction is canceled, SurrealDB returns an error
	if err != nil {
		fmt.Println("Transaction was rolled back (as expected)")
	} else {
		panic("Expected an error from canceled transaction, but got none")
	}

	// Verify that no new accounts were created
	allAccounts, err := surrealdb.Select[[]Account](ctx, db, "accounts")
	if err != nil {
		panic(fmt.Sprintf("Failed to select accounts: %v", err))
	}

	fmt.Printf("Number of accounts after rollback: %d\n", len(*allAccounts))

	// Verify that the original account balance wasn't changed
	updatedAccount, err := surrealdb.Select[Account](ctx, db, *initialAccount.ID)
	if err != nil {
		panic(fmt.Sprintf("Failed to select account: %v", err))
	}

	fmt.Printf("Original account balance after rollback: %.2f\n", updatedAccount.Balance)

	// Now demonstrate a committed transaction for comparison
	committedTransactionQuery := `
		BEGIN TRANSACTION;

		-- Create a new account within the transaction
		CREATE accounts SET name = $businessName, balance = $businessBalance;

		-- Commit the transaction
		COMMIT TRANSACTION;
	`

	_, err = surrealdb.Query[any](ctx, db, committedTransactionQuery, map[string]any{
		"businessName":    "Business Account",
		"businessBalance": 3000.00,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to execute committed transaction: %v", err))
	}

	fmt.Println("Second transaction executed and committed")

	// Verify that the new account was created
	finalAccounts, err := surrealdb.Select[[]Account](ctx, db, "accounts")
	if err != nil {
		panic(fmt.Sprintf("Failed to select accounts: %v", err))
	}

	fmt.Printf("Number of accounts after commit: %d\n", len(*finalAccounts))

	// Output:
	// Initial account created: Savings Account with balance 1000.00
	// Transaction was rolled back (as expected)
	// Number of accounts after rollback: 1
	// Original account balance after rollback: 1000.00
	// Second transaction executed and committed
	// Number of accounts after commit: 2
}
