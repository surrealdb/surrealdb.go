package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRange_String covers Range[T, TBeg, TEnd].String() for the four
// inclusive/exclusive bound combinations.
//
// Prior to the fix, convertToString was a stub that always returned "",
// so every case below would have produced just the join string (e.g. "..",
// "..=", ">..", ">..="). In addition, the end bound was rendered by calling
// convertToString(r.Begin) instead of convertToString(r.End), so even with
// convertToString implemented, the end value would have echoed the begin
// value instead of the real end value whenever begin != end.
func TestRange_String(t *testing.T) {
	t.Run("begin included, end included", func(t *testing.T) {
		r := Range[int, BoundIncluded[int], BoundIncluded[int]]{
			Begin: &BoundIncluded[int]{1},
			End:   &BoundIncluded[int]{10},
		}
		assert.Equal(t, "1..=10", r.String())
	})

	t.Run("begin included, end excluded", func(t *testing.T) {
		r := Range[int, BoundIncluded[int], BoundExcluded[int]]{
			Begin: &BoundIncluded[int]{1},
			End:   &BoundExcluded[int]{10},
		}
		assert.Equal(t, "1..10", r.String())
	})

	t.Run("begin excluded, end included", func(t *testing.T) {
		r := Range[int, BoundExcluded[int], BoundIncluded[int]]{
			Begin: &BoundExcluded[int]{1},
			End:   &BoundIncluded[int]{10},
		}
		assert.Equal(t, "1>..=10", r.String())
	})

	t.Run("begin excluded, end excluded", func(t *testing.T) {
		r := Range[int, BoundExcluded[int], BoundExcluded[int]]{
			Begin: &BoundExcluded[int]{1},
			End:   &BoundExcluded[int]{10},
		}
		assert.Equal(t, "1>..10", r.String())
	})

	t.Run("begin and end differ, end must not echo begin", func(t *testing.T) {
		// Regression test for the Begin/End field-name swap bug: the old
		// code called convertToString(r.Begin) for both bounds, so the end
		// bound rendered as "5" (the begin value) instead of "99".
		r := Range[int, BoundIncluded[int], BoundIncluded[int]]{
			Begin: &BoundIncluded[int]{5},
			End:   &BoundIncluded[int]{99},
		}
		result := r.String()
		assert.Equal(t, "5..=99", result)
		assert.NotEqual(t, "5..=5", result)
	})

	t.Run("string bounds are escaped like RecordID IDs", func(t *testing.T) {
		r := Range[string, BoundIncluded[string], BoundExcluded[string]]{
			Begin: &BoundIncluded[string]{"a-b"},
			End:   &BoundExcluded[string]{"c-d"},
		}
		assert.Equal(t, "⟨a-b⟩..⟨c-d⟩", r.String())
	})
}

// TestRecordRangeID_String covers RecordRangeID[T, TBeg, TEnd].String(),
// which prefixes the range with "<table>:".
//
// Same rationale as TestRange_String: pre-fix, the stubbed convertToString
// meant the bounds rendered as empty strings, and the Begin/End swap meant
// the end bound (when non-empty) would have echoed the begin value.
func TestRecordRangeID_String(t *testing.T) {
	t.Run("begin included, end included", func(t *testing.T) {
		rr := RecordRangeID[int, BoundIncluded[int], BoundIncluded[int]]{
			Range: Range[int, BoundIncluded[int], BoundIncluded[int]]{
				Begin: &BoundIncluded[int]{1},
				End:   &BoundIncluded[int]{10},
			},
			Table: Table("foo"),
		}
		assert.Equal(t, "foo:1..=10", rr.String())
	})

	t.Run("begin excluded, end excluded", func(t *testing.T) {
		rr := RecordRangeID[int, BoundExcluded[int], BoundExcluded[int]]{
			Range: Range[int, BoundExcluded[int], BoundExcluded[int]]{
				Begin: &BoundExcluded[int]{1},
				End:   &BoundExcluded[int]{10},
			},
			Table: Table("foo"),
		}
		assert.Equal(t, "foo:1>..10", rr.String())
	})

	t.Run("begin excluded, end included", func(t *testing.T) {
		rr := RecordRangeID[int, BoundExcluded[int], BoundIncluded[int]]{
			Range: Range[int, BoundExcluded[int], BoundIncluded[int]]{
				Begin: &BoundExcluded[int]{1},
				End:   &BoundIncluded[int]{10},
			},
			Table: Table("foo"),
		}
		assert.Equal(t, "foo:1>..=10", rr.String())
	})

	t.Run("end bound must reflect End, not Begin", func(t *testing.T) {
		rr := RecordRangeID[int, BoundIncluded[int], BoundIncluded[int]]{
			Range: Range[int, BoundIncluded[int], BoundIncluded[int]]{
				Begin: &BoundIncluded[int]{5},
				End:   &BoundIncluded[int]{99},
			},
			Table: Table("foo"),
		}
		result := rr.String()
		assert.Equal(t, "foo:5..=99", result)
		assert.NotEqual(t, "foo:5..=5", result)
	})
}

// TestConvertToString exercises the convertToString helper directly to make
// sure it no longer returns the empty-string stub, and that it applies the
// same escaping rules RecordID.String uses for string IDs.
func TestConvertToString(t *testing.T) {
	tests := []struct {
		name     string
		bound    any
		expected string
	}{
		{
			name:     "included int",
			bound:    &BoundIncluded[int]{42},
			expected: "42",
		},
		{
			name:     "excluded int",
			bound:    &BoundExcluded[int]{42},
			expected: "42",
		},
		{
			name:     "included plain string",
			bound:    &BoundIncluded[string]{"abc"},
			expected: "abc",
		},
		{
			name:     "included string needing escaping",
			bound:    &BoundIncluded[string]{"a-b"},
			expected: "⟨a-b⟩",
		},
		{
			name:     "excluded numeric-looking string needs escaping",
			bound:    &BoundExcluded[string]{"123"},
			expected: "⟨123⟩",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToString(tt.bound)
			assert.NotEmpty(t, result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
