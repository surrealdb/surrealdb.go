package surrealdb

import (
	"math"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// rawRow returns a result with the status and value given, encoded as CBOR.
func rawRow(t *testing.T, status string, v any) QueryResult[cbor.RawMessage] {
	t.Helper()
	b, err := cbor.Marshal(v)
	require.NoError(t, err)
	return QueryResult[cbor.RawMessage]{Status: status, Result: b}
}

// TestSplitRawResults feeds splitRawResults the results each version of SurrealDB
// returns for the statements given, each followed by the marker.
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
		aborted  = row{statusERR, errAbortedCommit}
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
			name:  "a transaction, 3.x",
			stmts: []string{"BEGIN", "CREATE person:1", "COMMIT"},
			rows:  []row{none, mark, created, mark, none, mark},
			want:  []row{none, created, none},
		},
		{
			name:  "a transaction, 2.x: BEGIN and COMMIT yield nothing",
			stmts: []string{"begin transaction;", "CREATE person:1", "commit transaction"},
			rows:  []row{mark, created, mark, mark},
			want:  []row{none, created, none},
		},
		{
			name:  "RETURN before COMMIT, 3.0 to 3.3: the marker after RETURN is skipped",
			stmts: []string{"BEGIN", "CREATE person:1", "RETURN true", "COMMIT"},
			rows:  []row{none, mark, created, mark, yes, none, mark},
			want:  []row{none, created, yes, none},
		},
		{
			name:  "RETURN before COMMIT, later versions",
			stmts: []string{"BEGIN", "CREATE person:1", "RETURN true", "COMMIT"},
			rows:  []row{none, mark, created, mark, yes, mark, none, mark},
			want:  []row{none, created, yes, none},
		},
		{
			name:  "RETURN before CANCEL, 3.0 to 3.3",
			stmts: []string{"BEGIN", "CREATE person:1", "RETURN true", "CANCEL"},
			rows:  []row{none, canceled, canceled, canceled, canceled, none, mark},
			want:  []row{none, canceled, canceled, none},
		},
		{
			name:  "a statement after RETURN in a transaction, 3.0 to 3.3",
			stmts: []string{"BEGIN", "RETURN true", "CREATE person:1", "COMMIT"},
			rows:  []row{none, mark, yes, none, mark},
			err:   "statement 1 did not yield exactly one result",
		},
		{
			name:  "RETURN in a transaction, 2.x: the transaction yields only the RETURN",
			stmts: []string{"BEGIN", "CREATE person:1", "RETURN true", "COMMIT"},
			rows:  []row{yes, mark},
			err:   "did not yield exactly one result",
		},
		{
			name:  "a failure before COMMIT, 3.0: COMMIT yields nothing",
			stmts: []string{"BEGIN", "CREATE person:1", "THROW 'x'", "COMMIT"},
			rows:  []row{none, failed, failed, failed, thrown, canceled, mark},
			want:  []row{none, failed, thrown, aborted},
		},
		{
			name:  "a failure before COMMIT, 3.3",
			stmts: []string{"BEGIN", "CREATE person:1", "THROW 'x'", "COMMIT"},
			rows:  []row{none, failed, failed, failed, thrown, canceled, aborted, mark},
			want:  []row{none, failed, thrown, aborted},
		},
		{
			name:  "a failure before COMMIT, 2.x",
			stmts: []string{"BEGIN", "CREATE person:1", "THROW 'x'", "COMMIT"},
			rows:  []row{failed, failed, failed, thrown, failed, mark},
			want:  []row{none, failed, thrown, aborted},
		},
		{
			name:  "a failure before CANCEL, 3.x: CANCEL yields nothing",
			stmts: []string{"BEGIN", "THROW 'x'", "CANCEL"},
			rows:  []row{none, failed, thrown, canceled, mark},
			want:  []row{none, thrown, none},
		},
		{
			name:  "a failure before CANCEL, later versions",
			stmts: []string{"BEGIN", "THROW 'x'", "CANCEL"},
			rows:  []row{none, failed, thrown, canceled, none, mark},
			want:  []row{none, thrown, none},
		},
		{
			name:  "a failure before CANCEL, 2.x",
			stmts: []string{"BEGIN", "THROW 'x'", "CANCEL"},
			rows:  []row{failed, thrown, failed, mark},
			want:  []row{none, thrown, none},
		},
		{
			name:  "CANCEL, 3.x",
			stmts: []string{"BEGIN", "CREATE person:1", "CANCEL"},
			rows:  []row{none, canceled, canceled, canceled, none, mark},
			want:  []row{none, canceled, none},
		},
		{
			name:  "CANCEL, 2.x",
			stmts: []string{"BEGIN", "CREATE person:1", "CANCEL"},
			rows:  []row{canceled, canceled, canceled, mark},
			want:  []row{none, canceled, none},
		},
		{
			name:  "a transaction that times out, 3.3: the results stop",
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
			err:   "2 results more than expected",
		},
		{
			name:  "a statement that yields nothing",
			stmts: []string{"RETURN true", "-- nothing"},
			rows:  []row{yes, mark, mark},
			err:   "statement 1 did not yield exactly one result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := make([]QueryResult[cbor.RawMessage], len(tt.rows))
			for i, r := range tt.rows {
				res[i] = rawRow(t, r.status, r.value)
			}
			kinds := make([]rawStmtKind, len(tt.stmts))
			for i, sql := range tt.stmts {
				kinds[i] = rawStmtKindOf(sql)
			}

			got, err := splitRawResults(&res, kinds, marker, surrealcbor.New())
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
