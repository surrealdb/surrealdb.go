package surrealql

import (
	"testing"
)

func TestEscapeIdent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", "users"},
		{"user_name", "user_name"},
		{"user-name", "`user-name`"},
		{"user name", "`user name`"},
		{"user:id", "`user:id`"},
		{"SELECT", "`SELECT`"},
		{"select", "`select`"},
		{"my`table", "`my\\`table`"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeIdent(tt.input)
			if got != tt.want {
				t.Errorf("escapeIdent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
