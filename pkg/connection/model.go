package connection

import (
	"errors"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// wireDecMode is a CBOR decode mode that decodes maps with string keys
// to map[string]any instead of map[any]any. This ensures the Details field
// in wireError is always map[string]any for consistent detail helper behavior.
var wireDecMode cbor.DecMode

//nolint:gochecknoinits // init is used to set up the CBOR decode mode for wireError
func init() {
	var err error
	wireDecMode, err = cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any(nil)),
	}.DecMode()
	if err != nil {
		panic(err)
	}
}

// RPCError represents a JSON-RPC error from the SurrealDB server.
//
// On SurrealDB v3 servers, use errors.As to extract a *ServerError for richer
// structured error information (Kind, Details, Cause).
//
// Deprecated: Use [ServerError] instead on SurrealDB v3 for richer error information.
// TODO(v2-compat): Remove in next major release.
type RPCError struct {
	// Code is the JSON-RPC numeric error code.
	// SurrealDB v2 and v3: Always present for RPC-level errors.
	Code int `json:"code"`

	// Message is the error message from the server.
	// SurrealDB v2 and v3: Always present.
	Message string `json:"message,omitempty"`

	// Description is a human-readable description of the error.
	// SurrealDB v2 only: Not populated by v3 servers.
	// Use ServerError on SurrealDB v3 instead.
	//
	// Deprecated: Not populated by SurrealDB v3 servers.
	// TODO(v2-compat): Remove in next major release.
	Description string `json:"description,omitempty"`

	// wire stores the full deserialized error data (v2 and v3 fields).
	// RPCError.As delegates to wireError.As for errors.As extraction of *ServerError.
	wire *wireError
}

// UnmarshalCBOR deserializes the RPC error from CBOR.
// It first deserializes into wireError (capturing all v2+v3 fields),
// then populates the v2 public fields and creates a ServerError for the v3 view.
func (r *RPCError) UnmarshalCBOR(data []byte) error {
	w := &wireError{}
	if err := wireDecMode.Unmarshal(data, w); err != nil {
		return err
	}
	r.Code = w.Code
	r.Message = w.Message
	r.Description = w.Description
	r.wire = w
	return nil
}

func (r RPCError) Error() string {
	if r.Description != "" {
		return r.Description
	}
	return r.Message
}

func (r *RPCError) Is(target error) bool {
	switch target.(type) {
	case RPCError, *RPCError, ServerError, *ServerError:
		return true
	default:
		return false
	}
}

func (r *RPCError) As(err any) bool {
	return errors.As(r.wire, err)
}

// RPCRequest represents an incoming JSON-RPC request.
// For SurrealDB v3+, Session and Txn can be set to scope the request to a specific session or transaction.
type RPCRequest struct {
	ID      any    `json:"id"`
	Method  string `json:"method,omitempty"`
	Params  []any  `json:"params,omitempty"`
	Session any    `json:"session,omitempty"` // SurrealDB v3: session UUID for session-scoped operations
	Txn     any    `json:"txn,omitempty"`     // SurrealDB v3: transaction UUID for transaction-scoped operations
}

// RPCResponse represents an outgoing JSON-RPC response
type RPCResponse[T any] struct {
	// ID is the ID of the request this response corresponds to.
	// Note that this is always nil in case of HTTPConnection.
	ID     any       `json:"id"`
	Error  *RPCError `json:"error,omitempty"`
	Result *T        `json:"result,omitempty"`
}

// RPCNotification represents an outgoing JSON-RPC notification
type RPCNotification struct {
	ID     any    `json:"id"`
	Method string `json:"method,omitempty"`
	Params []any  `json:"params,omitempty"`
}

type RPCFunction string

type ResponseID[T any] struct {
	ID *T `json:"id"`
}

var (
	Use          RPCFunction = "use"
	Info         RPCFunction = "info"
	SignUp       RPCFunction = "signup"
	SignIn       RPCFunction = "signin"
	Authenticate RPCFunction = "authenticate"
	Invalidate   RPCFunction = "invalidate"
	Let          RPCFunction = "let"
	Unset        RPCFunction = "unset"
	Live         RPCFunction = "live"
	Kill         RPCFunction = "kill"
	Query        RPCFunction = "query"
	Select       RPCFunction = "select"
	Create       RPCFunction = "create"
	Insert       RPCFunction = "insert"
	Update       RPCFunction = "update"
	Upsert       RPCFunction = "upsert"
	Relate       RPCFunction = "relate"
	Merge        RPCFunction = "merge"
	Patch        RPCFunction = "patch"
	Delete       RPCFunction = "delete"

	// SurrealDB v3+ session and transaction methods
	Attach RPCFunction = "attach"
	Detach RPCFunction = "detach"
	Begin  RPCFunction = "begin"
	Commit RPCFunction = "commit"
	Cancel RPCFunction = "cancel"
)
