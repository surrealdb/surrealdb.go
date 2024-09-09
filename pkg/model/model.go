package model

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
