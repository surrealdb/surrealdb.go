package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_geometryPoint tests decoding of geometry points
func TestDecode_geometryPoint(t *testing.T) {
	// Tag 88 (geometry point)
	// Geometry point expects an array [lat, lng]
	enc, _ := cbor.Marshal(cbor.Tag{
		Number:  88,
		Content: []float64{1.23, 4.56},
	})

	var point models.GeometryPoint
	err := Unmarshal(enc, &point)
	require.NoError(t, err)
	// GeometryPoint stores as [Longitude, Latitude]
	assert.Equal(t, 4.56, point.Latitude)
	assert.Equal(t, 1.23, point.Longitude)
}

// TestDecode_geometryPointInvalidContent tests invalid content for geometry point
func TestDecode_geometryPointInvalidContent(t *testing.T) {
	enc, _ := cbor.Marshal(cbor.Tag{
		Number:  88,
		Content: "not-a-point",
	})

	var point models.GeometryPoint
	err := Unmarshal(enc, &point)
	// Expects an array
	assert.Error(t, err)
}

// TestDecode_geometryPointAny tests decoding of geometry point into any
func TestDecode_geometryPointAny(t *testing.T) {
	enc, _ := cbor.Marshal(cbor.Tag{
		Number:  88,
		Content: []float64{1.23, 4.56},
	})

	var point any
	err := Unmarshal(enc, &point)
	require.NoError(t, err)

	// Expecting a GeometryPoint type
	gp, ok := point.(models.GeometryPoint)
	require.True(t, ok, "Decoded value should be of type GeometryPoint")
	assert.Equal(t, 4.56, gp.Latitude)
	assert.Equal(t, 1.23, gp.Longitude)
}

// TestDecode_geometryPointAny tests decoding of geometry point into any
func TestDecode_geometryPointAnyPointer(t *testing.T) {
	enc, _ := cbor.Marshal(cbor.Tag{
		Number:  88,
		Content: []float64{1.23, 4.56},
	})

	var point *any
	err := Unmarshal(enc, &point)
	require.NoError(t, err)

	// Expecting a GeometryPoint type
	gp, ok := (*point).(models.GeometryPoint)
	require.True(t, ok, "Decoded value should be of type GeometryPoint")
	assert.Equal(t, 4.56, gp.Latitude)
	assert.Equal(t, 1.23, gp.Longitude)
}
