package models

import (
	"fmt"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

type RecordID struct {
	Table string
	ID    any
}

type RecordIDType interface {
	~int | ~string | []any | map[string]any
}

func ParseRecordID(idStr string) *RecordID {
	expectedLen := 2
	bits := strings.Split(idStr, ":")
	if len(bits) != expectedLen {
		panic(fmt.Errorf("invalid id string. Expected format is 'tablename:indentifier'"))
	}
	return &RecordID{
		Table: bits[0], ID: bits[1],
	}
}

func NewRecordID(tableName string, id any) RecordID {
	return RecordID{Table: tableName, ID: id}
}

func (r *RecordID) MarshalCBOR() ([]byte, error) {
	enc := getCborEncoder()

	return enc.Marshal(cbor.Tag{
		Number:  TagRecordID,
		Content: []interface{}{r.Table, r.ID},
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
	r.ID = temp[1].(string)

	return nil
}

func (r *RecordID) String() string {
	return fmt.Sprintf("%s:%s", r.Table, r.ID)
}

func (r *RecordID) SurrealString() string {
	return fmt.Sprintf("r'%s'", r.String())
}
