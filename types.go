package surrealdb

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Deprecated: Use [ServerError] instead. RPCError is kept as a type alias
// for backward compatibility. Server errors are now returned as *ServerError
// with structured kind, details, and cause information.
type RPCError = connection.RPCError

// Deprecated: Use [ServerError] instead. QueryError is kept as a type alias
// for backward compatibility. Query errors are now returned as *ServerError
// with Kind set to the appropriate ErrorKind.
type QueryError = ServerError

// Patch represents a patch object set to MODIFY a record
type PatchData struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// QueryResult is a struct that represents one of the results
// of a SurrealDB query RPC method call, made via [Query], for example.
type QueryResult[T any] struct {
	Status string       `json:"status"`
	Time   string       `json:"time"`
	Result T            `json:"result"`
	Error  *ServerError `json:"-"`
}

// rawQueryResult is used internally to decode query results from CBOR
// before converting to the public QueryResult type. It captures the new
// structured error fields (kind, details, cause) alongside the standard
// status/time/result fields.
type rawQueryResult struct {
	Status  string               `json:"status"`
	Time    string               `json:"time"`
	Result  cbor.RawMessage      `json:"result"`
	Kind    string               `json:"kind,omitempty"`
	Details any                  `json:"details,omitempty"`
	Cause   *connection.RPCError `json:"cause,omitempty"`
}

type QueryStmt struct {
	unmarshaler codec.Unmarshaler
	SQL         string
	Vars        map[string]any
	Result      QueryResult[cbor.RawMessage]
}

func (q *QueryStmt) GetResult(dest any) error {
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
	Access    string `json:"AC,omitempty"`
	Username  string `json:"user,omitempty"`
	Password  string `json:"pass,omitempty"`
}

// Deprecated: Use map[string]any instead
type Obj map[any]any

// Deprecated: Use [RPCResponse] instead.
type Result[T any] struct {
	T any
}

type TableOrRecord interface {
	string | models.Table | models.RecordID | []models.Table | []models.RecordID
}
