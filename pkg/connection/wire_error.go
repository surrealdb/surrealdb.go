package connection

import "errors"

// wireError is an unexported deserialization target that captures ALL v2+v3 wire fields.
// It is used as the intermediate representation during RPCError.UnmarshalCBOR,
// before populating the v2 public fields on RPCError and creating a ServerError for the v3 view.
type wireError struct {
	Code        int        `json:"code"`
	Message     string     `json:"message,omitempty"`
	Description string     `json:"description,omitempty"` // SurrealDB v2 only
	Kind        string     `json:"kind,omitempty"`        // SurrealDB v3 only
	Details     any        `json:"details,omitempty"`     // SurrealDB v3 only
	Cause       *wireError `json:"cause,omitempty"`       // SurrealDB v3 only
}

// hasContent reports whether w carries any meaningful error data.
// A zero-value wireError (e.g. from CBOR deserializing an empty cause object)
// is treated as absent so it doesn't produce a spurious ServerError.Cause.
func (w *wireError) hasContent() bool {
	return w.Code != 0 || w.Message != "" || w.Kind != "" || w.Details != nil
}

func (w *wireError) Error() string {
	if w == nil {
		return "<nil>"
	}
	return w.Message
}

func (r *wireError) As(err any) bool {
	if r == nil {
		return false
	}
	switch e := err.(type) {
	case *ServerError:
		e.Message = r.Message
		e.Kind = r.Kind
		e.Code = r.Code
		e.Details = r.Details

		var cause ServerError
		if r.Cause != nil && r.Cause.hasContent() && errors.As(r.Cause, &cause) {
			e.Cause = &cause
		}

		return true
	case **ServerError:
		se := &ServerError{
			Message: r.Message,
			Kind:    r.Kind,
			Code:    r.Code,
			Details: r.Details,
		}

		var cause ServerError
		if r.Cause != nil && r.Cause.hasContent() && errors.As(r.Cause, &cause) {
			se.Cause = &cause
		}

		*e = se
		return true
	case *RPCError:
		e.Code = r.Code
		e.Message = r.Message
		e.Description = r.Description
		return true
	case **RPCError:
		rpcErr := &RPCError{
			Code:        r.Code,
			Message:     r.Message,
			Description: r.Description,
		}
		*e = rpcErr
		return true
	}
	return false
}

// NewStatementError returns the error of a failed statement in the result of
// the query RPC method, given the message, kind and details SurrealDB reports
// for it (SurrealDB v2 reports only the message). It is the *RPCError an RPC
// method returns, so it can also be extracted as a *ServerError.
//
// A statement's error carries no code, so its code is the one SurrealDB
// assigns an RPC error of that kind and details: -32009 for a transaction
// conflict, -32006 for a THROW, and so on. An error whose kind has no code of
// its own, or that has no kind (SurrealDB v2), gets -32000.
//
// The CRUD functions of the surrealdb package, which compile to queries,
// return it so that their errors stay those of the RPC methods they replace.
func NewStatementError(message, kind string, details any) error {
	code := statementErrorCode(kind, details)
	return &RPCError{
		Code:    code,
		Message: message,
		wire: &wireError{
			Code:    code,
			Message: message,
			Kind:    kind,
			Details: details,
		},
	}
}

// RPC error codes SurrealDB v3 assigns to an error of a given kind and
// details. They mirror the code module and the kind constructors of the
// server's surrealdb/types/src/error.rs, which Error::from_wire dispatches
// through to rebuild an error from its kind and details.
const (
	codeParseError               = -32700
	codeInvalidRequest           = -32600
	codeMethodNotFound           = -32601
	codeMethodNotAllowed         = -32602
	codeInvalidParams            = -32603
	codeLiveQueryNotSupported    = -32604
	codeBadLiveQueryConfig       = -32605
	codeBadGraphQLConfig         = -32606
	codeDialectNotSupported      = -32607
	codeInternalError            = -32000
	codeClientSideError          = -32001
	codeInvalidAuth              = -32002
	codeQueryNotExecuted         = -32003
	codeQueryTimedOut            = -32004
	codeQueryCancelled           = -32005
	codeThrown                   = -32006
	codeSerializationError       = -32007
	codeDeserializationError     = -32008
	codeQueryTransactionConflict = -32009
)

// detailErrorCodes maps a kind and the kind of its details to the RPC error
// code SurrealDB assigns them, for the kinds whose code depends on their
// details.
var detailErrorCodes = map[string]map[string]int{
	KindQuery: {
		detailKindNotExecuted:         codeQueryNotExecuted,
		detailKindTimedOut:            codeQueryTimedOut,
		detailKindCancelled:           codeQueryCancelled,
		detailKindTransactionConflict: codeQueryTransactionConflict,
	},
	KindValidation: {
		detailKindParse:    codeParseError,
		"InvalidRequest":   codeInvalidRequest,
		"InvalidParams":    codeInvalidParams,
		"NamespaceEmpty":   codeInvalidRequest,
		"DatabaseEmpty":    codeInvalidRequest,
		"InvalidParameter": codeInvalidRequest,
		"InvalidContent":   codeInvalidRequest,
		"InvalidMerge":     codeInvalidRequest,
	},
	KindConfiguration: {
		detailKindLiveQueryNotSupported: codeLiveQueryNotSupported,
		"BadLiveQueryConfig":            codeBadLiveQueryConfig,
		"BadGraphqlConfig":              codeBadGraphQLConfig,
		"DialectNotSupported":           codeDialectNotSupported,
	},
	KindSerialization: {
		"Serialization":           codeSerializationError,
		detailKindDeserialization: codeDeserializationError,
	},
	KindNotAllowed: {
		"Method":            codeMethodNotAllowed,
		detailKindScripting: codeMethodNotAllowed,
		"Function":          codeMethodNotAllowed,
		"Target":            codeMethodNotAllowed,
	},
	KindNotFound: {
		"Method": codeMethodNotFound,
	},
}

// authErrorCodes maps the kind of the details of a NotAllowed/Auth error to
// the RPC error code SurrealDB assigns it. SessionExpired has none.
var authErrorCodes = map[string]int{
	detailKindTokenExpired: codeInvalidAuth,
	detailKindInvalidAuth:  codeInvalidAuth,
	"UnexpectedAuth":       codeInvalidAuth,
	"MissingUserOrPass":    codeInvalidAuth,
	"NoSigninTarget":       codeInvalidAuth,
	"InvalidPass":          codeInvalidAuth,
	"TokenMakingFailed":    codeInvalidAuth,
	"InvalidSignup":        codeInvalidAuth,
	"InvalidRole":          codeInvalidAuth,
	"NotAllowed":           codeInvalidAuth,
}

// statementErrorCode returns the RPC error code SurrealDB assigns an error of
// the given kind and details, or -32000 when it assigns none: for an unknown
// kind or detail kind, for kinds such as NotFound (other than a missing
// method) and AlreadyExists, and for a SurrealDB v2 error, which has no kind.
func statementErrorCode(kind string, details any) int {
	switch kind {
	case KindThrown:
		return codeThrown
	case KindConnection:
		return codeClientSideError
	}
	detail, _ := detailKind(details)
	codes := detailErrorCodes[kind]
	if kind == KindNotAllowed && detail == detailKindAuth {
		detail, _ = detailKind(nestedDetails(details))
		codes = authErrorCodes
	}
	if code, ok := codes[detail]; ok {
		return code
	}
	return codeInternalError
}
