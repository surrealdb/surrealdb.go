package models

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
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
			require.NoError(t, err)

			t.Run("UnmarshalCBOR", func(t *testing.T) {
				var dt CustomDateTime
				require.NoError(t, dt.UnmarshalCBOR(data))
				assert.Equal(t, toLocal(tc.dt), dt)
			})

			t.Run("cbor.Marshal", func(t *testing.T) {
				cborData, marshalErr := cbor.Marshal(tc.dt)
				require.NoError(t, marshalErr)
				assert.Equal(t, data, cborData)
			})

			t.Run("cbor.Unmarshal", func(t *testing.T) {
				var dt CustomDateTime
				err = cbor.Unmarshal(data, &dt)
				require.NoError(t, err)
				assert.Equal(t, toLocal(tc.dt), dt)
			})

			t.Run("CborEncoder.Marshal", func(t *testing.T) {
				surrealCborData, marshalErr := getCborEncoder().Marshal(tc.dt)
				require.NoError(t, marshalErr)
				assert.Equal(t, data, surrealCborData)
			})

			t.Run("CborDecoder.Unmarshal to CustomDateTime", func(t *testing.T) {
				var dt CustomDateTime
				err = getCborDecoder().Unmarshal(data, &dt)
				require.NoError(t, err)
				assert.Equal(t, toLocal(tc.dt), dt)
			})

			t.Run("CborDecoder.Unmarshal to time.Time", func(t *testing.T) {
				var dt time.Time
				err = getCborDecoder().Unmarshal(data, &dt)
				require.ErrorContains(t, err, "cbor: cannot unmarshal array into Go value of type time.Time")
			})

			t.Run("CborDecoder.Unmarshal to any", func(t *testing.T) {
				var dt any
				err = getCborDecoder().Unmarshal(data, &dt)
				require.NoError(t, err)
				assert.Equal(t, toLocal(tc.dt), dt)
			})

			t.Run("CborUnmarshaler.Unmarshal to CustomDateTime", func(t *testing.T) {
				var dt CustomDateTime
				err = (&CborUnmarshaler{}).Unmarshal(data, &dt)
				require.NoError(t, err)
				assert.Equal(t, toLocal(tc.dt), dt)
			})

			t.Run("CborUnmarshaler.Unmarshal to time.Time", func(t *testing.T) {
				var dt time.Time
				err = (&CborUnmarshaler{}).Unmarshal(data, &dt)
				// TODO This could probably be handled in the replacerAfterDecode
				// but for now we just check that it returns an error.
				require.ErrorContains(t, err, "cbor: cannot unmarshal array into Go value of type time.Time")
			})

			t.Run("CborUnmarshaler.Unmarshal to any", func(t *testing.T) {
				var dt any
				err = (&CborUnmarshaler{}).Unmarshal(data, &dt)
				require.NoError(t, err)
				assert.Equal(t, toLocal(tc.dt), dt)
			})
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

	assert.Equal(t, customDT, decoded, "unmarshaled CustomDateTime does not match original")
}

func toLocal(dt CustomDateTime) CustomDateTime {
	return CustomDateTime{Time: dt.In(time.Local)}
}
