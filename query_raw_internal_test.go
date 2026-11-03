package surrealdb

import (
	"math"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/surrealcbor"
)

// rawRow returns a result with the status and value given, encoded as CBOR.
func rawRow(t *testing.T, status string, v any) QueryResult[cbor.RawMessage] {
	t.Helper()
	b, err := cbor.Marshal(v)
	require.NoError(t, err)
	return QueryResult[cbor.RawMessage]{Status: status, Result: b}
}

// TestSplitRawResults feeds splitRawResults the results SurrealDB returns for the
// statements given, each followed by the marker.
func TestSplitRawResults(t *testing.T) {
	const marker = "QueryRaw-test"

	type row struct {
		status string
		value  any
	}
	var (
		none     = row{statusOK, nil}
		mark     = row{statusOK, marker}
		failed   = row{statusERR, errFailedTransaction}
		canceled = row{statusERR, errCancelledTransaction}
		aborted  = row{statusERR, "Cannot COMMIT: the transaction was aborted due to a prior error"}
		thrown   = row{statusERR, "An error occurred: x"}
		created  = row{statusOK, []any{map[string]any{"id": "person:1"}}}
		yes      = row{statusOK, true}
	)

	tests := []struct {
		name  string
		stmts []string
		rows  []row
		want  []row
		err   string
	}{
		{
			name:  "statements",
			stmts: []string{"RETURN true", "CREATE person:1"},
			rows:  []row{yes, mark, created, mark},
			want:  []row{yes, created},
		},
		{
			name:  "a transaction",
			stmts: []string{"BEGIN", "CREATE person:1", "COMMIT"},
			rows:  []row{none, mark, created, mark, none, mark},
			want:  []row{none, created, none},
		},
		{
			name:  "RETURN before COMMIT",
			stmts: []string{"BEGIN", "CREATE person:1", "RETURN true", "COMMIT"},
			rows:  []row{none, mark, created, mark, yes, mark, none, mark},
			want:  []row{none, created, yes, none},
		},
		{
			name:  "a statement after RETURN in a transaction",
			stmts: []string{"BEGIN", "RETURN true", "CREATE person:1", "COMMIT"},
			rows:  []row{none, mark, yes, mark, created, mark, none, mark},
			want:  []row{none, yes, created, none},
		},
		{
			name:  "RETURN before CANCEL",
			stmts: []string{"BEGIN", "CREATE person:1", "RETURN true", "CANCEL"},
			rows:  []row{none, canceled, canceled, canceled, canceled, canceled, none, mark},
			want:  []row{none, canceled, canceled, none},
		},
		{
			name:  "a failure before COMMIT",
			stmts: []string{"BEGIN", "CREATE person:1", "THROW 'x'", "COMMIT"},
			rows:  []row{none, failed, failed, failed, thrown, canceled, aborted, mark},
			want:  []row{none, failed, thrown, aborted},
		},
		{
			name:  "a failure before CANCEL",
			stmts: []string{"BEGIN", "THROW 'x'", "CANCEL"},
			rows:  []row{none, failed, thrown, canceled, none, mark},
			want:  []row{none, thrown, none},
		},
		{
			name:  "CANCEL",
			stmts: []string{"BEGIN", "CREATE person:1", "CANCEL"},
			rows:  []row{none, canceled, canceled, canceled, none, mark},
			want:  []row{none, canceled, none},
		},
		{
			name:  "results that stop after an error",
			stmts: []string{"BEGIN", "CREATE person:1", "SLEEP 2s", "COMMIT", "RETURN 5"},
			rows: []row{
				none,
				{statusERR, "The transaction timed out: 1s"},
				{statusERR, "The transaction timed out: 1s"},
				{statusERR, "The transaction was not completed because it exceeded the timeout: 1s"},
			},
			err: "after the server returned an error: The transaction timed out: 1s",
		},
		{
			name:  "two statements in one before COMMIT",
			stmts: []string{"BEGIN", "CREATE person:1; RETURN true", "COMMIT"},
			rows:  []row{none, mark, created, yes, mark, none, mark},
			err:   "statement 1 did not yield exactly one result",
		},
		{
			name:  "a statement that yields nothing",
			stmts: []string{"RETURN true", "-- nothing"},
			rows:  []row{yes, mark, mark},
			err:   "statement 1 did not yield exactly one result",
		},
		{
			name:  "more results than statements",
			stmts: []string{"RETURN true"},
			rows:  []row{yes, mark, yes, mark},
			err:   "2 results more than expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := make([]QueryResult[cbor.RawMessage], len(tt.rows))
			for i, r := range tt.rows {
				res[i] = rawRow(t, r.status, r.value)
			}

			got, err := splitRawResults(&res, len(tt.stmts), marker, surrealcbor.New())
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			for i, w := range tt.want {
				assert.Equal(t, rawRow(t, w.status, w.value), got[i], "statement %d: %s", i, tt.stmts[i])
			}
		})
	}
}

func TestComposeRawQuery_variables(t *testing.T) {
	compose := func(a, b any) error {
		_, _, err := composeRawQuery([]QueryStmt{
			{SQL: "RETURN $x", Vars: map[string]any{"x": a}},
			{SQL: "RETURN $x", Vars: map[string]any{"x": b}},
		}, "m")
		return err
	}

	// Values the server sees as the same are accepted.
	require.NoError(t, compose(1, int64(1)))
	require.NoError(t, compose(math.NaN(), math.NaN()))
	require.NoError(t, compose(map[string]any{"a": 1}, map[string]any{"a": 1}))

	// Different values are refused, naming their types but not the values, which
	// may be secrets.
	err := compose("secret", 2)
	require.ErrorContains(t, err, "$x")
	require.ErrorContains(t, err, "(string and int)")
	assert.NotContains(t, err.Error(), "secret")
}
