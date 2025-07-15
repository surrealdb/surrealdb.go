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
	return [2]float64{gp.Latitude, gp.Longitude}
}

func (gp *GeometryPoint) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(cbor.Tag{
		Number:  TagGeometryPoint,
		Content: gp.GetCoordinates(),
	})
}

func (gp *GeometryPoint) UnmarshalCBOR(data []byte) error {
	var tag cbor.RawTag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return err
	}

	if tag.Number != TagGeometryPoint {
		return fmt.Errorf("unexpected tag number: got %d, want %d", tag.Number, TagGeometryPoint)
	}

	data, err := tag.Content.MarshalCBOR()
	if err != nil {
		return fmt.Errorf("failed to extract the raw bytes from cbor tag content of GeometryPoint: %w", err)
	}

	var latlon [2]float64
	if err := cbor.Unmarshal(data, &latlon); err != nil {
		return fmt.Errorf("failed to unmarshal GeometryPoint coordinates: %w", err)
	}
	gp.Latitude = latlon[0]
	gp.Longitude = latlon[1]

	return nil
}

type GeometryLine []GeometryPoint

type GeometryPolygon []GeometryLine

type GeometryMultiPoint []GeometryPoint

type GeometryMultiLine []GeometryLine

type GeometryMultiPolygon []GeometryPolygon

type GeometryCollection []any
