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
			wantSurQL: "UPDATE users SET active = $active_1",
		},
		{
			name:      "update specific record",
			query:     Update("users:123").Set("name", "Jane"),
			wantSurQL: "UPDATE users:123 SET name = $name_1",
		},
		{
			name:      "update with where",
			query:     Update("users").Set("active", false).Where("last_login < ?", "2024-01-01"),
			wantSurQL: "UPDATE users SET active = $active_1 WHERE last_login < $param_1",
		},
		{
			name:      "update with return diff",
			query:     Update("users").Set("name", "Jane").ReturnDiff(),
			wantSurQL: "UPDATE users SET name = $name_1 RETURN DIFF",
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
