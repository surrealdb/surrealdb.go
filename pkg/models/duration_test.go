package models

import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

func TestDuration_cbor_roundtrip(t *testing.T) {
	d := CustomDuration{time.Hour + 30*time.Minute + 15*time.Second + 1234567890*time.Nanosecond}
	data, err := cbor.Marshal(d)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var d2 CustomDuration
	assert.NoError(t, cbor.Unmarshal(data, &d2), "failed to unmarshal CustomDuration")
	assert.Equal(t, d, d2, "unmarshaled CustomDuration does not match original")

	surrealData, err := getCborEncoder().Marshal(d)
	assert.NoError(t, err, "failed to marshal CustomDuration with surreal cbor")
	var surrealDecoded CustomDuration
	assert.NoError(t, getCborDecoder().Unmarshal(surrealData, &surrealDecoded),
		"failed to unmarshal CustomDuration with surreal cbor")
	assert.Equal(t, d, surrealDecoded, "unmarshaled CustomDuration with surreal cbor does not match original")

	cborData, err := cbor.Marshal(d)
	assert.NoError(t, err, "failed to marshal CustomDuration with cbor")
	var cborDecoded CustomDuration
	assert.NoError(t, cbor.Unmarshal(cborData, &cborDecoded), "failed to unmarshal CustomDuration with cbor")
	assert.Equal(t, d, cborDecoded, "unmarshaled CustomDuration with cbor does not match original")
}
