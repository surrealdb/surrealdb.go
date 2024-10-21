package models

import (
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

type BoundIncluded[T any] struct {
	Value T
}

func (bi *BoundIncluded[T]) MarshalCBOR() ([]byte, error) {
	return getCborEncoder().Marshal(cbor.Tag{
		Number:  TagBoundIncluded,
		Content: bi.Value,
	})
}

func (bi *BoundIncluded[T]) UnmarshalCBOR(data []byte) error {
	var temp T
	err := getCborDecoder().Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	bi.Value = temp
	return nil
}

//------------------------------------------------------------------------------------------------//

type BoundExcluded[T any] struct {
	Value T
}

func (be *BoundExcluded[T]) MarshalCBOR() ([]byte, error) {
	return getCborEncoder().Marshal(cbor.Tag{
		Number:  TagBoundExcluded,
		Content: be.Value,
	})
}

func (be *BoundExcluded[T]) UnmarshalCBOR(data []byte) error {
	var temp T
	err := getCborDecoder().Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	be.Value = temp
	return nil
}

//------------------------------------------------------------------------------------------------//

type Bound[T any] interface {
	BoundIncluded[T] | BoundExcluded[T]
}

type Range[T any, TBeg Bound[T], TEnd Bound[T]] struct {
	Begin *TBeg
	End   *TEnd
}

func (r *Range[T, TBeg, TEnd]) GetJoinString() string {
	joinStr := ""

	if reflect.TypeOf(*r.Begin) == reflect.TypeOf(BoundExcluded[T]{}) {
		joinStr += ">"
	}
	joinStr += ".."
	if reflect.TypeOf(*r.End) == reflect.TypeOf(BoundIncluded[T]{}) {
		joinStr += "="
	}

	return joinStr
}

func (r *Range[T, TBeg, TEnd]) String() string {
	joinStr := r.GetJoinString()
	beginStr := ""
	endStr := ""

	if r.Begin != nil {
		beginStr = convertToString(r.Begin)
	}
	if r.End != nil {
		endStr = convertToString(r.Begin)
	}

	return fmt.Sprintf("%s%s%s", beginStr, joinStr, endStr)
}

func (r *Range[T, TBeg, TEnd]) MarshalCBOR() ([]byte, error) {
	return getCborEncoder().Marshal(cbor.Tag{
		Number:  TagRange,
		Content: []interface{}{r.Begin, r.End},
	})
}

func (r *Range[T, TBeg, TEnd]) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()
	var temp [2]cbor.RawTag
	err := getCborDecoder().Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	var begin TBeg
	beginEnc, _ := temp[0].MarshalCBOR()
	_ = dec.Unmarshal(beginEnc, &begin)

	var end TEnd
	endEnc, _ := temp[1].MarshalCBOR()
	_ = dec.Unmarshal(endEnc, &end)

	r.Begin = &begin
	r.End = &end
	return nil
}

//---------------------------------------------------------------------------------------------------------------------//

type RecordRangeID[T any, TBeg Bound[T], TEnd Bound[T]] struct {
	Range[T, TBeg, TEnd]
	Table Table
}

func (rr *RecordRangeID[T, TBeg, TEnd]) String() string {
	joinStr := rr.GetJoinString()
	beginStr := ""
	endStr := ""

	if rr.Begin != nil {
		beginStr = convertToString(rr.Begin)
	}
	if rr.End != nil {
		endStr = convertToString(rr.Begin)
	}

	return fmt.Sprintf("%s:%s%s%s", rr.Table, beginStr, joinStr, endStr)
}

func convertToString(v any) string {
	// todo: implement
	return ""
}
