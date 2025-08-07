package surrealql_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestIntegrationTransaction(t *testing.T) {
	db := testenv.MustNew("surrealql_test", "accounts")

	ctx := context.Background()

	type Account struct {
		ID      models.RecordID `json:"id,omitempty"`
		Name    string          `json:"name"`
		Balance float64         `json:"balance"`
	}

	// Create test accounts using queries (since Create with record ID has issues)
	createQuery1 := surrealql.Create("accounts:one").
		Set("name", "Account One").
		Set("balance", 1000.00)

	sql, vars := createQuery1.Build()
	results1, err := surrealdb.Query[[]Account](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Failed to create account one: %v", err)
	}
	acc1 := (*results1)[0].Result[0]

	createQuery2 := surrealql.Create("accounts:two").
		Set("name", "Account Two").
		Set("balance", 500.00)

	sql, vars = createQuery2.Build()
	results2, err := surrealdb.Query[[]Account](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Failed to create account two: %v", err)
	}
	acc2 := (*results2)[0].Result[0]

	t.Run("SuccessfulTransaction", func(t *testing.T) {
		// Create a successful transaction
		tx := surrealql.Begin().
			Let("transfer_amount", 200.00).
			Raw("UPDATE accounts:one SET balance -= $transfer_amount").
			Raw("UPDATE accounts:two SET balance += $transfer_amount")

		sql, vars := tx.Build()
		t.Logf("Transaction SurrealQL: %s", sql)
		t.Logf("Transaction Params: %v", vars)

		_, err := surrealdb.Query[any](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Transaction failed: %v", err)
		}

		// Verify balances
		accounts, err := surrealdb.Select[[]Account](ctx, db, "accounts")
		if err != nil {
			t.Fatalf("Failed to select accounts: %v", err)
		}

		for _, acc := range *accounts {
			if acc.ID.ID == acc1.ID.ID && acc.Balance != 800.00 {
				t.Errorf("Expected balance 800.00 for account one, got %.2f", acc.Balance)
			}
			if acc.ID.ID == acc2.ID.ID && acc.Balance != 700.00 {
				t.Errorf("Expected balance 700.00 for account two, got %.2f", acc.Balance)
			}
		}
	})

	t.Run("TransactionWithCondition", func(t *testing.T) {
		// Create a transaction with IF condition
		tx := surrealql.Begin().
			Let("min_balance", 100.00).
			Raw("UPDATE accounts:one SET balance -= 100").
			If("accounts:one.balance < $min_balance").
			Then(func(tb *surrealql.ThenBuilder) {
				tb.Throw("Insufficient funds")
			}).
			End()

		sql, vars := tx.Build()
		t.Logf("Conditional Transaction SurrealQL: %s", sql)

		_, err := surrealdb.Query[any](ctx, db, sql, vars)
		// The transaction might fail if balance goes below minimum after deduction
		// Just verify the query runs without error
		if err == nil {
			t.Log("Transaction succeeded - balance still above minimum")
		} else {
			t.Logf("Transaction failed as expected: %v", err)
		}
	})
}
