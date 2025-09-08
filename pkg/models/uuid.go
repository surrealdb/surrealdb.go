package models

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
)

type UUIDString string

// UUID represents a UUID v4 or v7 value.
//
// It implements cbor.Marshaler and cbor.Unmarshaler to handle
// CBOR encoding/decoding with tag 37 as specified by SurrealDB.
//
// Please see the [data type documentation] and the [CBOR tag documentation] for details.
//
// [data type documentation]: https://surrealdb.com/docs/surrealql/datamodel/uuid
// [CBOR tag documentation]: https://surrealdb.com/docs/surrealdb/integration/cbor#tag-37
type UUID struct {
	uuid.UUID
}

// MarshalCBOR implements cbor.Marshaler interface for UUID
func (u UUID) MarshalCBOR() ([]byte, error) {
	// Tag 37 is for UUID
	return cbor.Marshal(cbor.Tag{
		Number:  TagSpecBinaryUUID,
		Content: u.Bytes(),
	})
}

// UnmarshalCBOR implements cbor.Unmarshaler interface for UUID
func (u *UUID) UnmarshalCBOR(data []byte) error {
	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return err
	}

	if tag.Number != TagSpecBinaryUUID {
		return fmt.Errorf("unexpected tag number for UUID: got %d, want %d", tag.Number, TagSpecBinaryUUID)
	}

	bytes, ok := tag.Content.([]byte)
	if !ok {
		return fmt.Errorf("UUID tag content must be byte string, got %T", tag.Content)
	}

	// Both UUID v4 and v7 are 16 bytes
	if len(bytes) != 16 {
		return fmt.Errorf("UUID must be exactly 16 bytes, got %d", len(bytes))
	}

	parsed, err := uuid.FromBytes(bytes)
	if err != nil {
		return fmt.Errorf("failed to parse UUID bytes: %w", err)
	}

	u.UUID = parsed
	return nil
}
