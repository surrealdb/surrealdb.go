package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_recordID tests decoding of RecordID
func TestDecode_recordID(t *testing.T) {
	t.Run("decode recordid tag", func(t *testing.T) {
		// Tag 8 (recordid) as array
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  8,
			Content: []any{"table", "id123"},
		})

		var rid models.RecordID
		err := Unmarshal(enc, &rid)
		require.NoError(t, err)
		assert.Equal(t, "table", rid.Table)
		assert.Equal(t, "id123", rid.ID)
	})

	t.Run("decode recordid tag with invalid array length", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  8,
			Content: []any{"table"}, // Missing ID
		})

		var rid models.RecordID
		err := Unmarshal(enc, &rid)
		// Should return error for invalid array length
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid RecordID format")
	})

	t.Run("decode recordid tag with non-array", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  8,
			Content: "not-an-array",
		})

		var rid models.RecordID
		err := Unmarshal(enc, &rid)
		// Expects an array
		assert.Error(t, err)
	})
}
