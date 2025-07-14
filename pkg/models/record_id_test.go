package models

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordID_cbor_roundtrip(t *testing.T) {
	testcases := []struct {
		name string
		rid  RecordID
	}{
		{
			name: "string ID",
			rid: RecordID{
				Table: "test_table",
				ID:    "test_id",
			},
		},
		{
			name: "number-like string ID",
			rid: RecordID{
				Table: "test_table",
				ID:    "12345",
			},
		},
		{
			name: "numeric ID",
			rid: RecordID{
				Table: "test_table",
				// Note that int64(12345) result in a test failure because it is unmarshalled as uint64(0x3039)
				ID: uint64(12345),
			},
		},
		{
			name: "numeric negative ID",
			rid: RecordID{
				Table: "test_table",
				ID:    int64(-12345),
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := cbor.Marshal(tc.rid)
			require.NoError(t, err, "failed to marshal RecordID")

			var decoded RecordID
			require.NoError(t, cbor.Unmarshal(data, &decoded), "failed to unmarshal RecordID")
			assert.Equal(t, tc.rid.Table, decoded.Table, "unmarshaled RecordID Table does not match original")
			assert.Equal(t, tc.rid.ID, decoded.ID, "unmarshaled RecordID ID does not match original")

			cborData, err := cbor.Marshal(tc.rid)
			require.NoError(t, err, "failed to marshal RecordID with cbor")

			var cborDecoded RecordID
			err = cbor.Unmarshal(cborData, &cborDecoded)
			require.NoError(t, err, "failed to unmarshal RecordID with cbor")
			assert.Equal(t, tc.rid.Table, cborDecoded.Table, "unmarshaled RecordID with cbor Table does not match original")
			assert.Equal(t, tc.rid.ID, cborDecoded.ID, "unmarshaled RecordID with cbor ID does not match original")

			surrealData, err := getCborEncoder().Marshal(tc.rid)
			require.NoError(t, err, "failed to marshal RecordID with surreal cbor")

			var surrealDecoded RecordID
			err = getCborDecoder().Unmarshal(surrealData, &surrealDecoded)
			require.NoError(t, err, "failed to unmarshal RecordID with surreal cbor")
			assert.Equal(t, tc.rid.Table, surrealDecoded.Table,
				"unmarshaled RecordID with surreal cbor Table does not match original")
			assert.Equal(t, tc.rid.ID, surrealDecoded.ID, "unmarshaled RecordID with surreal cbor ID does not match original")
		})
	}
}
