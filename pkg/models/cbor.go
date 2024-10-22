package models

import (
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
)

var (
	TagNone           uint64 = 6
	TagTable          uint64 = 7
	TagRecordID       uint64 = 8
	TagCustomDatetime uint64 = 12
	TagCustomDuration uint64 = 14
	TagFuture         uint64 = 15

	TagStringUUID     uint64 = 9
	TagStringDecimal  uint64 = 10
	TagStringDuration uint64 = 13

	TagSpecBinaryUUID uint64 = 37

	TagRange         uint64 = 49
	TagBoundIncluded uint64 = 50
	TagBoundExcluded uint64 = 51

	TagGeometryPoint        uint64 = 88
	TagGeometryLine         uint64 = 89
	TagGeometryPolygon      uint64 = 90
	TagGeometryMultiPoint   uint64 = 91
	TagGeometryMultiLine    uint64 = 92
	TagGeometryMultiPolygon uint64 = 93
	TagGeometryCollection   uint64 = 94
)

func registerCborTags() cbor.TagSet {
	customTags := map[uint64]interface{}{
		TagNone:     CustomNil{},
		TagTable:    Table(""),
		TagRecordID: RecordID{},

		TagCustomDatetime: CustomDateTime{},
		TagCustomDuration: CustomDuration{},
		TagFuture:         Future{},

		TagStringUUID:     UUIDString(""),
		TagStringDecimal:  DecimalString(""),
		TagStringDuration: CustomDurationString(""),

		TagSpecBinaryUUID: UUID{},

		TagGeometryPoint:        GeometryPoint{},
		TagGeometryLine:         GeometryLine{},
		TagGeometryPolygon:      GeometryPolygon{},
		TagGeometryMultiPoint:   GeometryMultiPoint{},
		TagGeometryMultiLine:    GeometryMultiLine{},
		TagGeometryMultiPolygon: GeometryMultiPolygon{},
		TagGeometryCollection:   GeometryCollection{},
	}
	tags := cbor.NewTagSet()
	for tag, customType := range customTags {
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(customType),
			tag,
		)
		if err != nil {
			panic(err)
		}
	}

	return tags
}

type CborMarshaler struct {
}

func (c CborMarshaler) Marshal(v interface{}) ([]byte, error) {
	v = replacerBeforeEncode(v)
	em := getCborEncoder()
	return em.Marshal(v)
}

func (c CborMarshaler) NewEncoder(w io.Writer) codec.Encoder {
	em := getCborEncoder()
	return em.NewEncoder(w)
}

type CborUnmarshaler struct {
}

func (c CborUnmarshaler) Unmarshal(data []byte, dst interface{}) error {
	dm := getCborDecoder()
	err := dm.Unmarshal(data, dst)
	if err != nil {
		return err
	}

	replacerAfterDecode(&dst)
	return nil
}

func (c CborUnmarshaler) NewDecoder(r io.Reader) codec.Decoder {
	dm := getCborDecoder()
	return dm.NewDecoder(r)
}

func getCborEncoder() cbor.EncMode {
	tags := registerCborTags()
	em, err := cbor.EncOptions{
		Time:    cbor.TimeRFC3339,
		TimeTag: cbor.EncTagRequired,
	}.EncModeWithTags(tags)
	if err != nil {
		panic(err)
	}

	return em
}

func getCborDecoder() cbor.DecMode {
	tags := registerCborTags()
	dm, err := cbor.DecOptions{
		TimeTagToAny: cbor.TimeTagToTime,
	}.DecModeWithTags(tags)
	if err != nil {
		panic(err)
	}

	return dm
}
