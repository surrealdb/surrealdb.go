package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_future tests custom future decoding
func TestDecode_future(t *testing.T) {
	t.Run("decode future tag", func(t *testing.T) {
		// Tag 15 (future) - expects an array [block_id, ...]
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  15,
			Content: []any{int64(123), int64(456)},
		})

		var future models.Future
		err := Unmarshal(enc, &future)
		require.NoError(t, err)
	})

	t.Run("decode future tag with non-array", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  15,
			Content: 123,
		})

		var future models.Future
		err := Unmarshal(enc, &future)
		// Current implementation doesn't validate content, just creates empty Future
		assert.NoError(t, err)
	})
}
