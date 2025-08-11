package surrealql

import "testing"

func TestUpdate(t *testing.T) {
	tests := []struct {
		name      string
		query     Query
		wantSurQL string
	}{
		{
			name:      "update all with set",
			query:     Update("users").Set("active", false),
			wantSurQL: "UPDATE users SET active = $param_1",
		},
		{
			name:      "update specific record",
			query:     Update("users:123").Set("name", "Jane"),
			wantSurQL: "UPDATE users:123 SET name = $param_1",
		},
		{
			name:      "update with where",
			query:     Update("users").Set("active", false).Where("last_login < ?", "2024-01-01"),
			wantSurQL: "UPDATE users SET active = $param_1 WHERE last_login < $param_2",
		},
		{
			name:      "update with return diff",
			query:     Update("users").Set("name", "Jane").ReturnDiff(),
			wantSurQL: "UPDATE users SET name = $param_1 RETURN DIFF",
		},
		{
			name:      "update with compound operation",
			query:     Update("products").Set("stock -= ?", 5).Set("last_sold", "2024-01-01"),
			wantSurQL: "UPDATE products SET stock -= $param_1, last_sold = $param_2",
		},
		{
			name:      "update with multiple compound operations",
			query:     Update("stats").Set("views += ?", 1).Set("clicks += ?", 1),
			wantSurQL: "UPDATE stats SET views += $param_1, clicks += $param_2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSurQL, _ := tt.query.Build()

			if gotSurQL != tt.wantSurQL {
				t.Errorf("SurrealQL mismatch\ngot:  %q\nwant: %q", gotSurQL, tt.wantSurQL)
			}
		})
	}
}
