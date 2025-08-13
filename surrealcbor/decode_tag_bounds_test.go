package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecodeBoundIncludedTag(t *testing.T) {
	t.Run("decode BoundIncluded with int", func(t *testing.T) {
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagBoundIncluded,
			Content: 42,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		// For now, we just decode the content value
		assert.Equal(t, uint64(42), result)
	})

	t.Run("decode BoundIncluded with string", func(t *testing.T) {
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagBoundIncluded,
			Content: "test",
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result)
	})
}

func TestDecodeBoundExcludedTag(t *testing.T) {
	t.Run("decode BoundExcluded with int", func(t *testing.T) {
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagBoundExcluded,
			Content: 100,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		// For now, we just decode the content value
		assert.Equal(t, uint64(100), result)
	})

	t.Run("decode BoundExcluded with float", func(t *testing.T) {
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagBoundExcluded,
			Content: 3.14,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, 3.14, result)
	})
}

func TestDecodeRangeTag(t *testing.T) {
	t.Run("decode Range with integer bounds", func(t *testing.T) {
		// Range is encoded as [begin, end]
		rangeData := []any{
			cbor.Tag{Number: models.TagBoundIncluded, Content: 10},
			cbor.Tag{Number: models.TagBoundExcluded, Content: 20},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagRange,
			Content: rangeData,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		// For now, we decode as array
		arr, ok := result.([]any)
		assert.True(t, ok)
		assert.Len(t, arr, 2)
	})

	t.Run("decode Range with string bounds", func(t *testing.T) {
		rangeData := []any{
			cbor.Tag{Number: models.TagBoundIncluded, Content: "a"},
			cbor.Tag{Number: models.TagBoundIncluded, Content: "z"},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagRange,
			Content: rangeData,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		arr, ok := result.([]any)
		assert.True(t, ok)
		assert.Len(t, arr, 2)
	})
}
