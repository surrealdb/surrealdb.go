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
	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return err
	}

	if tag.Number != TagGeometryPoint {
		return fmt.Errorf("unexpected tag number: got %d, want %d", tag.Number, TagGeometryPoint)
	}

	content, ok := tag.Content.([]any)
	if !ok {
		return fmt.Errorf("unexpected content type: got %T, want [2]float64", tag.Content)
	}

	lat, ok := content[0].(float64)
	if !ok {
		return fmt.Errorf("unexpected type for latitude: got %T, want float64", content[0])
	}

	lon, ok := content[1].(float64)
	if !ok {
		return fmt.Errorf("unexpected type for longitude: got %T, want float64", content[1])
	}

	gp.Latitude = lat
	gp.Longitude = lon

	return nil
}

type GeometryLine []GeometryPoint

type GeometryPolygon []GeometryLine

type GeometryMultiPoint []GeometryPoint

type GeometryMultiLine []GeometryLine

type GeometryMultiPolygon []GeometryPolygon

type GeometryCollection []any
