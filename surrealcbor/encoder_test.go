package surrealcbor

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncoder tests Encoder functionality
func TestEncoder(t *testing.T) {
	t.Run("NewEncoder and Encode", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf)

		err := encoder.Encode([]int{1, 2, 3})
		require.NoError(t, err)

		// Verify the encoded data
		var v []int
		err = Unmarshal(buf.Bytes(), &v)
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, v)
	})

	t.Run("Encode multiple values", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf)

		err := encoder.Encode(1)
		require.NoError(t, err)

		err = encoder.Encode("hi")
		require.NoError(t, err)

		// Verify the encoded data
		decoder := NewDecoder(&buf)

		var i int
		err = decoder.Decode(&i)
		require.NoError(t, err)
		assert.Equal(t, 1, i)

		var s string
		err = decoder.Decode(&s)
		require.NoError(t, err)
		assert.Equal(t, "hi", s)
	})

	t.Run("Encode with write error", func(t *testing.T) {
		w := &errorWriter{err: errors.New("write error")}
		encoder := NewEncoder(w)

		err := encoder.Encode(123)
		assert.Error(t, err)
	})
}

// errorWriter is a writer that always returns an error
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}
