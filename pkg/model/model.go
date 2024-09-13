package model

import (
	"github.com/fxamacker/cbor/v2"
	"time"
)

type GeometryPoint struct {
	Latitude  float64
	Longitude float64
}

func NewGeometryPoint(latitude float64, longitude float64) GeometryPoint {
	return GeometryPoint{
		Latitude: latitude, Longitude: longitude,
	}
}

func (g *GeometryPoint) GetCoordinates() [2]float64 {
	return [2]float64{g.Latitude, g.Longitude}
}

func (gp *GeometryPoint) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(GeometryPointTag),
		Content: gp.GetCoordinates(),
	})
}

func (g *GeometryPoint) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]float64
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	g.Latitude = temp[0]
	g.Longitude = temp[1]

	return nil
}

type GeometryLine []GeometryPoint

type GeometryPolygon []GeometryLine

type GeometryMultiPoint []GeometryPoint

type GeometryMultiLine []GeometryLine

type GeometryMultiPolygon []GeometryPolygon

type GeometryCollection []any

type Table string

type UUID string

type UUIDBin []byte

type RecordID struct {
	Table string
	ID    interface{}
}

type Decimal string

type CustomDateTime time.Time

type CustomDuration time.Duration

// Auth is a struct that holds surrealdb auth data for login.
type Auth struct {
	Namespace string `json:"NS,omitempty"`
	Database  string `json:"DB,omitempty"`
	Scope     string `json:"SC,omitempty"`
	Username  string `json:"user,omitempty"`
	Password  string `json:"pass,omitempty"`
}
