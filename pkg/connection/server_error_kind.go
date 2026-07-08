package connection

import "errors"

// Known values for ServerError.Kind, matching the top-level error kinds
// returned by SurrealDB v3's structured error wire format ({ "kind": "...",
// "details": ... }).
//
// These mirror the ErrorKind map in surrealdb.js (packages/sdk/src/errors.ts)
// and the ErrorDetails variants in the server's surrealdb/types/src/error.rs
// (see ErrorDetails::kind_str). Servers may introduce new kinds over time;
// unknown kinds should be treated the same as KindInternal by callers.
const (
	KindValidation    = "Validation"
	KindConfiguration = "Configuration"
	KindThrown        = "Thrown"
	KindQuery         = "Query"
	KindSerialization = "Serialization"
	KindNotAllowed    = "NotAllowed"
	KindNotFound      = "NotFound"
	KindAlreadyExists = "AlreadyExists"
	KindConnection    = "Connection"
	KindInternal      = "Internal"
)

// Nested detail-kind values used to further discriminate within a top-level
// Kind. On SurrealDB v3, ServerError.Details typically decodes to
// map[string]any{"kind": "<detailKind>", "details": ...} (or, for
// double-nested details such as auth failures, another map of the same
// shape). These are unexported because, unlike the top-level Kind values,
// they are only meaningful in combination with a specific top-level Kind --
// callers should use the Is* helpers below rather than comparing directly.
const (
	detailKindAuth         = "Auth"
	detailKindTokenExpired = "TokenExpired"
	detailKindInvalidAuth  = "InvalidAuth"
	detailKindScripting    = "Scripting"
	detailKindParse        = "Parse"
	detailKindNotExecuted  = "NotExecuted"
	// detailKindCancelled matches the server's wire value verbatim (a
	// British-spelling variant name shared by the Rust and JS sources) --
	// not a typo.
	detailKindCancelled             = "Cancelled" //nolint:misspell
	detailKindTransactionConflict   = "TransactionConflict"
	detailKindTimedOut              = "TimedOut"
	detailKindDeserialization       = "Deserialization"
	detailKindLiveQueryNotSupported = "LiveQueryNotSupported"
)

// hasKind reports whether err unwraps (via errors.As) to a *ServerError with
// the given top-level Kind. Returns false for nil, for errors that aren't
// (or don't wrap) a *ServerError, and for a *ServerError with a different
// Kind.
func hasKind(err error, kind string) bool {
	var se *ServerError
	if !errors.As(err, &se) {
		return false
	}
	return se.Kind == kind
}

// detailKind extracts the "kind" string from a details value shaped like
// map[string]any{"kind": "...", "details": ...} -- the shape ServerError.Details
// takes on SurrealDB v3 after CBOR/JSON decoding. ok is false if details is
// nil, isn't a map, or has no string "kind" entry.
func detailKind(details any) (kind string, ok bool) {
	m, isMap := details.(map[string]any)
	if !isMap {
		return "", false
	}
	kind, ok = m["kind"].(string)
	return kind, ok
}

// nestedDetails returns the "details" entry nested inside a details map, e.g.
// the AuthErrorDetail nested inside a NotAllowedErrorDetail's
// { "kind": "Auth", "details": { "kind": "TokenExpired" } } shape. Returns
// nil if details isn't shaped that way.
func nestedDetails(details any) any {
	m, ok := details.(map[string]any)
	if !ok {
		return nil
	}
	return m["details"]
}

// hasDetailKind reports whether err is a *ServerError whose top-level Kind is
// kind and whose Details map has the nested "kind" equal to detailKind.
func hasDetailKind(err error, kind, wantDetailKind string) bool {
	var se *ServerError
	if !errors.As(err, &se) {
		return false
	}
	if se.Kind != kind {
		return false
	}
	got, ok := detailKind(se.Details)
	return ok && got == wantDetailKind
}

// hasAuthDetailKind reports whether err is a *ServerError with Kind
// "NotAllowed" whose Details is shaped like
// { "kind": "Auth", "details": { "kind": wantAuthKind } }, i.e. an IAM/auth
// failure of the given sub-kind.
func hasAuthDetailKind(err error, wantAuthKind string) bool {
	var se *ServerError
	if !errors.As(err, &se) {
		return false
	}
	if se.Kind != KindNotAllowed {
		return false
	}
	dk, ok := detailKind(se.Details)
	if !ok || dk != detailKindAuth {
		return false
	}
	inner, ok := detailKind(nestedDetails(se.Details))
	return ok && inner == wantAuthKind
}

