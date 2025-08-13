package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_array tests CBOR decoding of arrays
func TestDecode_array(t *testing.T) {
	t.Run("None in array", func(t *testing.T) {
		arr := []any{"first", models.None, "third", nil}
		data, err := Marshal(arr)
		require.NoError(t, err, "Marshal array failed")

		var decodedArr []any
		err = Unmarshal(data, &decodedArr)
		require.NoError(t, err, "Unmarshal array failed")

		require.Len(t, decodedArr, 4, "Array length mismatch")
		assert.Equal(t, "first", decodedArr[0], "Array[0] mismatch")
		assert.Nil(t, decodedArr[1], "Array[1] should be nil (from None)")
		assert.Equal(t, "third", decodedArr[2], "Array[2] mismatch")
		assert.Nil(t, decodedArr[3], "Array[3] should be nil")
	})
}
