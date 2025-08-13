package surrealcbor

import (
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestCompatibilityWithFxamacker tests that data encoded with fxamacker can be decoded with our implementation
func TestCompatibilityWithFxamacker(t *testing.T) {
	// Create test data
	testData := map[string]any{
		"string": "hello",
		"number": 42,
		"float":  3.14,
		"bool":   true,
		"null":   nil,
		"array":  []any{1, 2, 3},
		"object": map[string]any{"key": "value"},
		"table":  models.Table("users"),
		"record": models.NewRecordID("users", 123),
	}

	// Encode with fxamacker
	em := getEncMode()
	fxData, err := em.Marshal(testData)
	require.NoError(t, err, "fxamacker Marshal failed")

	// Decode with our implementation
	var ourDecoded map[string]any
	err = Unmarshal(fxData, &ourDecoded)
	require.NoError(t, err, "Our Unmarshal failed")

	// Verify basic types
	assert.Equal(t, "hello", ourDecoded["string"], "String mismatch")

	// Handle both int64 and uint64 for number comparison
	switch v := ourDecoded["number"].(type) {
	case int64:
		assert.Equal(t, int64(42), v, "Number mismatch")
	case uint64:
		assert.Equal(t, uint64(42), v, "Number mismatch")
	default:
		t.Errorf("Number type mismatch: got %T, want int64 or uint64", ourDecoded["number"])
	}

	assert.Equal(t, true, ourDecoded["bool"], "Bool mismatch")
	assert.Nil(t, ourDecoded["null"], "Null should be nil")

	// Test that we can also encode with our Marshal and decode with fxamacker (except for None)
	ourData, err := Marshal(map[string]any{
		"test": "value",
		"num":  100,
	})
	require.NoError(t, err, "Our Marshal failed")

	dm, _ := cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any(nil)),
	}.DecMode()

	var fxDecoded map[string]any
	err = dm.Unmarshal(ourData, &fxDecoded)
	require.NoError(t, err, "fxamacker Unmarshal of our data failed")

	assert.Equal(t, "value", fxDecoded["test"], "Test value mismatch")
}
