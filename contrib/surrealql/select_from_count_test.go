package surrealql

import "testing"

func TestSelect_counts(t *testing.T) {
	tests := []struct {
		name   string
		query  Query
		wantQL string
	}{
		{
			name:   "count all",
			query:  Select("users").Fields("count()").GroupAll(),
			wantQL: "SELECT count() FROM users GROUP ALL",
		},
		{
			name:   "count field",
			query:  Select("users").Fields("id, count(id) AS count_0").GroupAll(),
			wantQL: "SELECT id, count(id) AS count_0 FROM users GROUP ALL",
		},
		{
			name:   "count with where",
			query:  Select("users").Fields("count()").Where("active = ?", true).GroupAll(),
			wantQL: "SELECT count() FROM users WHERE active = $param_1 GROUP ALL",
		},
		{
			name:   "count group by",
			query:  Select("products").Fields("category, count() AS count").GroupBy("category"),
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
