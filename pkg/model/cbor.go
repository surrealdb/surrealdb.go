package model

import (
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"io"
	"reflect"
)

type CustomCBORTag uint64

var (
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

		TableNameTag:     Table(""),
		UUIDStringTag:    UUID(""),
		DecimalStringTag: Decimal(""),
		BinaryUUIDTag:    UUIDBin{},

		DateTimeCompactString: CustomDateTime{},
		//DurationStringTag:        Duration("0"),
		//DurationCompactTag: time.Duration(0), // Duration(""),
		//DurationCompactTag: Duration(0), // Duration(""),
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
	em := getCborEncoder()
	return em.Marshal(v)
}

func (c CborMarshaler) NewEncoder(w io.Writer) codec.Encoder {
	em := getCborEncoder()
	return em.NewEncoder(w)
}

type CborUnmashaler struct {
}

func (c CborUnmashaler) Unmarshal(data []byte, dst interface{}) error {
	dm := getCborDecoder()
	return dm.Unmarshal(data, dst)
}

func (c CborUnmashaler) NewDecoder(r io.Reader) codec.Decoder {
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

func (d *CustomDateTime) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  uint64(DateTimeCompactString),
		Content: [2]int64{1213, 123},
	})
}

func (d *CustomDateTime) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	fmt.Println(temp)
	return nil
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

	fmt.Println(temp)
	return nil
}
