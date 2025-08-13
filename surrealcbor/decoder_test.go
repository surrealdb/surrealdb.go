package surrealcbor

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecoder tests Decoder functionality
func TestDecoder(t *testing.T) {
	t.Run("NewDecoder and Decode", func(t *testing.T) {
		data := []byte{0x83, 0x01, 0x02, 0x03} // [1, 2, 3]
		buf := bytes.NewBuffer(data)

		decoder := NewDecoder(buf)

		var v []int
		err := decoder.Decode(&v)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, v)
	})

	t.Run("Decode multiple values", func(t *testing.T) {
		data := []byte{}
		data = append(data, 0x01, 0x62, 0x68, 0x69) // 1, "hi"
		buf := bytes.NewBuffer(data)

		decoder := NewDecoder(buf)

		var i int
		err := decoder.Decode(&i)
		require.NoError(t, err)
		assert.Equal(t, 1, i)

		var s string
		err = decoder.Decode(&s)
		require.NoError(t, err)
		assert.Equal(t, "hi", s)
	})

	t.Run("Decode with EOF", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{})
		decoder := NewDecoder(buf)

		var v int
		err := decoder.Decode(&v)
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("Decode with read error", func(t *testing.T) {
		// Create a reader that returns an error
		r := &errorReader{err: errors.New("read error")}
		decoder := NewDecoder(r)

		var v int
		err := decoder.Decode(&v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read error")
	})

	t.Run("Decode with partial read", func(t *testing.T) {
		// Array with 3 elements but incomplete data
		data := []byte{0x83, 0x01} // [1, ?, ?] - missing 2 elements
		buf := bytes.NewBuffer(data)
		decoder := NewDecoder(buf)

		var v []int
		err := decoder.Decode(&v)
		assert.Error(t, err)
	})
}

// errorReader is a reader that always returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}
