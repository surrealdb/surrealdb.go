package surrealcbor

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// CustomSimpleString is a simple custom type based on string
type CustomSimpleString string

// UnmarshalCBOR implements custom unmarshaling for CustomSimpleString
func (cs *CustomSimpleString) UnmarshalCBOR(data []byte) error {
	// Parse CBOR string and add custom prefix
	var str string
	if err := cbor.Unmarshal(data, &str); err != nil {
		return err
	}
	*cs = CustomSimpleString("custom:" + str)
	return nil
}

// MarshalCBOR implements custom marshaling for CustomSimpleString
func (cs CustomSimpleString) MarshalCBOR() ([]byte, error) {
	// Remove custom prefix when marshaling
	str := strings.TrimPrefix(string(cs), "custom:")
	return cbor.Marshal(str)
}

// CustomRecordID is a custom type that wraps models.RecordID
type CustomRecordID struct {
	models.RecordID
	Metadata string // Additional field
}

// UnmarshalCBOR implements custom unmarshaling for CustomRecordID
func (cr *CustomRecordID) UnmarshalCBOR(data []byte) error {
	// First unmarshal as regular RecordID using the decoder
	dec := NewDecoder(bytes.NewReader(data))
	var rid models.RecordID
	if err := dec.Decode(&rid); err != nil {
		return err
	}

	cr.RecordID = rid
	cr.Metadata = fmt.Sprintf("unmarshaled:%s:%v", rid.Table, rid.ID)
	return nil
}

// MarshalCBOR implements custom marshaling for CustomRecordID
func (cr CustomRecordID) MarshalCBOR() ([]byte, error) {
	// Marshal only the RecordID part with tag
	enc := NewEncoder(bytes.NewBuffer(nil))
	if err := enc.Encode(cr.RecordID); err != nil {
		return nil, err
	}

	// Get the bytes from the encoder's writer
	buf := enc.w.(*bytes.Buffer)
	return buf.Bytes(), nil
}

// CustomComplexType demonstrates a more complex custom type
type CustomComplexType struct {
	Value  string
	Count  int
	Nested map[string]interface{}
}

// UnmarshalCBOR implements custom unmarshaling for CustomComplexType
func (ct *CustomComplexType) UnmarshalCBOR(data []byte) error {
	// Unmarshal as a map and convert
	var m map[string]interface{}
	if err := cbor.Unmarshal(data, &m); err != nil {
		return err
	}

	// Custom conversion logic
	if v, ok := m["value"].(string); ok {
		ct.Value = "complex:" + v
	}
	if c, ok := m["count"].(uint64); ok {
		// Check for overflow before converting and doubling
		const maxInt = int(^uint(0) >> 1)
		if c > uint64(maxInt)/2 {
			return fmt.Errorf("count value %d would overflow int when doubled", c)
		}
		ct.Count = int(c) * 2 // Double the count as custom behavior
	}
	if n, ok := m["nested"].(map[interface{}]interface{}); ok {
		ct.Nested = make(map[string]interface{})
		for k, v := range n {
			if ks, ok := k.(string); ok {
				ct.Nested[ks] = v
			}
		}
	}

	return nil
}

// MarshalCBOR implements custom marshaling for CustomComplexType
func (ct CustomComplexType) MarshalCBOR() ([]byte, error) {
	// Create a map for marshaling
	m := map[string]interface{}{
		"value":  strings.TrimPrefix(ct.Value, "complex:"),
		"count":  ct.Count / 2, // Halve the count when marshaling
		"nested": ct.Nested,
	}
	return cbor.Marshal(m)
}

func TestCustomUnmarshalSimpleString(t *testing.T) {
	// Create test data
	original := "hello"
	encoded, err := cbor.Marshal(original)
	require.NoError(t, err)

	// Decode with custom unmarshaler
	dec := NewDecoder(bytes.NewReader(encoded))
	var custom CustomSimpleString
	err = dec.Decode(&custom)
	require.NoError(t, err)

	// Should have custom prefix
	assert.Equal(t, CustomSimpleString("custom:hello"), custom)

	// Test marshaling back
	marshaled, err := custom.MarshalCBOR()
	require.NoError(t, err)

	var back string
	err = cbor.Unmarshal(marshaled, &back)
	require.NoError(t, err)
	assert.Equal(t, original, back)
}

func TestCustomUnmarshalRecordID(t *testing.T) {
	// Create a RecordID and encode it
	original := models.RecordID{
		Table: "users",
		ID:    "john",
	}

	enc := NewEncoder(bytes.NewBuffer(nil))
	err := enc.Encode(original)
	require.NoError(t, err)
	encoded := enc.w.(*bytes.Buffer).Bytes()

	// Decode with custom unmarshaler
	dec := NewDecoder(bytes.NewReader(encoded))
	var custom CustomRecordID
	err = dec.Decode(&custom)
	require.NoError(t, err)

	// Check the unmarshaled values
	assert.Equal(t, original.Table, custom.Table)
	assert.Equal(t, original.ID, custom.ID)
	assert.Equal(t, "unmarshaled:users:john", custom.Metadata)

	// Test marshaling back
	marshaled, err := custom.MarshalCBOR()
	require.NoError(t, err)

	// Should be able to decode back to regular RecordID
	dec2 := NewDecoder(bytes.NewReader(marshaled))
	var back models.RecordID
	err = dec2.Decode(&back)
	require.NoError(t, err)
	assert.Equal(t, original, back)
}

