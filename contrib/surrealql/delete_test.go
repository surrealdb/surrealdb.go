package surrealql

import "testing"

func TestDelete(t *testing.T) {
	tests := []struct {
		name   string
		query  Query
		wantQL string
	}{
		{
			name:   "delete all",
			query:  Delete("users"),
			wantQL: "DELETE users",
		},
		{
			name:   "delete specific record",
			query:  Delete("users:123"),
			wantQL: "DELETE users:123",
		},
		{
			name:   "delete with where",
			query:  Delete("users").Where("active = ?", false),
			wantQL: "DELETE users WHERE active = $param_1",
		},
		{
			name:   "delete with return none",
			query:  Delete("users").ReturnNone(),
			wantQL: "DELETE users RETURN NONE",
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
