package models

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

func getTaggedContent(data []byte, tagNumber uint64) ([]byte, error) {
	var tag cbor.RawTag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return nil, err
	}

	if tag.Number != tagNumber {
		return nil, fmt.Errorf("unexpected tag number: got %d, want %d", tag.Number, tagNumber)
	}

	// Note that the below is impossible due to `invalid composite literal type T`:
	//   var tag cbor.Tag
	//   cbor.Unmarshal(data, &tag)
	//   ...
	//   v, ok := tag.Content.(T)
	// So all we can do is unmarshal once more into a temporary variable of type T.
	// This is a workaround for the fact that cbor.Tag do not carry type information for Content.

	// Although this looks marshaling the unmarshaled data againn which might be inefficient,
	// this is actually not the case because cbor.Tag.Content(RawMessage) is already a raw byte slice
	// and RawMessage.MarshalCBOR() just returns the raw bytes without any additional encoding.
	contentData, err := tag.Content.MarshalCBOR()
	if err != nil {
		return nil, fmt.Errorf("failed to extract the raw bytes from cbor tag content: %w", err)
	}

	return contentData, nil
}
