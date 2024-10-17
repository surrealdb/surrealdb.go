package models

import (
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

type BoundIncluded[T any] struct {
	value T
}

func (bi BoundIncluded[T]) Value() T {
	return bi.value
}

type BoundExcluded[T any] struct {
	value T
}

func (be BoundExcluded[T]) Value() T {
	return be.value
}

type Bound[T any] interface {
	BoundIncluded[T] | BoundExcluded[T]
	Value() T
}

type Range[T any, TBeg Bound[T], TEnd Bound[T]] struct {
	begin *TBeg
	end   *TEnd
}

func (r *Range[T, TBeg, TEnd]) GetJoinString() string {
	joinStr := ""

	if reflect.TypeOf(*r.begin) == reflect.TypeOf(BoundExcluded[T]{}) {
		joinStr += ">"
	}
	joinStr += ".."
	if reflect.TypeOf(*r.end) == reflect.TypeOf(BoundIncluded[T]{}) {
		joinStr += "="
	}

	return joinStr
}

func (r *Range[T, TBeg, TEnd]) String() string {
	joinStr := r.GetJoinString()
	beginStr := ""
	endStr := ""

	// if r.begin != nil {
	//	//beginStr = fmt.Sprintf("%s", string((*r.begin).Value()))
	//}
	//if r.end != nil {
	//	//endStr = fmt.Sprintf("%s", (*r.end).Value())
	//}

	return fmt.Sprintf("%s%s%s", beginStr, joinStr, endStr)
}

func (r *Range[T, TBeg, TEnd]) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()
	return enc.Marshal(cbor.Tag{
		Number:  TagRange,
		Content: []interface{}{r.begin, r.end},
	})
}

func (r *Range[T, TBeg, TEnd]) UnmarshalCBOR(data []byte) error {
	dec := getCborDecoder()

	var temp [2]interface{}
	err := dec.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	// r.begin = temp[0]
	// r.end = temp[1]
	return nil
}
