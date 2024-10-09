package models

import (
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/constants"

	"github.com/fxamacker/cbor/v2"
)

type CustomDuration time.Duration

type CustomDurationStr string

func (d *CustomDuration) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	totalNS := time.Duration(*d).Nanoseconds()
	s := totalNS / constants.OneSecondToNanoSecond
	ns := totalNS % constants.OneSecondToNanoSecond

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

	*d = CustomDuration(time.Duration((float64(s) * constants.OneSecondToNanoSecond) + float64(ns)))

	return nil
}
