package models

import (
	"testing"
)

func TestRecordID_String(t *testing.T) {
	tests := []struct {
		name     string
		recordID RecordID
		expected string
	}{
		{
			name:     "simple alphanumeric table and ID",
			recordID: RecordID{Table: "users", ID: "123"},
			expected: "users:⟨123⟩",
		},
		{
			name:     "ID with special characters needs escaping",
			recordID: RecordID{Table: "users", ID: "user-123"},
			expected: "users:⟨user-123⟩",
		},
		{
			name:     "ID with special characters needs escaping",
			recordID: RecordID{Table: "user-profiles", ID: "id-123"},
			expected: "user-profiles:⟨id-123⟩",
		},
		{
			name:     "numeric ID",
			recordID: RecordID{Table: "users", ID: 123},
			expected: "users:123",
		},
		{
			name:     "ID with full width digits",
			recordID: RecordID{Table: "users", ID: "０１２３"},
			expected: "users:⟨０１２３⟩",
		},
		{
			name:     "ID with emoji",
			recordID: RecordID{Table: "users", ID: "user😀"},
			expected: "users:⟨user😀⟩",
		},
		// In the following cases, we demonstrate that complex ID types are
		// formatted differently in this SDK and in Rust.
		{
			name:     "complex ID with array",
			recordID: RecordID{Table: "users", ID: []any{"a", "b", "c"}},
			// This should be formatted as `users:['a','b','c']` in Rust.
			expected: "users:[a b c]",
		},
		{
			name:     "complex ID with map",
			recordID: RecordID{Table: "users", ID: map[string]any{"key": "value"}},
			// This should be formatted as `users:{key:'value'}` in Rust.
			expected: "users:map[key:value]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.recordID.String()
			if result != tt.expected {
				t.Errorf("RecordID.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}
