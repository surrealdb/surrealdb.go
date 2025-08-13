package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode_null_none(t *testing.T) {
	t.Run("decode null to non-nilable type", func(t *testing.T) {
		data := []byte{0xF6} // null

		var i int
		err := Unmarshal(data, &i)
		require.NoError(t, err)
		assert.Equal(t, 0, i) // Should get zero value
	})

	t.Run("decode None to various types", func(t *testing.T) {
		// Encode None
		data := []byte{0xC6, 0xF6} // Tag 6 (None) with null

		// Test with pointer
		var p *int
		err := Unmarshal(data, &p)
		require.NoError(t, err)
		assert.Nil(t, p)

		// Test with interface
		var i any
		err = Unmarshal(data, &i)
		require.NoError(t, err)
		assert.Nil(t, i)

		// Test with slice
		var s []int
		err = Unmarshal(data, &s)
		require.NoError(t, err)
		assert.Nil(t, s)

		// Test with map
		var m map[string]int
		err = Unmarshal(data, &m)
		require.NoError(t, err)
		assert.Nil(t, m)
	})
}
