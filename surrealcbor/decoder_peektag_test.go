package surrealcbor

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecoder_peekTag(t *testing.T) {
	t.Run("peekTag with EOF", func(t *testing.T) {
		d := &decoder{data: []byte{}, pos: 0}
		tag, err := d.peekTag()
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, uint64(0), tag)
	})

	t.Run("peekTag with non-tag", func(t *testing.T) {
		d := &decoder{data: []byte{0x01}, pos: 0}
		tag, err := d.peekTag()
		assert.Error(t, err)
		assert.Equal(t, uint64(0), tag)
	})
}
