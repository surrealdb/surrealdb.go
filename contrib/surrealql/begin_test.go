package surrealql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
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

// Values that write themselves as SurrealQL literals are written as those
// literals, not as their display text: LET $d = 2024-02-29 would be integer
// arithmetic.
func TestBegin_letWritesSurrealQLLiterals(t *testing.T) {
	sql, _ := Begin().
		Let("due", models.MustParsePlainDate("2024-02-29")).
		Let("opens", models.MustParsePlainTime("09:30")).
		Let("at", models.MustParseTimestamp("2024-02-29T09:30:00")).
		Let("every", models.Interval{Days: 1}).
		LetTyped("price", "bigdecimal", models.MustParseBigDecimal("1.50")).
		Let("pattern", models.Regex{Pattern: "a/b"}).
		Let("owner", &models.RecordID{Table: "person", ID: "tobie"}).
		Throw(models.Interval{Months: 1}).
		Build()
	assert.Equal(t, `BEGIN TRANSACTION;
LET $due = pd'2024-02-29';
LET $opens = pt'09:30:00';
LET $at = ts'2024-02-29T09:30:00';
LET $every = ivl'P1D';
LET $price: bigdecimal = 1.50bigdec;
LET $pattern = /a\/b/;
LET $owner = r'person:tobie';
THROW ivl'P1M';
COMMIT TRANSACTION;`, sql)
}

// A nil pointer whose SurrealString method cannot take a nil receiver is
// written with %v, as before, rather than panicking.
func TestBegin_letNilSurrealStringer(t *testing.T) {
	sql, _ := Begin().Let("owner", (*models.RecordID)(nil)).Throw((*models.CustomDateTime)(nil)).Build()
	assert.Equal(t, "BEGIN TRANSACTION;\nLET $owner = <nil>;\nTHROW <nil>;\nCOMMIT TRANSACTION;", sql)
}
