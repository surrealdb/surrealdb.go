package connection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWireError_As_RPCError_And_ServerError creates a wireError with all v2+v3
// fields populated (including a cause chain), then extracts both RPCError and
// ServerError via errors.As to show the information available in each.
//
// wireError fields:
//
//	Code, Message, Description (v2), Kind (v3), Details (v3), Cause (v3)
//
// RPCError (v2 backward compat) gets: Code, Message, Description — 3 fields.
// ServerError (v3 migration path) gets: Code, Message, Kind, Details, Cause — 5 fields.
func TestWireError_As_RPCError_And_ServerError(t *testing.T) {
	w := &wireError{
		Code:        -32002,
		Message:     "Token has expired",
		Description: "v2 description", // v2 only; v3 servers leave this empty
		Kind:        "NotAllowed",
		Details:     map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
		Cause: &wireError{
			Code:    -32000,
			Message: "Session invalidated",
			Kind:    "Internal",
			Details: map[string]any{"kind": "Session", "details": map[string]any{"id": "sess-123"}},
		},
	}

	// --- errors.As → *RPCError: v2 backward compat, fewer fields ---
	var rpcErr RPCError
	require.True(t, errors.As(w, &rpcErr))
	require.True(t, errors.Is(&rpcErr, &RPCError{}))        // RPCError is still in the Is chain
	require.True(t, errors.Is(error(&rpcErr), &RPCError{})) // pointer receiver works with Is
	assert.Equal(t, -32002, rpcErr.Code)
	assert.Equal(t, "Token has expired", rpcErr.Message)
	assert.Equal(t, "v2 description", rpcErr.Description)
	// RPCError does NOT expose Kind, Details, or Cause.

	// --- errors.As → *ServerError: v3 migration path, all fields ---
	var se ServerError
	require.True(t, errors.As(w, &se))
	require.True(t, errors.Is(se, ServerError{}))        // ServerError is in the Is chain
	require.True(t, errors.Is(error(se), ServerError{})) // pointer receiver works with Is
	assert.Equal(t, -32002, se.Code)
	assert.Equal(t, "Token has expired", se.Message)
	assert.Equal(t, "NotAllowed", se.Kind)
	assert.Equal(t, map[string]any{
		"kind":    "Auth",
		"details": map[string]any{"kind": "TokenExpired"},
	}, se.Details)

	// Cause chain: ServerError exposes the full recursive cause.
	require.Equal(t, &ServerError{
		Code:    -32000,
		Message: "Session invalidated",
		Kind:    "Internal",
		Details: map[string]any{
			"kind":    "Session",
			"details": map[string]any{"id": "sess-123"},
		},
		Cause: nil,
	}, se.Cause)

	var cause ServerError
	require.True(t, errors.As(errors.Unwrap(se), &cause))
	assert.Equal(t, ServerError{
		Code:    -32000,
		Message: "Session invalidated",
		Kind:    "Internal",
		Details: map[string]any{
			"kind":    "Session",
			"details": map[string]any{"id": "sess-123"},
		},
		Cause: nil,
	}, cause)

	var causePtr *ServerError
	require.True(t, errors.As(errors.Unwrap(se), &causePtr))
	assert.Equal(t, &ServerError{
		Code:    -32000,
		Message: "Session invalidated",
		Kind:    "Internal",
		Details: map[string]any{
			"kind":    "Session",
			"details": map[string]any{"id": "sess-123"},
		},
		Cause: nil,
	}, causePtr)

	// Error() joins the cause chain with ": ".
	assert.Equal(t, "Token has expired: Session invalidated", se.Error())
}

