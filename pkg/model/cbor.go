package model

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/connection"
	"reflect"
)

type CustomCBORTag uint64

var (
	DateTimeStringTag        CustomCBORTag = 0
	NoneTag                  CustomCBORTag = 6
	TableNameTag             CustomCBORTag = 7
	RecordIDTag              CustomCBORTag = 8
	UUIDStringTag            CustomCBORTag = 9
	DecimalStringTag         CustomCBORTag = 10
	DateTimeCompactString    CustomCBORTag = 12
	DurationStringTag        CustomCBORTag = 13
	DurationCompactStringTag CustomCBORTag = 14
	BinaryUUIDTag            CustomCBORTag = 37
	GeometryPointTag         CustomCBORTag = 88
	GeometryLineTag          CustomCBORTag = 89
	GeometryPolygonTag       CustomCBORTag = 90
	GeometryMultiPointTag    CustomCBORTag = 91
	GeometryMultiLineTag     CustomCBORTag = 92
	GeometryMultiPolygonTag  CustomCBORTag = 93
	GeometryCollectionTag    CustomCBORTag = 94
)

func registerCborTags() cbor.TagSet {
	customTags := map[CustomCBORTag]interface{}{
		GeometryPointTag:        GeometryPoint{},
		GeometryLineTag:         GeometryLine{},
		GeometryPolygonTag:      GeometryPolygon{},
		GeometryMultiPointTag:   GeometryMultiPoint{},
		GeometryMultiLineTag:    GeometryMultiLine{},
		GeometryMultiPolygonTag: GeometryMultiPolygon{},
		GeometryCollectionTag:   GeometryCollection{},

		TableNameTag:  Table(""),
		UUIDStringTag: UUID(""),
		BinaryUUIDTag: UUIDBin{},
	}

	tags := cbor.NewTagSet()
	for tag, customType := range customTags {
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(customType),
			uint64(tag),
		)
		if err != nil {
			panic(err)
		}
	}

	return tags
}

func GetCborEncoder() connection.Encoder {
	tags := registerCborTags()
	em, err := cbor.EncOptions{}.EncModeWithTags(tags)
	if err != nil {
		panic(err)
	}

	return em.Marshal
}

func GetCborDecoder() connection.Decoder {
	tags := registerCborTags()
	dm, err := cbor.DecOptions{}.DecModeWithTags(tags)
	if err != nil {
		panic(err)
	}

	return dm.Unmarshal
}

func (gp *GeometryPoint) MarshalCBOR() ([]byte, error) {
	enc := GetCborEncoder()

	return enc(cbor.Tag{
		Number:  uint64(GeometryPointTag),
		Content: gp.GetCoordinates(),
	})
}

func (g *GeometryPoint) UnmarshalCBOR(data []byte) error {
	dec := GetCborDecoder()

	var temp [2]float64
	err := dec(data, &temp)
	if err != nil {
		return err
	}

	g.Latitude = temp[0]
	g.Longitude = temp[1]

	return nil
}

func (r *RecordID) MarshalCBOR() ([]byte, error) {
	enc := GetCborEncoder()

	return enc(cbor.Tag{
		Number:  uint64(RecordIDTag),
		Content: []interface{}{r.ID, r.Table},
	})
}

func (r *RecordID) UnmarshalCBOR(data []byte) error {
	dec := GetCborDecoder()

	var temp []interface{}
	err := dec(data, &temp)
	if err != nil {
		return err
	}

	r.Table = temp[0].(string)
	r.ID = temp[1]

	return nil
}
