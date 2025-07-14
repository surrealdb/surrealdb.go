package models

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateTime_cbor_roundtrip(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name string
		dt   CustomDateTime
	}{
		{
			name: "current time",
			// .UTC() ensures the monotonic clock is stripped.
			// Otherwise, the test fails with diff like this:
			// -(time.Time) 2025-07-14 12:31:59.611364256 +0000 UTC m=+0.000407643
			// +(time.Time) 2025-07-14 12:31:59.611364256 +0000 UTC
			dt: CustomDateTime{Time: time.Now().UTC()},
		},
		{
			name: "specific time",
			dt:   CustomDateTime{Time: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.dt.MarshalCBOR()
			require.NoError(t, err, "failed to marshal CustomDateTime")

			var decoded CustomDateTime
			require.NoError(t, decoded.UnmarshalCBOR(data), "failed to unmarshal CustomDateTime")
			assert.Equal(t, tc.dt.Time, decoded.Time)

			cborData, err := cbor.Marshal(tc.dt)
			require.NoError(t, err, "failed to marshal CustomDateTime with cbor")

			var decodedCbor CustomDateTime
			err = cbor.Unmarshal(cborData, &decodedCbor)
			require.NoError(t, err, "failed to unmarshal CustomDateTime with cbor")
			assert.Equal(t, tc.dt.Time, decodedCbor.Time)

			surrealCborData, err := getCborEncoder().Marshal(tc.dt)
			require.NoError(t, err, "failed to marshal CustomDateTime with surreal cbor")

			var decodedSurreal CustomDateTime
			err = getCborDecoder().Unmarshal(surrealCborData, &decodedSurreal)
			require.NoError(t, err, "failed to unmarshal CustomDateTime with surreal cbor")
			assert.Equal(t, tc.dt.Time, decodedSurreal.Time)
		})
	}
}

// TestDateTime_cbor_local_time tests that the CustomDateTime can handle local time correctly.
// SurrealDB stores all times in UTC without the time zone information.
// So, all the unmarshal function can do is to parse it as UTC.
func TestDateTime_cbor_local_time(t *testing.T) {
	t.Parallel()

	localTime := time.Date(2023, 10, 1, 12, 0, 0, 0, time.Local)
	customDT := CustomDateTime{Time: localTime}

	data, err := customDT.MarshalCBOR()
	if err != nil {
		t.Fatalf("failed to marshal CustomDateTime: %v", err)
	}

	var decoded CustomDateTime
	if err := decoded.UnmarshalCBOR(data); err != nil {
		t.Fatalf("failed to unmarshal CustomDateTime: %v", err)
	}

	assert.Equal(t, localTime.UTC(), decoded.Time, "unmarshaled CustomDateTime does not match original")
}
