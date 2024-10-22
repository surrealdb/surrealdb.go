package models

import "github.com/fxamacker/cbor/v2"

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
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  TagGeometryPoint,
		Content: gp.GetCoordinates(),
	})
}

func (gp *GeometryPoint) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]float64
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	gp.Latitude = temp[0]
	gp.Longitude = temp[1]

	return nil
}

type GeometryLine []GeometryPoint

type GeometryPolygon []GeometryLine

type GeometryMultiPoint []GeometryPoint

type GeometryMultiLine []GeometryLine

type GeometryMultiPolygon []GeometryPolygon

type GeometryCollection []any
