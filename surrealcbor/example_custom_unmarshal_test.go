package surrealcbor_test

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// MyCustomType demonstrates a custom type with its own CBOR marshaling/unmarshaling logic
type MyCustomType struct {
	value string
}

// UnmarshalCBOR implements custom CBOR unmarshaling
func (m *MyCustomType) UnmarshalCBOR(data []byte) error {
	var str string
	if err := cbor.Unmarshal(data, &str); err != nil {
		return err
	}
	// Add custom processing during unmarshaling
	m.value = "Processed: " + str
	return nil
}

// MarshalCBOR implements custom CBOR marshaling
func (m MyCustomType) MarshalCBOR() ([]byte, error) {
	// Remove the prefix when marshaling
	str := m.value
	if len(str) > 11 && str[:11] == "Processed: " {
		str = str[11:]
	}
	return cbor.Marshal(str)
}

func (m MyCustomType) String() string {
	return m.value
}

func Example_customUnmarshalCBOR() {
	// Original data
	original := "Hello, World!"

	// Encode the string
	encoded, err := cbor.Marshal(original)
	if err != nil {
		panic(err)
	}

	// Decode into our custom type using surrealcbor decoder
	dec := surrealcbor.NewDecoder(bytes.NewReader(encoded))
	var custom MyCustomType
	if err := dec.Decode(&custom); err != nil {
		panic(err)
	}

	fmt.Println(custom) // Will print with "Processed: " prefix

	// Marshal it back using surrealcbor encoder
	var buf bytes.Buffer
	enc := surrealcbor.NewEncoder(&buf)
	if err := enc.Encode(custom); err != nil {
		panic(err)
	}

	// Decode the marshaled data to verify it's back to original
	var decoded string
	if err := cbor.Unmarshal(buf.Bytes(), &decoded); err != nil {
		panic(err)
	}
	fmt.Println(decoded) // Should be the original string

	// Output:
	// Processed: Hello, World!
	// Hello, World!
}

