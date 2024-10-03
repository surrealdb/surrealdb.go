package models

import (
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"strings"
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

type TableOrRecord interface {
	string | Table | RecordID | []Table | []RecordID
}

type Table string

//type UUID string

// type UUIDBin []byte
type UUID struct {
	uuid.UUID
}

type RecordID struct {
	Table string
	ID    interface{}
}

func NewRecordID(idStr string) RecordID {
	bits := strings.Split(idStr, ":")
	if len(bits) != 2 {
		panic(fmt.Errorf("invalid id string. Expected format is 'tablename:indentifier'"))
	}
	return RecordID{
		ID: bits[0], Table: bits[1],
	}
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

type CustomDateTime time.Time

func (d *CustomDateTime) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	totalNS := time.Time(*d).Nanosecond()

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

	*d = CustomDateTime(time.Unix(s, ns))

	return nil
}

type CustomDuration time.Duration

func (d *CustomDuration) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	totalNS := time.Duration(*d).Nanoseconds()
	s := totalNS / 1_000_000_000
	ns := totalNS % 1_000_000_000

	return enc.Marshal(cbor.Tag{
		Number:  uint64(DurationCompactTag),
		Content: [2]int64{s, ns},
	})
}

func (d *CustomDuration) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	s := temp[0].(int64)
	ns := temp[1].(int64)

	*d = CustomDuration(time.Duration((float64(s) * 1_000_000_000) + float64(ns)))

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
