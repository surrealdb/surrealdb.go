package surrealcbor

import (
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// getEncMode returns encoder mode using fxamacker/cbor
func getEncMode() cbor.EncMode {
	tags := cbor.NewTagSet()

	// Register SurrealDB custom types
	customTypes := []struct {
		tag uint64
		typ any
	}{
		{models.TagNone, models.CustomNil{}},
		{models.TagTable, models.Table("")},
		{models.TagRecordID, models.RecordID{}},
		{models.TagCustomDatetime, models.CustomDateTime{}},
		{models.TagCustomDuration, models.CustomDuration{}},
		{models.TagFuture, models.Future{}},
		{models.TagStringUUID, models.UUIDString("")},
		{models.TagStringDecimal, models.DecimalString("")},
		{models.TagStringDuration, models.CustomDurationString("")},
		{models.TagSpecBinaryUUID, models.UUID{}},
		{models.TagGeometryPoint, models.GeometryPoint{}},
		{models.TagGeometryLine, models.GeometryLine{}},
		{models.TagGeometryPolygon, models.GeometryPolygon{}},
		{models.TagGeometryMultiPoint, models.GeometryMultiPoint{}},
		{models.TagGeometryMultiLine, models.GeometryMultiLine{}},
		{models.TagGeometryMultiPolygon, models.GeometryMultiPolygon{}},
		{models.TagGeometryCollection, models.GeometryCollection{}},
	}

	for _, ct := range customTypes {
		err := tags.Add(
			cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
			reflect.TypeOf(ct.typ),
			ct.tag,
		)
		if err != nil {
			panic(err)
		}
	}

	em, err := cbor.EncOptions{
		Time:    cbor.TimeRFC3339,
		TimeTag: cbor.EncTagRequired,
	}.EncModeWithTags(tags)
	if err != nil {
		panic(err)
	}

	return em
}

// Encoder writes CBOR values to an output stream
type Encoder struct {
	w  io.Writer
	em cbor.EncMode
}

// Encode writes the CBOR encoding of v to the stream
func (enc *Encoder) Encode(v any) error {
	data, err := enc.em.Marshal(v)
	if err != nil {
		return err
	}
	_, err = enc.w.Write(data)
	return err
}
