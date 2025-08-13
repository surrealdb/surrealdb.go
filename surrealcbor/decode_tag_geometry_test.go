package surrealcbor

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestDecodeGeometryLineTag(t *testing.T) {
	t.Run("decode GeometryLine", func(t *testing.T) {
		// GeometryLine is an array of GeometryPoints
		points := []models.GeometryPoint{
			{Longitude: 1.0, Latitude: 2.0},
			{Longitude: 3.0, Latitude: 4.0},
			{Longitude: 5.0, Latitude: 6.0},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryLine,
			Content: points,
		})
		require.NoError(t, err)

		var result models.GeometryLine
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Equal(t, 1.0, result[0].Longitude)
		assert.Equal(t, 2.0, result[0].Latitude)
	})

	t.Run("decode into interface", func(t *testing.T) {
		points := []models.GeometryPoint{
			{Longitude: 10.0, Latitude: 20.0},
			{Longitude: 30.0, Latitude: 40.0},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryLine,
			Content: points,
		})
		require.NoError(t, err)

		var result any
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		line, ok := result.(models.GeometryLine)
		assert.True(t, ok)
		assert.Len(t, line, 2)
	})
}

func TestDecodeGeometryPolygonTag(t *testing.T) {
	t.Run("decode GeometryPolygon", func(t *testing.T) {
		// GeometryPolygon is an array of GeometryLines
		polygon := models.GeometryPolygon{
			models.GeometryLine{
				{Longitude: 0.0, Latitude: 0.0},
				{Longitude: 0.0, Latitude: 1.0},
				{Longitude: 1.0, Latitude: 1.0},
				{Longitude: 1.0, Latitude: 0.0},
				{Longitude: 0.0, Latitude: 0.0}, // Close the ring
			},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryPolygon,
			Content: polygon,
		})
		require.NoError(t, err)

		var result models.GeometryPolygon
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Len(t, result[0], 5)
	})
}

func TestDecodeGeometryMultiPointTag(t *testing.T) {
	t.Run("decode GeometryMultiPoint", func(t *testing.T) {
		multiPoint := models.GeometryMultiPoint{
			{Longitude: 1.0, Latitude: 2.0},
			{Longitude: 3.0, Latitude: 4.0},
			{Longitude: 5.0, Latitude: 6.0},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryMultiPoint,
			Content: multiPoint,
		})
		require.NoError(t, err)

		var result models.GeometryMultiPoint
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Equal(t, 3.0, result[1].Longitude)
		assert.Equal(t, 4.0, result[1].Latitude)
	})
}

func TestDecodeGeometryMultiLineTag(t *testing.T) {
	t.Run("decode GeometryMultiLine", func(t *testing.T) {
		multiLine := models.GeometryMultiLine{
			models.GeometryLine{
				{Longitude: 1.0, Latitude: 2.0},
				{Longitude: 3.0, Latitude: 4.0},
			},
			models.GeometryLine{
				{Longitude: 5.0, Latitude: 6.0},
				{Longitude: 7.0, Latitude: 8.0},
			},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryMultiLine,
			Content: multiLine,
		})
		require.NoError(t, err)

		var result models.GeometryMultiLine
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Len(t, result[0], 2)
		assert.Len(t, result[1], 2)
	})
}

func TestDecodeGeometryMultiPolygonTag(t *testing.T) {
	t.Run("decode GeometryMultiPolygon", func(t *testing.T) {
		multiPolygon := models.GeometryMultiPolygon{
			models.GeometryPolygon{
				models.GeometryLine{
					{Longitude: 0.0, Latitude: 0.0},
					{Longitude: 0.0, Latitude: 1.0},
					{Longitude: 1.0, Latitude: 1.0},
					{Longitude: 0.0, Latitude: 0.0},
				},
			},
			models.GeometryPolygon{
				models.GeometryLine{
					{Longitude: 2.0, Latitude: 2.0},
					{Longitude: 2.0, Latitude: 3.0},
					{Longitude: 3.0, Latitude: 3.0},
					{Longitude: 2.0, Latitude: 2.0},
				},
			},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryMultiPolygon,
			Content: multiPolygon,
		})
		require.NoError(t, err)

		var result models.GeometryMultiPolygon
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Len(t, result[0], 1)
		assert.Len(t, result[1], 1)
	})
}

func TestDecodeGeometryCollectionTag(t *testing.T) {
	t.Run("decode GeometryCollection", func(t *testing.T) {
		collection := models.GeometryCollection{
			models.GeometryPoint{Longitude: 1.0, Latitude: 2.0},
			models.GeometryLine{
				{Longitude: 3.0, Latitude: 4.0},
				{Longitude: 5.0, Latitude: 6.0},
			},
		}

		data, err := cbor.Marshal(cbor.Tag{
			Number:  models.TagGeometryCollection,
			Content: collection,
		})
		require.NoError(t, err)

		var result models.GeometryCollection
		err = Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})
}
