package surrealdb

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Patch represents a patch object set to MODIFY a record
type PatchData struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type QueryResult[T any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result T      `json:"result"`
}

type QueryStmt struct {
	unmarshaler codec.Unmarshaler
	SQL         string
	Vars        map[string]interface{}
	Result      QueryResult[cbor.RawMessage]
}

func (q *QueryStmt) GetResult(dest interface{}) error {
	if q.unmarshaler == nil {
		return constants.ErrNoUnmarshaler
	}
	return q.unmarshaler.Unmarshal(q.Result.Result, dest)
}

type Relationship struct {
	ID       *models.RecordID `json:"id"`
	In       models.RecordID  `json:"in"`
	Out      models.RecordID  `json:"out"`
	Relation models.Table     `json:"relation"`
	Data     map[string]any   `json:"data"`
}

// Auth is a struct that holds surrealdb auth data for login.
type Auth struct {
	Namespace string `json:"NS,omitempty"`
	Database  string `json:"DB,omitempty"`
	Scope     string `json:"SC,omitempty"`
	Username  string `json:"user,omitempty"`
	Password  string `json:"pass,omitempty"`
}

type Obj map[interface{}]interface{}

type Result[T any] struct {
	T any
}

type TableOrRecord interface {
	string | models.Table | models.RecordID | []models.Table | []models.RecordID
}
