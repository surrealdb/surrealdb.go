package surrealql

import "testing"

func TestCount(t *testing.T) {
	tests := []struct {
		name   string
		query  Query
		wantQL string
	}{
		{
			name:   "count all",
			query:  Count[string]().FromTable("users").GroupAll(),
			wantQL: "SELECT count() FROM users GROUP ALL",
		},
		{
			name:   "count field",
			query:  Count("id").FromTable("users").GroupAll(),
			wantQL: "SELECT id, count(id) AS count_0 FROM users GROUP ALL",
		},
		{
			name:   "count with alias",
			query:  Count[string]().As("total").FromTable("users").GroupAll(),
			wantQL: "SELECT count() AS total FROM users GROUP ALL",
		},
		{
			name:   "count with where",
			query:  Count[string]().FromTable("users").Where("active = ?", true).GroupAll(),
			wantQL: "SELECT count() FROM users WHERE active = $param_1 GROUP ALL",
		},
		{
			name:   "count group by",
			query:  CountGroupBy("category").FromTable("products"),
			wantQL: "SELECT category, count() AS count FROM products GROUP BY category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQL, _ := tt.query.Build()

			if gotQL != tt.wantQL {
				t.Errorf("SurrealQL mismatch\ngot:  %q\nwant: %q", gotQL, tt.wantQL)
			}
		})
	}
}
