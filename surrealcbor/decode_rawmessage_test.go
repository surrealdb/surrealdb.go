package surrealcbor_test

import (
	"math"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

func TestDecodeRawMessage(t *testing.T) {
	t.Run("decode simple integer into RawMessage", func(t *testing.T) {
		// Encode an integer
		data, err := cbor.Marshal(42)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		// RawMessage should contain the exact CBOR bytes
		assert.Equal(t, data, []byte(rawMsg))

		// We should be able to unmarshal it again
		var value int
		err = cbor.Unmarshal(rawMsg, &value)
		require.NoError(t, err)
		assert.Equal(t, 42, value)
	})

	t.Run("decode string into RawMessage", func(t *testing.T) {
		data, err := cbor.Marshal("hello world")
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))

		var value string
		err = cbor.Unmarshal(rawMsg, &value)
		require.NoError(t, err)
		assert.Equal(t, "hello world", value)
	})

	t.Run("decode map into RawMessage", func(t *testing.T) {
		testMap := map[string]any{
			"name":   "John",
			"age":    30,
			"active": true,
		}

		data, err := cbor.Marshal(testMap)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))

		var decoded map[string]any
		err = cbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "John", decoded["name"])
		assert.Equal(t, uint64(30), decoded["age"])
		assert.Equal(t, true, decoded["active"])
	})

	t.Run("decode array into RawMessage", func(t *testing.T) {
		testArray := []any{1, 2, 3, "four", true}

		data, err := cbor.Marshal(testArray)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))

		var decoded []any
		err = cbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)
		assert.Len(t, decoded, 5)
		assert.Equal(t, uint64(1), decoded[0])
		assert.Equal(t, "four", decoded[3])
	})

	t.Run("decode null into RawMessage", func(t *testing.T) {
		data, err := cbor.Marshal(nil)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))
		assert.Equal(t, []byte{0xf6}, []byte(rawMsg)) // CBOR null

		var decoded any
		err = cbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)
		assert.Nil(t, decoded)
	})

	t.Run("decode Tag 6 (None) into RawMessage", func(t *testing.T) {
		// Create CBOR Tag 6 with null content
		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagNone,
			Content: nil,
		})
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		// RawMessage should contain the tag bytes
		assert.Equal(t, data, []byte(rawMsg))
		assert.Equal(t, []byte{0xc6, 0xf6}, []byte(rawMsg)) // Tag 6 + null
	})

	t.Run("decode nested structure with RawMessage", func(t *testing.T) {
		type TestStruct struct {
			ID     string            `cbor:"id"`
			Result cbor.RawMessage   `cbor:"result"`
			Meta   map[string]string `cbor:"meta"`
		}

		// Create nested data
		innerData := map[string]any{
			"value": 123,
			"text":  "test",
		}
		innerBytes, err := cbor.Marshal(innerData)
		require.NoError(t, err)

		testData := map[string]any{
			"id":     "test-id",
			"result": cbor.RawMessage(innerBytes),
			"meta": map[string]string{
				"version": "1.0",
			},
		}

		data, err := cbor.Marshal(testData)
		require.NoError(t, err)

		var decoded TestStruct
		err = surrealcbor.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "test-id", decoded.ID)
		assert.Equal(t, "1.0", decoded.Meta["version"])
		assert.Equal(t, innerBytes, []byte(decoded.Result))

		// Verify we can unmarshal the RawMessage
		var innerDecoded map[string]any
		err = cbor.Unmarshal(decoded.Result, &innerDecoded)
		require.NoError(t, err)
		assert.Equal(t, uint64(123), innerDecoded["value"])
		assert.Equal(t, "test", innerDecoded["text"])
	})

	t.Run("decode RPCResponse with RawMessage", func(t *testing.T) {
		// Create a response similar to what SurrealDB sends
		resultData := map[string]any{
			"name": "Test",
			"age":  25,
		}
		resultBytes, err := cbor.Marshal(resultData)
		require.NoError(t, err)

		response := connection.RPCResponse[cbor.RawMessage]{
			ID:     "123",
			Result: (*cbor.RawMessage)(&resultBytes),
		}

		data, err := cbor.Marshal(response)
		require.NoError(t, err)

		var decoded connection.RPCResponse[cbor.RawMessage]
		err = surrealcbor.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, "123", decoded.ID)
		assert.NotNil(t, decoded.Result)
		assert.Equal(t, resultBytes, []byte(*decoded.Result))

		// Verify we can unmarshal the result
		var result map[string]any
		err = cbor.Unmarshal(*decoded.Result, &result)
		require.NoError(t, err)
		assert.Equal(t, "Test", result["name"])
		assert.Equal(t, uint64(25), result["age"])
	})

	t.Run("decode complex CBOR with tags into RawMessage", func(t *testing.T) {
		// Create complex data with various CBOR tags
		complexData := map[string]any{
			"table": cbor.Tag{
				Number:  models.TagTable,
				Content: "users",
			},
			"recordID": cbor.Tag{
				Number:  models.TagRecordID,
				Content: []any{"users", "123"},
			},
			"datetime": cbor.Tag{
				Number:  models.TagCustomDatetime,
				Content: []int64{1234567890, 123456789},
			},
		}

		data, err := cbor.Marshal(complexData)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))

		// Verify the raw message contains valid CBOR
		var decoded map[string]any
		err = cbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)
		assert.Contains(t, decoded, "table")
		assert.Contains(t, decoded, "recordID")
		assert.Contains(t, decoded, "datetime")
	})

	t.Run("decode indefinite length array into RawMessage", func(t *testing.T) {
		// Create indefinite length array
		// CBOR: 0x9f (array, indefinite), items..., 0xff (break)
		cborData := []byte{
			0x9f,          // Indefinite array start
			0x01,          // 1
			0x02,          // 2
			0x03,          // 3
			0x63,          // Text string of length 3
			'a', 'b', 'c', // "abc"
			0xff, // Break marker
		}

		var rawMsg cbor.RawMessage
		err := surrealcbor.Unmarshal(cborData, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, cborData, []byte(rawMsg))

		// Verify we can unmarshal it
		var decoded []any
		err = cbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)
		assert.Len(t, decoded, 4)
		assert.Equal(t, uint64(1), decoded[0])
		assert.Equal(t, uint64(2), decoded[1])
		assert.Equal(t, uint64(3), decoded[2])
		assert.Equal(t, "abc", decoded[3])
	})

	t.Run("decode indefinite length map into RawMessage", func(t *testing.T) {
		// Create indefinite length map
		// CBOR: 0xbf (map, indefinite), key-value pairs..., 0xff (break)
		cborData := []byte{
			0xbf,      // Indefinite map start
			0x61, 'a', // Key: "a"
			0x01,      // Value: 1
			0x61, 'b', // Key: "b"
			0x02, // Value: 2
			0xff, // Break marker
		}

		var rawMsg cbor.RawMessage
		err := surrealcbor.Unmarshal(cborData, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, cborData, []byte(rawMsg))

		// Verify we can unmarshal it
		var decoded map[string]any
		err = cbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), decoded["a"])
		assert.Equal(t, uint64(2), decoded["b"])
	})
}

