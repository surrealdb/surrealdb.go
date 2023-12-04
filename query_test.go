package surrealdb

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

// TestSurrealDBLocal runs a local SurrealDB instance by calling
// `NewTestSurrealDB(t)`, which is a dedicated SurrealDB instance with no data.
func TestSurrealDBLocal(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		schema              []string
		dbInteraction       func(*testing.T, *DB)
		keepServerUponError bool
	}{
		"first case": {
			schema: []string{
				"CREATE user:x SET name = 'xxxx'",
				"CREATE user:y SET name = 'yyyy'",
			},
			dbInteraction: func(t *testing.T, db *DB) {
				data, err := db.Select("user")
				if err != nil {
					t.Fatalf("failed to select, %v", err)
				}
				type dataholder struct {
					ID   string `json:"id,omitempty"`
					Name string `json:"name,omitempty"`
				}
				d, err := marshal.SmartUnmarshal[dataholder](data, nil)
				if err != nil {
					t.Fatal(err)
				}
				want := []dataholder{
					{ID: "user:x", Name: "xxxx"},
					{ID: "user:y", Name: "yyyy"},
				}
				if diff := cmp.Diff(want, d); diff != "" {
					t.Errorf("Result mismatch (-want +got):\n%s", diff)
				}
			},
		},
		"second case": {
			schema: []string{
				"CREATE user:x SET name = 'xxxx'",
				"CREATE user:y SET name = 'yyyy'",
			},
			dbInteraction: func(t *testing.T, db *DB) {
			},
		},
		"third case": {
			schema: []string{
				"CREATE user:x SET name = 'xxxx'",
				"CREATE user:y SET name = 'yyyy'",
				"CREATE user:y SET name = 'yyyy'", // Should fail
			},
			dbInteraction: func(t *testing.T, db *DB) {
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			endpoint, db, cancel := NewTestSurrealDB(t)
			defer func() {
				// If test case failed, keep the server running for further
				// investigation. This could be something we can turn on with
				// some special flag.
				if tc.keepServerUponError && t.Failed() {
					return
				}
				// Otherwise cleanup the server.
				cancel()
			}()
			_ = endpoint // Not used for this test.

			for _, s := range tc.schema {
				db.Prepare(t, s)
			}

			tc.dbInteraction(t, db.DB)
		})
	}
}
