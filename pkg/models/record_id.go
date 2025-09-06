package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

// RecordID represents a SurrealDB record ID
//
// A record ID consists of a table name and an identifier,
// allowing for a simple and consistent way to reference records across the database.
//
// Record IDs are used to uniquely identify records within a table, to query, update,
// and delete records, and serve as links from one record to another.
//
// Do not conflate RecordID with a plain string representation of a record ID,
// which is typically formatted as "<table>:<identifier>" (e.g., "user:12345").
//
// ":" is not a reserved character in SurrealQL, and it's possible to have table names or IDs containing ":",
// in which case it's string representation can look like:
//
//	`foo:`:[1,2,{a:3}]
//
// The use of RecordID struct helps to avoid ambiguity and ensures that
// the table and identifier components are always clearly defined and separated.
//
// See https://surrealdb.com/docs/surrealql/datamodel/ids for details.
type RecordID struct {
	// Table is the name of the table this record belongs to.
	// It must be a non-empty string.
	Table string
	// ID can be of type whose value can be marshaled as a CBOR value that SurrealDB accepts.
	ID any
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

	if len(temp) != 2 {
		return fmt.Errorf("invalid RecordID format: expected array of 2 elements, got %d", len(temp))
	}

	tableStr, ok := temp[0].(string)
	if !ok {
		return fmt.Errorf("invalid RecordID format: table must be a string")
	}
	r.Table = tableStr
	r.ID = temp[1]

	return nil
}

func (r *RecordID) SurrealString() string {
	return fmt.Sprintf("r'%s'", r.String())
}
