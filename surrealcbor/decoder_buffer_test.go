package surrealcbor

import (
	"bytes"
	"io"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecoderWithCustomBufferSize(t *testing.T) {
	t.Run("small buffer size works correctly", func(t *testing.T) {
		// Create test data larger than small buffer
		testData := []any{
			"hello",
			12345,
			true,
			map[string]any{"key": "value", "number": 42},
			[]int{1, 2, 3, 4, 5},
		}

		var buf bytes.Buffer
		enc := cbor.NewEncoder(&buf)
		for _, v := range testData {
			err := enc.Encode(v)
			require.NoError(t, err)
		}

		// Decode with small buffer size (10 bytes)
		dec := NewDecoderWithBufferSize(&buf, 10)

		var decoded []any
		for i := 0; i < len(testData); i++ {
			var v any
			err := dec.Decode(&v)
			require.NoError(t, err)
			decoded = append(decoded, v)
		}

		// Verify all values decoded correctly
		assert.Equal(t, len(testData), len(decoded))
		assert.Equal(t, "hello", decoded[0])
		assert.Equal(t, uint64(12345), decoded[1]) // CBOR decodes positive ints as uint64
		assert.Equal(t, true, decoded[2])
	})

	t.Run("default buffer size when zero", func(t *testing.T) {
		testData := map[string]string{"test": "value"}

		var buf bytes.Buffer
		enc := cbor.NewEncoder(&buf)
		err := enc.Encode(testData)
		require.NoError(t, err)

		// Create decoder with buffer size 0 (should use default)
		dec := NewDecoderWithBufferSize(&buf, 0)

		var decoded map[string]any
		err = dec.Decode(&decoded)
		require.NoError(t, err)

		assert.Equal(t, "value", decoded["test"])
	})

	t.Run("handles partial reads correctly", func(t *testing.T) {
		// Create a reader that returns data in small chunks
		type slowReader struct {
			data []byte
			pos  int
		}

		sr := &slowReader{}

		// Encode test data
		testData := map[string]any{
			"field1": "a long string value that spans multiple reads",
			"field2": 123456789,
			"field3": []string{"item1", "item2", "item3"},
		}

		encoded, err := cbor.Marshal(testData)
		require.NoError(t, err)
		sr.data = encoded

		// Reader that returns max 5 bytes at a time
		reader := funcReader(func(p []byte) (int, error) {
			if sr.pos >= len(sr.data) {
				return 0, io.EOF
			}
			n := 5
			if n > len(p) {
				n = len(p)
			}
			if sr.pos+n > len(sr.data) {
				n = len(sr.data) - sr.pos
			}
			copy(p, sr.data[sr.pos:sr.pos+n])
			sr.pos += n
			return n, nil
		})

		dec := NewDecoderWithBufferSize(reader, 8)

		var decoded map[string]any
		err = dec.Decode(&decoded)
		require.NoError(t, err)

		assert.Equal(t, testData["field1"], decoded["field1"])
		assert.Equal(t, uint64(123456789), decoded["field2"]) // CBOR decodes positive ints as uint64
	})
}

// funcReader wraps a function as an io.Reader
type funcReader func([]byte) (int, error)

func (f funcReader) Read(p []byte) (int, error) {
	return f(p)
}
