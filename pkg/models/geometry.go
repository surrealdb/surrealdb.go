package models

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

type GeometryPoint struct {
	Latitude  float64
	Longitude float64
}

func NewGeometryPoint(latitude, longitude float64) GeometryPoint {
	return GeometryPoint{
		Latitude: latitude, Longitude: longitude,
	}
}

func (gp *GeometryPoint) GetCoordinates() [2]float64 {
	return [2]float64{gp.Longitude, gp.Latitude}
}

func (gp *GeometryPoint) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagGeometryPoint,
		Content: gp.GetCoordinates(),
	})
}

func (gp *GeometryPoint) UnmarshalCBOR(data []byte) error {
	data, err := getTaggedContent(data, TagGeometryPoint)
	if err != nil {
		return fmt.Errorf("GeometryPoint: %w", err)
	}

	var lonlat [2]float64
	if err := cbor.Unmarshal(data, &lonlat); err != nil {
		return fmt.Errorf("failed to unmarshal GeometryPoint coordinates: %w", err)
	}
	gp.Longitude = lonlat[0]
	gp.Latitude = lonlat[1]

	return nil
}

type GeometryLine []GeometryPoint

type GeometryPolygon []GeometryLine

type GeometryMultiPoint []GeometryPoint

type GeometryMultiLine []GeometryLine

type GeometryMultiPolygon []GeometryPolygon

type GeometryCollection []any
