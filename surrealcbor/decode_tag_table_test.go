package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecode_table(t *testing.T) {
	t.Run("decode table tag", func(t *testing.T) {
		// Tag 7 (table)
		data := []byte{0xC7, 0x65, 0x74, 0x61, 0x62, 0x6C, 0x65} // Tag 7 with "table"

		var table models.Table
		err := Unmarshal(data, &table)
		require.NoError(t, err)
		assert.Equal(t, "table", table.String())
	})
}
