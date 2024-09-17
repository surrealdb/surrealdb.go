package models

import (
	"github.com/fxamacker/cbor/v2"
	"time"
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
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(GeometryPointTag),
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

type Table string

type UUID string

type UUIDBin []byte

type RecordID struct {
	Table string
	ID    interface{}
}

func NewRecordID(table string, id interface{}) *RecordID {
	return &RecordID{Table: table, ID: id}
}

func (r *RecordID) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(RecordIDTag),
		Content: []interface{}{r.ID, r.Table},
	})
}

func (r *RecordID) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp []interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	r.Table = temp[0].(string)
	r.ID = temp[1]

	return nil
}

type Decimal string

type CustomDateTime struct {
	time.Time
}

func (d *CustomDateTime) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	totalNS := d.Nanosecond()

	s := totalNS / 1_000_000_000
	ns := totalNS % 1_000_000_000

	return enc.Marshal(cbor.Tag{
		Number:  uint64(DateTimeCompactString),
		Content: [2]int64{int64(s), int64(ns)},
	})
}

func (d *CustomDateTime) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	s := temp[0].(int64)
	ns := temp[1].(int64)

	d = &CustomDateTime{time.Unix(s, ns)}

	return nil
}

type CustomDuration struct {
	time.Duration
}

func (d *CustomDuration) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(DurationCompactTag),
		Content: [2]int64{1213, 123},
	})
}

func (d *CustomDuration) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	return nil
}

type CustomDurationStr string

type CustomNil struct {
}

func (c *CustomNil) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(NoneTag),
		Content: nil,
	})
}

func (c *CustomNil) CustomNil(data []byte) error {
	c = &CustomNil{}
	return nil
}

var None = CustomNil{}
