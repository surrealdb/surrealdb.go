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
		// SurrealQL reads these as values, not as the fields they name.
		{"none", "`none`"},
		{"NULL", "`NULL`"},
		{"true", "`true`"},
		{"false", "`false`"},
		// Anything but a plain identifier is quoted, so that it cannot add to
		// the statement.
		{"n;CREATE/**/pwned", "`n;CREATE/**/pwned`"},
		{"n;CREATE\tpwned", "`n;CREATE\tpwned`"},
		{"a -- b", "`a -- b`"},
		{"rand()", "`rand()`"},
		{"1st", "`1st`"},
		{"", "``"},
		// A backslash would otherwise escape the closing backtick.
		{"x\\`;DELETE t;--", "`x\\\\\\`;DELETE t;--`"},
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

func TestEscapeFieldPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"name", "name"},
		{"address.city", "address.city"},
		{"tags.*", "tags.*"},
		{"meta.none", "meta.`none`"},
		{"a.b;DELETE t", "a.`b;DELETE t`"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := escapeFieldPath(tt.input); got != tt.want {
				t.Errorf("escapeFieldPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
