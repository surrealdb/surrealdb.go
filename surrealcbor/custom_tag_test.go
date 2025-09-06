package surrealcbor

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CustomTaggedType wants to handle its own CBOR tag
type CustomTaggedType struct {
	Value   string
	TagNum  uint64
	Wrapped bool
}

// UnmarshalCBOR can handle tagged CBOR data
func (ct *CustomTaggedType) UnmarshalCBOR(data []byte) error {
	// Check if data starts with a tag
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	majorType := data[0] >> 5
	if majorType == 6 { // CBOR tag
		// This is tagged data - parse the tag
		var tag cbor.Tag
		if err := cbor.Unmarshal(data, &tag); err != nil {
			return err
		}

		ct.TagNum = tag.Number
		ct.Wrapped = true

		// Extract the content
		if str, ok := tag.Content.(string); ok {
			ct.Value = "tagged:" + str
		} else {
			ct.Value = fmt.Sprintf("tagged:<%T>", tag.Content)
		}
	} else {
		// Not tagged, just unmarshal as string
		var str string
		if err := cbor.Unmarshal(data, &str); err != nil {
			return err
		}
		ct.Value = "untagged:" + str
		ct.TagNum = 0
		ct.Wrapped = false
	}

	return nil
}

// MarshalCBOR can produce tagged CBOR data
func (ct CustomTaggedType) MarshalCBOR() ([]byte, error) {
	if ct.Wrapped {
		// Re-wrap with the original tag
		cleanValue := ct.Value
		if len(cleanValue) > 7 && cleanValue[:7] == "tagged:" {
			cleanValue = cleanValue[7:]
		}
		return cbor.Marshal(cbor.Tag{
			Number:  ct.TagNum,
			Content: cleanValue,
		})
	}

	// No tag, just marshal the value
	cleanValue := ct.Value
	if len(cleanValue) > 9 && cleanValue[:9] == "untagged:" {
		cleanValue = cleanValue[9:]
	}
	return cbor.Marshal(cleanValue)
}

func TestCustomTypeWithTag(t *testing.T) {
	t.Run("unmarshal tagged value", func(t *testing.T) {
		// Create a tagged CBOR value (tag 100 wrapping string "hello")
		tagged := cbor.Tag{
			Number:  100,
			Content: "hello",
		}
		encoded, err := cbor.Marshal(tagged)
		require.NoError(t, err)

		// Decode with our custom type
		dec := NewDecoder(bytes.NewReader(encoded))
		var custom CustomTaggedType
		err = dec.Decode(&custom)
		require.NoError(t, err)

		// Verify it detected and handled the tag
		assert.True(t, custom.Wrapped)
		assert.Equal(t, uint64(100), custom.TagNum)
		assert.Equal(t, "tagged:hello", custom.Value)

		// Marshal it back
		marshaled, err := custom.MarshalCBOR()
		require.NoError(t, err)

		// Verify it's still tagged
		var backTag cbor.Tag
		err = cbor.Unmarshal(marshaled, &backTag)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), backTag.Number)
		assert.Equal(t, "hello", backTag.Content)
	})

	t.Run("unmarshal untagged value", func(t *testing.T) {
		// Create an untagged CBOR string
		encoded, err := cbor.Marshal("world")
		require.NoError(t, err)

		// Decode with our custom type
		dec := NewDecoder(bytes.NewReader(encoded))
		var custom CustomTaggedType
		err = dec.Decode(&custom)
		require.NoError(t, err)

		// Verify it detected no tag
		assert.False(t, custom.Wrapped)
		assert.Equal(t, uint64(0), custom.TagNum)
		assert.Equal(t, "untagged:world", custom.Value)

		// Marshal it back
		marshaled, err := custom.MarshalCBOR()
		require.NoError(t, err)

		// Verify it's still untagged
		var backStr string
		err = cbor.Unmarshal(marshaled, &backStr)
		require.NoError(t, err)
		assert.Equal(t, "world", backStr)
	})
}

// CustomRecordIDWrapper wants to handle RecordID with a different tag
type CustomRecordIDWrapper struct {
	Table string
	ID    interface{}
}

func (cr *CustomRecordIDWrapper) UnmarshalCBOR(data []byte) error {
	// We get the full CBOR data including any tags
	// Let's say we want to accept both tag 8 (standard RecordID)
	// and tag 200 (our custom variant)
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	majorType := data[0] >> 5
	if majorType == 6 { // Tagged
		var tag cbor.Tag
		if err := cbor.Unmarshal(data, &tag); err != nil {
			return err
		}

		// Accept both standard and custom tag
		if tag.Number != 8 && tag.Number != 200 {
			return fmt.Errorf("expected tag 8 or 200, got %d", tag.Number)
		}

		// Parse the array content
		if arr, ok := tag.Content.([]interface{}); ok && len(arr) == 2 {
			if table, ok := arr[0].(string); ok {
				cr.Table = table
				cr.ID = arr[1]
			}
		}
	}

	return nil
}

func (cr CustomRecordIDWrapper) MarshalCBOR() ([]byte, error) {
	// Always use our custom tag 200
	return cbor.Marshal(cbor.Tag{
		Number:  200,
		Content: []interface{}{cr.Table, cr.ID},
	})
}

func TestCustomRecordIDWrapper(t *testing.T) {
	// Create a standard RecordID with tag 8
	standardTag := cbor.Tag{
		Number:  8,
		Content: []interface{}{"users", "alice"},
	}
	encoded, err := cbor.Marshal(standardTag)
	require.NoError(t, err)

	// Decode with our custom wrapper
	dec := NewDecoder(bytes.NewReader(encoded))
	var custom CustomRecordIDWrapper
	err = dec.Decode(&custom)
	require.NoError(t, err)

	assert.Equal(t, "users", custom.Table)
	assert.Equal(t, "alice", custom.ID)

	// Marshal it back - should use tag 200
	marshaled, err := custom.MarshalCBOR()
	require.NoError(t, err)

	var backTag cbor.Tag
	err = cbor.Unmarshal(marshaled, &backTag)
	require.NoError(t, err)
	assert.Equal(t, uint64(200), backTag.Number) // Our custom tag!
}