func TestDecodeRawMessageInSlice(t *testing.T) {
	t.Run("decode slice of RawMessages", func(t *testing.T) {
		// Create test data
		data1, _ := cbor.Marshal(42)
		data2, _ := cbor.Marshal("hello")
		data3, _ := cbor.Marshal(map[string]int{"key": 100})

		sliceData, err := cbor.Marshal([]cbor.RawMessage{
			cbor.RawMessage(data1),
			cbor.RawMessage(data2),
			cbor.RawMessage(data3),
		})
		require.NoError(t, err)

		var decoded []cbor.RawMessage
		err = surrealcbor.Unmarshal(sliceData, &decoded)
		require.NoError(t, err)

		assert.Len(t, decoded, 3)
		assert.Equal(t, data1, []byte(decoded[0]))
		assert.Equal(t, data2, []byte(decoded[1]))
		assert.Equal(t, data3, []byte(decoded[2]))

		// Verify we can unmarshal each RawMessage
		var val1 int
		var val2 string
		var val3 map[string]int

		err = cbor.Unmarshal(decoded[0], &val1)
		require.NoError(t, err)
		assert.Equal(t, 42, val1)

		err = cbor.Unmarshal(decoded[1], &val2)
		require.NoError(t, err)
		assert.Equal(t, "hello", val2)

		err = cbor.Unmarshal(decoded[2], &val3)
		require.NoError(t, err)
		assert.Equal(t, 100, val3["key"])
	})
}

