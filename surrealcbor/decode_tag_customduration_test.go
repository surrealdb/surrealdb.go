package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_customDuration tests custom duration decoding
func TestDecode_customDuration(t *testing.T) {
	t.Run("decode duration tag", func(t *testing.T) {
		// Tag 12 (duration)
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  12,
			Content: "1h30m",
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		// Tag 12 is not specifically handled, falls back to datetime handling
		assert.Error(t, err)
	})

	t.Run("decode duration tag with invalid string", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  12,
			Content: "invalid",
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		// Tag 12 not specifically handled
		assert.Error(t, err)
	})

	t.Run("decode duration tag with non-string", func(t *testing.T) {
		enc, _ := cbor.Marshal(cbor.Tag{
			Number:  12,
			Content: 123,
		})

		var dur models.CustomDuration
		err := Unmarshal(enc, &dur)
		// Tag 12 not specifically handled
		assert.Error(t, err)
	})
}
