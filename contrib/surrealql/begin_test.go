package surrealql

import (
	"testing"
)

func TestBegin(t *testing.T) {
	tests := []struct {
		name       string
		query      *TransactionQuery
		wantQL     string
		wantParams map[string]any
	}{
		{
			name: "simple transaction",
			query: Begin().
				Raw("CREATE person:test SET name = 'John'").
				Raw("CREATE person:test2 SET name = 'Jane'"),
			wantQL: `BEGIN TRANSACTION;
CREATE person:test SET name = 'John';
CREATE person:test2 SET name = 'Jane';
COMMIT TRANSACTION;`,
			wantParams: map[string]any{},
		},
		{
			name: "transaction with let and throw",
			query: Begin().
				Let("amount", 100).
				Raw("UPDATE account SET balance -= $amount").
				If("balance < 0").
				Then(func(tb *ThenBuilder) {
					tb.Throw("Insufficient funds")
				}).
				End(),
			wantQL: `BEGIN TRANSACTION;
LET $amount = 100;
UPDATE account SET balance -= $amount;
IF balance < 0 {
    THROW "Insufficient funds";
};
COMMIT TRANSACTION;`,
			wantParams: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, vars := tt.query.Build()

			if sql != tt.wantQL {
				t.Errorf("Build() sql = %v, want %v", sql, tt.wantQL)
			}

			if len(vars) != len(tt.wantParams) {
				t.Errorf("Build() params = %v, want %v", vars, tt.wantParams)
			}
		})
	}
}