// TestRawMessagePointerNotSupported verifies that *cbor.RawMessage is not supported
// Users should use cbor.RawMessage directly since it's already a reference type ([]byte)
func TestRawMessagePointerNotSupported(t *testing.T) {
	t.Run("use cbor.RawMessage instead of *cbor.RawMessage", func(t *testing.T) {
		// cbor.RawMessage is already a reference type ([]byte), so there's no need
		// to use a pointer to it. This test demonstrates the correct usage.

		data, err := cbor.Marshal("test string")
		require.NoError(t, err)

		// Correct usage: cbor.RawMessage (not *cbor.RawMessage)
		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))

		var value string
		err = cbor.Unmarshal(rawMsg, &value)
		require.NoError(t, err)
		assert.Equal(t, "test string", value)
	})

	t.Run("cbor.RawMessage handles nil correctly", func(t *testing.T) {
		data, err := cbor.Marshal(nil)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		// RawMessage contains the CBOR encoding of nil
		assert.Equal(t, []byte{0xf6}, []byte(rawMsg))
	})
}

func TestDecodeAllTypeMapIntoRawMessage(t *testing.T) {
	t.Run("decode map with all types into RawMessage", func(t *testing.T) {
		testMap := map[string]any{
			"int":    42,
			"int64":  math.MaxInt64,
			"float":  3.14,
			"string": "hello",
			"bool":   true,
			"null":   nil,
			"array":  []any{1, 2, 3},
			"object": map[string]any{"key": "value"},
			"table":  models.Table("users"),
			"record": models.NewRecordID("users", 123),
		}

		// Use our Marshal to properly encode SurrealDB types with tags
		data, err := surrealcbor.Marshal(testMap)
		require.NoError(t, err)

		var rawMsg cbor.RawMessage
		err = surrealcbor.Unmarshal(data, &rawMsg)
		require.NoError(t, err)

		assert.Equal(t, data, []byte(rawMsg))

		var decoded map[string]any
		// Use our own Unmarshal to properly decode SurrealDB types
		err = surrealcbor.Unmarshal(rawMsg, &decoded)
		require.NoError(t, err)

		assert.Equal(t, uint64(42), decoded["int"])
		assert.Equal(t, uint64(math.MaxInt64), decoded["int64"])
		assert.Equal(t, 3.14, decoded["float"])
		assert.Equal(t, "hello", decoded["string"])
		assert.Equal(t, true, decoded["bool"])
		assert.Nil(t, decoded["null"])

		// Check array
		arr, ok := decoded["array"].([]any)
		require.True(t, ok, "array should be []any")
		assert.Equal(t, []any{uint64(1), uint64(2), uint64(3)}, arr)

		// Check nested object
		obj, ok := decoded["object"].(map[string]any)
		require.True(t, ok, "object should be map[string]any")
		assert.Equal(t, "value", obj["key"])

		// Check SurrealDB types
		assert.Equal(t, models.Table("users"), decoded["table"])
		// RecordID.ID will be decoded as uint64 since CBOR encodes positive integers as unsigned
		assert.Equal(t, models.RecordID{Table: "users", ID: uint64(123)}, decoded["record"])
	})
}