// IsNotFound reports whether err is (or wraps) a *ServerError whose Kind is
// "NotFound" -- a missing table, record, namespace, database, session, RPC
// method, or transaction. Returns false for nil, for errors that aren't a
// *ServerError, and for a *ServerError with a different Kind.
func IsNotFound(err error) bool {
	return hasKind(err, KindNotFound)
}

// IsNotAllowed reports whether err is (or wraps) a *ServerError whose Kind is
// "NotAllowed" -- a permission failure, disallowed method/function/net
// target, blocked scripting, or authentication failure. Returns false for
// nil, for errors that aren't a *ServerError, and for a *ServerError with a
// different Kind.
func IsNotAllowed(err error) bool {
	return hasKind(err, KindNotAllowed)
}

// IsTransactionConflict reports whether err is a *ServerError representing a
// Query/TransactionConflict failure -- a concurrent transaction wrote to the
// same data. Such errors are safe to retry. Mirrors
// QueryError.isTransactionConflict in surrealdb.js's errors.ts.
func IsTransactionConflict(err error) bool {
	return hasDetailKind(err, KindQuery, detailKindTransactionConflict)
}

// IsTimedOut reports whether err is a *ServerError representing a
// Query/TimedOut failure -- the query exceeded its configured timeout.
// Mirrors QueryError.isTimedOut in surrealdb.js's errors.ts.
func IsTimedOut(err error) bool {
	return hasDetailKind(err, KindQuery, detailKindTimedOut)
}

// IsNotExecuted reports whether err is a *ServerError representing a
// Query/NotExecuted failure -- the query was never run, typically because an
// earlier statement in the same batch/transaction failed. Mirrors
// QueryError.isNotExecuted in surrealdb.js's errors.ts.
func IsNotExecuted(err error) bool {
	return hasDetailKind(err, KindQuery, detailKindNotExecuted)
}

// IsCancelled reports whether err is a *ServerError representing a canceled
// Query (see detailKindCancelled for the server's exact wire spelling).
// Mirrors QueryError.isCancelled in surrealdb.js's errors.ts.
func IsCancelled(err error) bool {
	return hasDetailKind(err, KindQuery, detailKindCancelled)
}

// IsParseError reports whether err is a *ServerError representing a
// Validation/Parse failure -- a SurrealQL parse error. Mirrors
// ValidationError.isParseError in surrealdb.js's errors.ts.
func IsParseError(err error) bool {
	return hasDetailKind(err, KindValidation, detailKindParse)
}

// IsDeserialization reports whether err is a *ServerError representing a
// Serialization/Deserialization failure, as opposed to a serialization
// failure. Mirrors SerializationError.isDeserialization in surrealdb.js's
// errors.ts.
func IsDeserialization(err error) bool {
	return hasDetailKind(err, KindSerialization, detailKindDeserialization)
}

// IsLiveQueryNotSupported reports whether err is a *ServerError representing
// a Configuration/LiveQueryNotSupported failure -- live queries are not
// supported by the server configuration. Mirrors
// ConfigurationError.isLiveQueryNotSupported in surrealdb.js's errors.ts.
func IsLiveQueryNotSupported(err error) bool {
	return hasDetailKind(err, KindConfiguration, detailKindLiveQueryNotSupported)
}

// IsScriptingBlocked reports whether err is a *ServerError representing a
// NotAllowed/Scripting failure -- server-side scripting is blocked. Mirrors
// NotAllowedError.isScriptingBlocked in surrealdb.js's errors.ts.
func IsScriptingBlocked(err error) bool {
	return hasDetailKind(err, KindNotAllowed, detailKindScripting)
}

// IsTokenExpired reports whether err is a *ServerError representing a
// NotAllowed/Auth/TokenExpired failure -- the auth token used for the
// request has expired. Mirrors NotAllowedError.isTokenExpired in
// surrealdb.js's errors.ts.
func IsTokenExpired(err error) bool {
	return hasAuthDetailKind(err, detailKindTokenExpired)
}

// IsInvalidAuth reports whether err is a *ServerError representing a
// NotAllowed/Auth/InvalidAuth failure -- the supplied credentials were
// invalid. Mirrors NotAllowedError.isInvalidAuth in surrealdb.js's
// errors.ts.
func IsInvalidAuth(err error) bool {
	return hasAuthDetailKind(err, detailKindInvalidAuth)
}