func TestCustomUnmarshalComplexType(t *testing.T) {
	// Create test data as a map
	testData := map[string]interface{}{
		"value": "test",
		"count": uint64(5),
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	encoded, err := cbor.Marshal(testData)
	require.NoError(t, err)

	// Decode with custom unmarshaler
	dec := NewDecoder(bytes.NewReader(encoded))
	var custom CustomComplexType
	err = dec.Decode(&custom)
	require.NoError(t, err)

	// Check custom transformations
	assert.Equal(t, "complex:test", custom.Value)
	assert.Equal(t, 10, custom.Count) // 5 * 2
	assert.Equal(t, "value1", custom.Nested["key1"])
	assert.Equal(t, uint64(42), custom.Nested["key2"])

	// Test marshaling back
	marshaled, err := custom.MarshalCBOR()
	require.NoError(t, err)

	var back map[string]interface{}
	err = cbor.Unmarshal(marshaled, &back)
	require.NoError(t, err)
	assert.Equal(t, "test", back["value"])
	assert.Equal(t, uint64(5), back["count"]) // 10 / 2
}

func TestCustomUnmarshalPointerReceiver(t *testing.T) {
	// Test that pointer receiver works correctly
	str := "pointer_test"
	encoded, err := cbor.Marshal(str)
	require.NoError(t, err)

	dec := NewDecoder(bytes.NewReader(encoded))
	custom := new(CustomSimpleString)
	err = dec.Decode(custom)
	require.NoError(t, err)

	assert.Equal(t, CustomSimpleString("custom:pointer_test"), *custom)
}

func TestCustomUnmarshalInStruct(t *testing.T) {
	// Test custom unmarshalers within a struct
	type Container struct {
		Simple  CustomSimpleString
		Complex CustomComplexType
		// Skip testing embedded RecordID due to tag complexity in nested structures
	}

	testData := map[string]interface{}{
		"Simple": "embedded",
		"Complex": map[string]interface{}{
			"value": "nested",
			"count": uint64(3),
			"nested": map[string]interface{}{
				"inner": "data",
			},
		},
	}

	// Encode the test data
	encoded, err := cbor.Marshal(testData)
	require.NoError(t, err)

	// Decode the container
	dec := NewDecoder(bytes.NewReader(encoded))
	var container Container
	err = dec.Decode(&container)
	require.NoError(t, err)

	// Verify custom unmarshaling worked
	assert.Equal(t, CustomSimpleString("custom:embedded"), container.Simple)
	assert.Equal(t, "complex:nested", container.Complex.Value)
	assert.Equal(t, 6, container.Complex.Count)
}

func TestCustomUnmarshalArray(t *testing.T) {
	// Test custom unmarshalers in arrays
	testData := []string{"one", "two", "three"}
	encoded, err := cbor.Marshal(testData)
	require.NoError(t, err)

	dec := NewDecoder(bytes.NewReader(encoded))
	var customs []CustomSimpleString
	err = dec.Decode(&customs)
	require.NoError(t, err)

	assert.Len(t, customs, 3)
	assert.Equal(t, CustomSimpleString("custom:one"), customs[0])
	assert.Equal(t, CustomSimpleString("custom:two"), customs[1])
	assert.Equal(t, CustomSimpleString("custom:three"), customs[2])
}

func TestCustomUnmarshalMap(t *testing.T) {
	// Test custom unmarshalers in map values
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	encoded, err := cbor.Marshal(testData)
	require.NoError(t, err)

	dec := NewDecoder(bytes.NewReader(encoded))
	var customs map[string]CustomSimpleString
	err = dec.Decode(&customs)
	require.NoError(t, err)

	assert.Len(t, customs, 2)
	assert.Equal(t, CustomSimpleString("custom:value1"), customs["key1"])
	assert.Equal(t, CustomSimpleString("custom:value2"), customs["key2"])
}

// TestCustomUnmarshalFallback verifies that types without UnmarshalCBOR
// still work with the default decoder
func TestCustomUnmarshalFallback(t *testing.T) {
	type RegularStruct struct {
		Name  string
		Value int
	}

	testData := RegularStruct{
		Name:  "test",
		Value: 42,
	}

	encoded, err := cbor.Marshal(testData)
	require.NoError(t, err)

	dec := NewDecoder(bytes.NewReader(encoded))
	var decoded RegularStruct
	err = dec.Decode(&decoded)
	require.NoError(t, err)

	assert.Equal(t, testData, decoded)
}
