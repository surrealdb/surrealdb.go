package surrealdb

import (
	"errors"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// ------------------------------------------------------------------ //
//  ErrorKind constants                                                //
// ------------------------------------------------------------------ //

// Known error kinds returned by the SurrealDB server.
// Use these constants for matching against ServerError.Kind().
const (
	ErrorKindValidation    = "Validation"
	ErrorKindConfiguration = "Configuration"
	ErrorKindThrown        = "Thrown"
	ErrorKindQuery         = "Query"
	ErrorKindSerialization = "Serialization"
	ErrorKindNotAllowed    = "NotAllowed"
	ErrorKindNotFound      = "NotFound"
	ErrorKindAlreadyExists = "AlreadyExists"
	ErrorKindConnection    = "Connection"
	ErrorKindInternal      = "Internal"
)

// ------------------------------------------------------------------ //
//  Legacy code-to-kind mapping                                        //
// ------------------------------------------------------------------ //

// codeToKind maps legacy JSON-RPC error codes to ErrorKind values.
// Used when kind is absent (old server format).
var codeToKind = map[int]string{
	-32700: ErrorKindValidation,
	-32600: ErrorKindValidation,
	-32603: ErrorKindValidation,
	-32601: ErrorKindNotFound,
	-32602: ErrorKindNotAllowed,
	-32002: ErrorKindNotAllowed,
	-32604: ErrorKindConfiguration,
	-32605: ErrorKindConfiguration,
	-32606: ErrorKindConfiguration,
	-32000: ErrorKindInternal,
	-32001: ErrorKindConnection,
	-32003: ErrorKindQuery,
	-32004: ErrorKindQuery,
	-32005: ErrorKindQuery,
	-32006: ErrorKindThrown,
	-32007: ErrorKindSerialization,
	-32008: ErrorKindSerialization,
}

// resolveKind determines the error kind from kind and/or legacy code.
func resolveKind(kind string, code int) string {
	if kind != "" {
		return kind
	}
	if k, ok := codeToKind[code]; ok {
		return k
	}
	return ErrorKindInternal
}

// ------------------------------------------------------------------ //
//  Detail helpers                                                     //
// ------------------------------------------------------------------ //
//
// SurrealDB v3 switched from serde externally-tagged enums to a
// recursive { "kind": "...", "details": ... } format for error details.
//
// Old (v2, externally-tagged):
//   Unit:    "Parse"
//   Newtype: { "Auth": "TokenExpired" }
//   Struct:  { "Table": { "name": "users" } }
//
// New (v3, internally-tagged):
//   Unit:    { "kind": "Parse" }
//   Newtype: { "kind": "Auth", "details": { "kind": "TokenExpired" } }
//   Struct:  { "kind": "Table", "details": { "name": "users" } }
//
// All helpers support both formats for backward compatibility.

// detailKind returns the "kind" string from a { kind, details? } detail
// object. Returns "" if details is not in internally-tagged format.
func detailKind(details any) string {
	m, ok := details.(map[string]any)
	if !ok {
		return ""
	}
	k, _ := m["kind"].(string)
	return k
}

// detailInner returns the "details" value from a { kind, details? }
// detail object. Returns nil if not present.
func detailInner(details any) any {
	m, ok := details.(map[string]any)
	if !ok {
		return nil
	}
	return m["details"]
}

// hasDetailKey checks if details matches the given variant key.
// Supports both old and new formats.
func hasDetailKey(details any, key string) bool {
	// New format: { "kind": "Parse" }
	if detailKind(details) == key {
		return true
	}
	// Old format: "Parse" (bare string) or { "Parse": ... } (map key)
	switch d := details.(type) {
	case string:
		return d == key
	case map[string]any:
		_, ok := d[key]
		return ok
	}
	return false
}

// getDetailValue returns the inner value for a given variant key.
// New format: if kind == key, returns details["details"].
// Old format: returns details[key].
func getDetailValue(details any, key string) any {
	// New format: { "kind": "Auth", "details": { "kind": "TokenExpired" } }
	if detailKind(details) == key {
		return detailInner(details)
	}
	// Old format: { "Auth": "TokenExpired" }
	if d, ok := details.(map[string]any); ok {
		return d[key]
	}
	return nil
}

// getDetailString returns a string from the inner details of a variant.
// New format: if kind == key and details["details"] is { "kind": val }, returns val.
// Also handles direct string inner for old newtype variants.
func getDetailString(details any, key string) string {
	inner := getDetailValue(details, key)
	if inner == nil {
		return ""
	}
	// New format newtype: inner is { "kind": "TokenExpired" }
	if k := detailKind(inner); k != "" {
		return k
	}
	// Old format newtype: inner is "TokenExpired"
	if s, ok := inner.(string); ok {
		return s
	}
	return ""
}

// getDetailMapString extracts a string field from a variant's inner details.
//
// New format: { "kind": "Table", "details": { "name": "users" } }
//   -> getDetailMapString(d, "Table", "name") returns "users"
//
// Old format: { "Table": { "name": "users" } }
//   -> getDetailMapString(d, "Table", "name") returns "users"
func getDetailMapString(details any, key, field string) string {
	v := getDetailValue(details, key)
	if m, ok := v.(map[string]any); ok {
		if s, ok := m[field].(string); ok {
			return s
		}
	}
	return ""
}

// ------------------------------------------------------------------ //
//  ServerError                                                        //
// ------------------------------------------------------------------ //

// ServerError represents an error originating from the SurrealDB server.
//
// Server errors carry structured information:
//   - Kind — the error category (e.g. "NotAllowed", "NotFound")
//   - Code — legacy JSON-RPC numeric error code (0 when unavailable)
//   - Details — kind-specific structured details from the server
//   - Cause — the underlying server error, if any (recursive chain)
//
// Use the helper functions IsNotAllowed, IsNotFound, etc. for ergonomic
// kind checking, or inspect Kind() directly. Use errors.As to extract
// a *ServerError from an error chain.
type ServerError struct {
	kind    string
	code    int
	message string
	details any          // string, map[string]any, or nil
	cause   *ServerError // recursive cause chain
}

// Error implements the error interface.
// When a cause chain is present, the messages are joined with ": "
// so that the full chain is visible in logs and fmt output, since
// Go has no automatic error-chain display like JS or Python.
func (e *ServerError) Error() string {
	if e.cause == nil {
		return e.message
	}
	return e.message + ": " + e.cause.Error()
}

// Message returns the server's error message for this error only,
// without appending the cause chain. Use Error() for the full chain.
func (e *ServerError) Message() string {
	return e.message
}

// Kind returns the structured error kind (e.g. "NotAllowed", "NotFound", "Internal").
func (e *ServerError) Kind() string {
	return e.kind
}

// Code returns the legacy JSON-RPC error code. 0 when not available
// (e.g. query result errors).
func (e *ServerError) Code() int {
	return e.code
}

// Details returns the kind-specific structured details.
// In SurrealDB v3, this follows the { "kind": "...", "details": ... } format.
// In older versions, this may be a string (unit variants) or a map with the
// variant name as key (externally-tagged format).
// Returns nil when not provided by the server.
func (e *ServerError) Details() any {
	return e.details
}

// ServerCause returns the underlying server error in the chain, if any.
func (e *ServerError) ServerCause() *ServerError {
	return e.cause
}

// Unwrap implements the Go errors.Unwrap interface, enabling
// errors.Unwrap(), errors.Is(), and errors.As() to traverse the
// server error cause chain.
func (e *ServerError) Unwrap() error {
	if e.cause == nil {
		return nil
	}
	return e.cause
}

// Is supports errors.Is matching.
// A zero-kind target acts as a catch-all: errors.Is(err, &ServerError{})
// matches any *ServerError. A target with a non-empty kind only matches
// if this error has the same kind, enabling
// errors.Is(err, &ServerError{kind: "NotFound"}).
func (e *ServerError) Is(target error) bool {
	t, ok := target.(*ServerError)
	if !ok {
		return false
	}
	return t.kind == "" || t.kind == e.kind
}

// HasKind checks if this error or any error in the cause chain matches
// the given kind.
func (e *ServerError) HasKind(kind string) bool {
	if e.kind == kind {
		return true
	}
	if e.cause != nil {
		return e.cause.HasKind(kind)
	}
	return false
}

// FindCause finds the first error in the cause chain (including this error)
// that matches the given kind. Returns nil if no match is found.
func (e *ServerError) FindCause(kind string) *ServerError {
	if e.kind == kind {
		return e
	}
	if e.cause != nil {
		return e.cause.FindCause(kind)
	}
	return nil
}

// ------------------------------------------------------------------ //
//  Convenience detail accessors                                       //
// ------------------------------------------------------------------ //

// --- Validation ---

// IsParseError returns true if this is a SurrealQL parse error.
// Only meaningful when Kind() is ErrorKindValidation.
func (e *ServerError) IsParseError() bool {
	return hasDetailKey(e.details, "Parse")
}

// ParameterName returns the name of the invalid parameter, if applicable.
// Only meaningful when Kind() is ErrorKindValidation.
func (e *ServerError) ParameterName() string {
	return getDetailMapString(e.details, "InvalidParameter", "name")
}

// --- Configuration ---

// IsLiveQueryNotSupported returns true if live queries are not supported
// by the server configuration.
// Only meaningful when Kind() is ErrorKindConfiguration.
func (e *ServerError) IsLiveQueryNotSupported() bool {
	return hasDetailKey(e.details, "LiveQueryNotSupported")
}

// --- Query ---

// IsNotExecuted returns true if the query was not executed (e.g. due to
// a prior error in the batch).
// Only meaningful when Kind() is ErrorKindQuery.
func (e *ServerError) IsNotExecuted() bool {
	return hasDetailKey(e.details, "NotExecuted")
}

// IsTimedOut returns true if the query timed out.
// Only meaningful when Kind() is ErrorKindQuery.
func (e *ServerError) IsTimedOut() bool {
	return hasDetailKey(e.details, "TimedOut")
}

// IsCancelled returns true if the query was cancelled.
// Only meaningful when Kind() is ErrorKindQuery.
func (e *ServerError) IsCancelled() bool {
	return hasDetailKey(e.details, "Cancelled")
}

// Timeout returns the timeout duration as (secs, nanos) if this is a
// timeout error. The ok return value is false if this is not a timeout
// error or the duration is not available.
// Only meaningful when Kind() is ErrorKindQuery.
func (e *ServerError) Timeout() (secs int, nanos int, ok bool) {
	v := getDetailValue(e.details, "TimedOut")
	m, mOk := v.(map[string]any)
	if !mOk {
		return 0, 0, false
	}
	dur, dOk := m["duration"].(map[string]any)
	if !dOk {
		return 0, 0, false
	}
	s, sOk := toInt(dur["secs"])
	n, nOk := toInt(dur["nanos"])
	if !sOk || !nOk {
		return 0, 0, false
	}
	return s, n, true
}

// --- Serialization ---

// IsDeserialization returns true if this is a deserialization error
// (as opposed to serialization).
// Only meaningful when Kind() is ErrorKindSerialization.
func (e *ServerError) IsDeserialization() bool {
	return hasDetailKey(e.details, "Deserialization")
}

// --- NotAllowed ---

// IsTokenExpired returns true if the auth token has expired.
// Only meaningful when Kind() is ErrorKindNotAllowed.
func (e *ServerError) IsTokenExpired() bool {
	return getDetailString(e.details, "Auth") == "TokenExpired"
}

// IsInvalidAuth returns true if authentication credentials are invalid.
// Only meaningful when Kind() is ErrorKindNotAllowed.
func (e *ServerError) IsInvalidAuth() bool {
	return getDetailString(e.details, "Auth") == "InvalidAuth"
}

// IsScriptingBlocked returns true if scripting is blocked.
// Only meaningful when Kind() is ErrorKindNotAllowed.
func (e *ServerError) IsScriptingBlocked() bool {
	return hasDetailKey(e.details, "Scripting")
}

// MethodName returns the method name that is not allowed or not found,
// if applicable. Works for both ErrorKindNotAllowed and ErrorKindNotFound.
func (e *ServerError) MethodName() string {
	return getDetailMapString(e.details, "Method", "name")
}

// FunctionName returns the function name that is not allowed, if applicable.
// Only meaningful when Kind() is ErrorKindNotAllowed.
func (e *ServerError) FunctionName() string {
	return getDetailMapString(e.details, "Function", "name")
}

// TargetName returns the target name that is not allowed, if applicable.
// Only meaningful when Kind() is ErrorKindNotAllowed.
func (e *ServerError) TargetName() string {
	return getDetailMapString(e.details, "Target", "name")
}

// --- NotFound / AlreadyExists ---

// TableName returns the table name that was not found or already exists,
// if applicable. Works for both ErrorKindNotFound and ErrorKindAlreadyExists.
func (e *ServerError) TableName() string {
	return getDetailMapString(e.details, "Table", "name")
}

// RecordID returns the record ID that was not found or already exists,
// if applicable. Works for both ErrorKindNotFound and ErrorKindAlreadyExists.
func (e *ServerError) RecordID() string {
	return getDetailMapString(e.details, "Record", "id")
}

// NamespaceName returns the namespace name that was not found or already exists,
// if applicable. Works for both ErrorKindNotFound and ErrorKindAlreadyExists.
func (e *ServerError) NamespaceName() string {
	return getDetailMapString(e.details, "Namespace", "name")
}

// DatabaseName returns the database name that was not found or already exists,
// if applicable. Works for both ErrorKindNotFound and ErrorKindAlreadyExists.
func (e *ServerError) DatabaseName() string {
	return getDetailMapString(e.details, "Database", "name")
}

// SessionID returns the session ID that was not found or already exists,
// if applicable. Works for both ErrorKindNotFound and ErrorKindAlreadyExists.
func (e *ServerError) SessionID() string {
	return getDetailMapString(e.details, "Session", "id")
}

// ------------------------------------------------------------------ //
//  Parsing                                                            //
// ------------------------------------------------------------------ //

// parseRpcError parses an RPC-level error (connection.RPCError) into a
// *ServerError. Handles both old format (code + message) and new format
// (code + kind + message + details + cause).
func parseRpcError(raw *connection.RPCError) *ServerError {
	var cause *ServerError
	if raw.Cause != nil {
		cause = parseRpcError(raw.Cause)
	}
	return &ServerError{
		kind:    resolveKind(raw.Kind, raw.Code),
		code:    raw.Code,
		message: raw.Message,
		details: raw.Details,
		cause:   cause,
	}
}

// parseQueryError parses a query result error into a *ServerError.
// Query result errors use result as the message field and have no code.
//
// The server's query result path may double-wrap details: the outer object
// duplicates the error kind (e.g. { "kind": "AlreadyExists", "details":
// { "kind": "Record", ... } }). When the outer kind matches the resolved
// error kind, we unwrap to expose the inner detail directly.
func parseQueryError(message string, kind string, details any, rawCause *connection.RPCError) *ServerError {
	var cause *ServerError
	if rawCause != nil {
		cause = parseRpcError(rawCause)
	}

	resolved := resolveKind(kind, 0)

	// Unwrap double-wrapped details when the outer kind matches the error kind.
	if dk := detailKind(details); dk != "" && dk == resolved {
		if inner := detailInner(details); inner != nil {
			details = inner
		}
	}

	return &ServerError{
		kind:    resolved,
		code:    0,
		message: message,
		details: details,
		cause:   cause,
	}
}

// convertError converts an error from the connection layer into a
// *ServerError if the error is an *RPCError. Other errors pass through
// unchanged.
func convertError(err error) error {
	var rpcErr *connection.RPCError
	if errors.As(err, &rpcErr) {
		return parseRpcError(rpcErr)
	}
	return err
}

// ------------------------------------------------------------------ //
//  Top-level helper functions                                         //
// ------------------------------------------------------------------ //

// IsServerError reports whether err or any error in its chain is a *ServerError.
func IsServerError(err error) bool {
	var se *ServerError
	return errors.As(err, &se)
}

// IsValidation reports whether err or any error in its chain is a
// *ServerError with kind Validation.
func IsValidation(err error) bool {
	return hasErrorKind(err, ErrorKindValidation)
}

// IsConfiguration reports whether err or any error in its chain is a
// *ServerError with kind Configuration.
func IsConfiguration(err error) bool {
	return hasErrorKind(err, ErrorKindConfiguration)
}

// IsThrown reports whether err or any error in its chain is a
// *ServerError with kind Thrown.
func IsThrown(err error) bool {
	return hasErrorKind(err, ErrorKindThrown)
}

// IsQueryError reports whether err or any error in its chain is a
// *ServerError with kind Query.
func IsQueryError(err error) bool {
	return hasErrorKind(err, ErrorKindQuery)
}

// IsSerialization reports whether err or any error in its chain is a
// *ServerError with kind Serialization.
func IsSerialization(err error) bool {
	return hasErrorKind(err, ErrorKindSerialization)
}

// IsNotAllowed reports whether err or any error in its chain is a
// *ServerError with kind NotAllowed.
func IsNotAllowed(err error) bool {
	return hasErrorKind(err, ErrorKindNotAllowed)
}

// IsNotFound reports whether err or any error in its chain is a
// *ServerError with kind NotFound.
func IsNotFound(err error) bool {
	return hasErrorKind(err, ErrorKindNotFound)
}

// IsAlreadyExists reports whether err or any error in its chain is a
// *ServerError with kind AlreadyExists.
func IsAlreadyExists(err error) bool {
	return hasErrorKind(err, ErrorKindAlreadyExists)
}

// IsConnectionError reports whether err or any error in its chain is a
// *ServerError with kind Connection.
func IsConnectionError(err error) bool {
	return hasErrorKind(err, ErrorKindConnection)
}

// IsInternal reports whether err or any error in its chain is a
// *ServerError with kind Internal.
func IsInternal(err error) bool {
	return hasErrorKind(err, ErrorKindInternal)
}

// HasKind reports whether err contains a *ServerError whose cause chain
// includes the given kind.
func HasKind(err error, kind string) bool {
	var se *ServerError
	if errors.As(err, &se) {
		return se.HasKind(kind)
	}
	return false
}

// FindCause extracts the first *ServerError in the cause chain
// (including the error itself) that matches the given kind.
// Returns nil if no match is found or err is not a *ServerError.
func FindCause(err error, kind string) *ServerError {
	var se *ServerError
	if errors.As(err, &se) {
		return se.FindCause(kind)
	}
	return nil
}

// hasErrorKind is a shared helper for the Is* functions.
func hasErrorKind(err error, kind string) bool {
	var se *ServerError
	if errors.As(err, &se) {
		return se.kind == kind
	}
	return false
}

// toInt converts a numeric value from CBOR/JSON decoding to int.
// CBOR may decode numbers as various numeric types.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case uint64:
		return int(n), true
	}
	return 0, false
}
