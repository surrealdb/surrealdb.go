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
