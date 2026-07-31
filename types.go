package surrealdb

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Deprecated: Use [ServerError] instead on SurrealDB v3 for richer error information.
// TODO(v2-compat): Remove in next major release.
//
//nolint:staticcheck // v2 backward compat with RPCError
type RPCError = connection.RPCError

// ServerError represents a structured error from SurrealDB v3.
// Only use this when you know you are running against a SurrealDB v3 server.
//
// Extract from RPC errors using errors.As:
//
//	var se *surrealdb.ServerError
//	if errors.As(err, &se) {
//	    fmt.Println(se.Kind, se.Details)
//	}
//
// For ergonomic kind checking without extracting a *ServerError yourself,
// use the Is* helpers below (e.g. [IsNotFound], [IsNotAllowed]) or compare
// se.Kind directly against the Kind* constants (e.g. [KindNotFound]).
type ServerError = connection.ServerError

// Known values for ServerError.Kind. See [connection.KindNotFound] and its
// siblings for the full list and provenance.
const (
	KindValidation    = connection.KindValidation
	KindConfiguration = connection.KindConfiguration
	KindThrown        = connection.KindThrown
	KindQuery         = connection.KindQuery
	KindSerialization = connection.KindSerialization
	KindNotAllowed    = connection.KindNotAllowed
	KindNotFound      = connection.KindNotFound
	KindAlreadyExists = connection.KindAlreadyExists
	KindConnection    = connection.KindConnection
	KindInternal      = connection.KindInternal
)

// IsNotFound reports whether err is (or wraps) a *ServerError whose Kind is
// "NotFound". See [connection.IsNotFound].
var IsNotFound = connection.IsNotFound

// IsNotAllowed reports whether err is (or wraps) a *ServerError whose Kind is
// "NotAllowed". See [connection.IsNotAllowed].
var IsNotAllowed = connection.IsNotAllowed

// IsTransactionConflict reports whether err is a *ServerError representing a
// Query/TransactionConflict failure, safe to retry. See
// [connection.IsTransactionConflict].
var IsTransactionConflict = connection.IsTransactionConflict

// IsTimedOut reports whether err is a *ServerError representing a
// Query/TimedOut failure. See [connection.IsTimedOut].
var IsTimedOut = connection.IsTimedOut

// IsNotExecuted reports whether err is a *ServerError representing a
// Query/NotExecuted failure. See [connection.IsNotExecuted].
var IsNotExecuted = connection.IsNotExecuted

// IsCancelled reports whether err is a *ServerError representing a canceled
// query. See [connection.IsCancelled].
var IsCancelled = connection.IsCancelled

// IsParseError reports whether err is a *ServerError representing a
// Validation/Parse failure. See [connection.IsParseError].
var IsParseError = connection.IsParseError

// IsDeserialization reports whether err is a *ServerError representing a
// Serialization/Deserialization failure. See [connection.IsDeserialization].
var IsDeserialization = connection.IsDeserialization

// IsLiveQueryNotSupported reports whether err is a *ServerError representing
// a Configuration/LiveQueryNotSupported failure. See
// [connection.IsLiveQueryNotSupported].
var IsLiveQueryNotSupported = connection.IsLiveQueryNotSupported

// IsScriptingBlocked reports whether err is a *ServerError representing a
// NotAllowed/Scripting failure. See [connection.IsScriptingBlocked].
var IsScriptingBlocked = connection.IsScriptingBlocked

// IsTokenExpired reports whether err is a *ServerError representing a
// NotAllowed/Auth/TokenExpired failure. See [connection.IsTokenExpired].
var IsTokenExpired = connection.IsTokenExpired

// IsInvalidAuth reports whether err is a *ServerError representing a
// NotAllowed/Auth/InvalidAuth failure. See [connection.IsInvalidAuth].
var IsInvalidAuth = connection.IsInvalidAuth

// Patch represents a patch object set to MODIFY a record
type PatchData struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// QueryResult is a struct that represents one of the results
// of a SurrealDB query RPC method call, made via [Query], for example.
type QueryResult[T any] struct {
	Status string      `json:"status"`
	Time   string      `json:"time"`
	Result T           `json:"result"`
	Error  *QueryError `json:"-"`
}

// QueryError represents an error that occurred during a query execution.
//
// The caller can type-assert the return errror to QueryError to see if
// the error is a query error or not.
type QueryError struct {
	Message string
}

func (e *QueryError) Error() string {
	if e == nil {
		return ""
	}

	return e.Message
}

func (e *QueryError) Is(target error) bool {
	if target == nil {
		return e == nil
	}

	_, ok := target.(*QueryError)
	return ok
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
	Password  string `json:"pass,omitempty"` //nolint:gosec // G117: user-supplied auth credential
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
