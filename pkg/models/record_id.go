package models

import (
	"errors"
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

var ErrBadRecordID = errors.New("invalid record ID (want <table>:<identifier>)")

func ParseRecordID(idStr string) (*RecordID, error) {
	expectedLen := 2
	bits := strings.Split(idStr, ":")
	if len(bits) != expectedLen {
		return nil, fmt.Errorf("%w: %q", ErrBadRecordID, idStr)
	}
	return &RecordID{
		Table: bits[0], ID: bits[1],
	}, nil
}

func NewRecordID(tableName string, id any) RecordID {
	return RecordID{Table: tableName, ID: id}
}

func (r *RecordID) MarshalCBOR() ([]byte, error) {
	// We must prevent returning an invalid RecordID,
	// because it results in SurrealDB returning an error without the response ID
	// if the RPC is made over WebSocket, and
	// we cannot distinguish it from a notification,
	// nor can we return an error to the caller.
	// See https://github.com/surrealdb/surrealdb.go/issues/273
	if r.Table == "" || r.ID == nil {
		return nil, fmt.Errorf("cannot marshal RecordID with empty table or ID: want <table>:<identifier> but got %s:%v", r.Table, r.ID)
	}
	return cbor.Marshal(cbor.Tag{
		Number:  TagRecordID,
		Content: []any{r.Table, r.ID},
	})
}

func (r *RecordID) UnmarshalCBOR(data []byte) error {
	var tag cbor.Tag
	if err := cbor.Unmarshal(data, &tag); err != nil {
		return err
	}

	if tag.Number != TagRecordID {
		return fmt.Errorf("unexpected tag number: got %d, want %d", tag.Number, TagRecordID)
	}

	var temp []any
	err := cbor.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	r.Table = temp[0].(string)
	r.ID = temp[1]

	return nil
}

func (r *RecordID) SurrealString() string {
	return fmt.Sprintf("r'%s'", r.String())
}