// TestWireError_As_ZeroValueCause verifies that a non-nil but zero-value
// *wireError cause (e.g. from CBOR deserializing an empty cause object)
// does NOT produce a spurious ServerError.Cause.
func TestWireError_As_ZeroValueCause(t *testing.T) {
	w := &wireError{
		Code:    -32002,
		Message: "Auth failed",
		Kind:    "NotAllowed",
		Cause:   &wireError{}, // zero-value: server sent empty cause object
	}

	var se ServerError
	require.True(t, errors.As(w, &se))
	assert.Equal(t, -32002, se.Code)
	assert.Equal(t, "Auth failed", se.Message)
	assert.Equal(t, "NotAllowed", se.Kind)
	assert.Nil(t, se.Cause, "zero-value wireError cause should not produce a ServerError.Cause")

	var sePtr *ServerError
	require.True(t, errors.As(w, &sePtr))
	assert.Nil(t, sePtr.Cause, "zero-value wireError cause should not produce a ServerError.Cause (pointer)")
}

// TestNewStatementError_Code checks that a failed statement gets the RPC error
// code SurrealDB assigns an RPC error of its kind and details, as the server's
// Error::from_wire does, rather than always -32000.
func TestNewStatementError_Code(t *testing.T) {
	d := func(kind string) map[string]any { return map[string]any{"kind": kind} }
	auth := func(kind string) map[string]any {
		return map[string]any{"kind": "Auth", "details": map[string]any{"kind": kind}}
	}

	tests := []struct {
		name    string
		kind    string
		details any
		code    int
	}{
		{"v2 error without a kind", "", nil, -32000},
		{"thrown", KindThrown, nil, -32006},
		{"transaction conflict", KindQuery, d("TransactionConflict"), -32009},
		{"timed out", KindQuery, map[string]any{"kind": "TimedOut", "details": map[string]any{"duration": "1ns"}}, -32004},
		{"cancelled", KindQuery, d("Cancelled"), -32005}, //nolint:misspell // the server's spelling
		{"not executed", KindQuery, d("NotExecuted"), -32003},
		{"query without details", KindQuery, nil, -32000},
		{"parse", KindValidation, d("Parse"), -32700},
		{"invalid params", KindValidation, d("InvalidParams"), -32603},
		{"invalid request", KindValidation, d("InvalidRequest"), -32600},
		{"invalid content", KindValidation, map[string]any{"kind": "InvalidContent", "details": map[string]any{"value": "x"}}, -32600},
		{"namespace empty", KindValidation, d("NamespaceEmpty"), -32600},
		{"live query not supported", KindConfiguration, d("LiveQueryNotSupported"), -32604},
		{"bad graphql config", KindConfiguration, d("BadGraphqlConfig"), -32606},
		{"serialization", KindSerialization, d("Serialization"), -32007},
		{"deserialization", KindSerialization, d("Deserialization"), -32008},
		{"scripting not allowed", KindNotAllowed, d("Scripting"), -32602},
		{"function not allowed", KindNotAllowed, map[string]any{"kind": "Function", "details": map[string]any{"name": "http::get"}}, -32602},
		{"token expired", KindNotAllowed, auth("TokenExpired"), -32002},
		{"invalid auth", KindNotAllowed, auth("InvalidAuth"), -32002},
		{"session expired", KindNotAllowed, auth("SessionExpired"), -32000},
		{"method not found", KindNotFound, map[string]any{"kind": "Method", "details": map[string]any{"name": "nope"}}, -32601},
		{"table not found", KindNotFound, map[string]any{"kind": "Table", "details": map[string]any{"name": "t"}}, -32000},
		{"record already exists", KindAlreadyExists, map[string]any{"kind": "Record", "details": map[string]any{"id": "t:1"}}, -32000},
		{"internal", KindInternal, nil, -32000},
		{"unknown kind", "Telepathic", d("TransactionConflict"), -32000},
		{"unknown detail kind", KindQuery, d("Telepathic"), -32000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStatementError("message", tt.kind, tt.details)

			var rpcErr *RPCError
			require.True(t, errors.As(err, &rpcErr))
			assert.Equal(t, tt.code, rpcErr.Code)

			var se *ServerError
			require.True(t, errors.As(err, &se))
			assert.Equal(t, tt.code, se.Code)
			assert.Equal(t, tt.kind, se.Kind)
			assert.Equal(t, tt.details, se.Details)
		})
	}
}
