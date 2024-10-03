package models

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"io"
	"reflect"
	"time"
)

type CustomCBORTag uint64

var (
	NoneTag                 CustomCBORTag = 6
	TableNameTag            CustomCBORTag = 7
	RecordIDTag             CustomCBORTag = 8
	UUIDStringTag           CustomCBORTag = 9
	DecimalStringTag        CustomCBORTag = 10
	DateTimeCompactString   CustomCBORTag = 12
	DurationStringTag       CustomCBORTag = 13
	DurationCompactTag      CustomCBORTag = 14
	BinaryUUIDTag           CustomCBORTag = 37
	GeometryPointTag        CustomCBORTag = 88
	GeometryLineTag         CustomCBORTag = 89
	GeometryPolygonTag      CustomCBORTag = 90
	GeometryMultiPointTag   CustomCBORTag = 91
	GeometryMultiLineTag    CustomCBORTag = 92
	GeometryMultiPolygonTag CustomCBORTag = 93
	GeometryCollectionTag   CustomCBORTag = 94
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

		TableNameTag: Table(""),
		//UUIDStringTag:    UUID(""),
		DecimalStringTag: Decimal(""),
		BinaryUUIDTag:    UUID{},
		NoneTag:          CustomNil{},

		DateTimeCompactString: CustomDateTime(time.Now()),
		DurationStringTag:     CustomDurationStr("2w"),
		//DurationCompactTag:    CustomDuration(0),
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

type CborMarshaler struct {
}

func (c CborMarshaler) Marshal(v interface{}) ([]byte, error) {
	//v = replacerBeforeEncode(v)
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

	//replacerAfterDecode(&dst)
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
