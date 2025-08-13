package models

import (
	"reflect"
	"sync"

	"github.com/fxamacker/cbor/v2"
)

const (
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
	customTags := map[uint64]any{
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
	once sync.Once
	em   cbor.EncMode
}

func (c *CborMarshaler) Marshal(v any) ([]byte, error) {
	v = replacerBeforeEncode(v)
	return c.cborEncMode().Marshal(v)
}

func (c *CborMarshaler) cborEncMode() cbor.EncMode {
	c.once.Do(func() {
		c.em = getCborEncoder()
	})

	return c.em
}

type CborUnmarshaler struct {
	once sync.Once
	dm   cbor.DecMode
}

func (c *CborUnmarshaler) Unmarshal(data []byte, dst any) error {
	err := c.cborDecMode().Unmarshal(data, dst)
	if err != nil {
		return err
	}

	replacerAfterDecode(&dst)
	return nil
}

func (c *CborUnmarshaler) cborDecMode() cbor.DecMode {
	c.once.Do(func() {
		c.dm = getCborDecoder()
	})

	return c.dm
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
		TimeTagToAny:   cbor.TimeTagToTime,
		DefaultMapType: reflect.TypeOf(map[string]any(nil)),
	}.DecModeWithTags(tags)
	if err != nil {
		panic(err)
	}

	return dm
}
