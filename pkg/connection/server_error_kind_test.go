package connection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKindHelpers_TopLevelKind exercises the Is* helpers that only check the
// top-level ServerError.Kind (IsNotFound, IsNotAllowed), across a matching
// error, a same-family-but-different-detail error (to prove the helper isn't
// a tautology that always returns true for any *ServerError), a
// non-ServerError error, and nil.
func TestKindHelpers_TopLevelKind(t *testing.T) {
	tests := []struct {
		name string
		fn   func(error) bool
		kind string
	}{
		{"IsNotFound", IsNotFound, KindNotFound},
		{"IsNotAllowed", IsNotAllowed, KindNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("matching kind", func(t *testing.T) {
				err := &ServerError{Kind: tt.kind, Message: "boom"}
				assert.True(t, tt.fn(err))
			})

			t.Run("different kind", func(t *testing.T) {
				err := &ServerError{Kind: KindInternal, Message: "boom"}
				if tt.kind == KindInternal {
					err.Kind = KindThrown
				}
				assert.False(t, tt.fn(err))
			})

			t.Run("wrapped matching kind", func(t *testing.T) {
				inner := &ServerError{Kind: tt.kind, Message: "boom"}
				wrapped := &wrapError{err: inner}
				assert.True(t, tt.fn(wrapped))
			})

			t.Run("non-ServerError", func(t *testing.T) {
				assert.False(t, tt.fn(errors.New("boom")))
			})

			t.Run("nil", func(t *testing.T) {
				assert.False(t, tt.fn(nil))
			})
		})
	}
}

// wrapError wraps an error the same way callers commonly would (e.g. with
// fmt.Errorf("%w", err) or a custom error implementing Unwrap), to confirm
// the helpers unwrap via errors.As rather than requiring an exact
// *ServerError at the top of the chain.
type wrapError struct{ err error }

func (w *wrapError) Error() string { return "wrapped: " + w.err.Error() }
func (w *wrapError) Unwrap() error { return w.err }

// TestKindHelpers_NestedDetailKind exercises the Is* helpers that check a
// nested Details.kind alongside the top-level Kind (e.g. Query +
// TransactionConflict). For each helper we verify: true for a ServerError
// with the exact Kind+Details shape, false when the Kind matches but the
// nested detail kind doesn't (proving the helper actually inspects Details,
// not just Kind), false when the Kind doesn't match at all, false for a
// non-ServerError error, and false for nil.
func TestKindHelpers_NestedDetailKind(t *testing.T) {
	tests := []struct {
		name           string
		fn             func(error) bool
		kind           string
		matchingDetail any
		otherDetail    any
	}{
		{
			name:           "IsTransactionConflict",
			fn:             IsTransactionConflict,
			kind:           KindQuery,
			matchingDetail: map[string]any{"kind": "TransactionConflict"},
			otherDetail:    map[string]any{"kind": "TimedOut"},
		},
		{
			name:           "IsTimedOut",
			fn:             IsTimedOut,
			kind:           KindQuery,
			matchingDetail: map[string]any{"kind": "TimedOut"},
			otherDetail:    map[string]any{"kind": detailKindCancelled},
		},
		{
			name:           "IsNotExecuted",
			fn:             IsNotExecuted,
			kind:           KindQuery,
			matchingDetail: map[string]any{"kind": "NotExecuted"},
			otherDetail:    map[string]any{"kind": detailKindCancelled},
		},
		{
			name:           "IsCancelled",
			fn:             IsCancelled,
			kind:           KindQuery,
			matchingDetail: map[string]any{"kind": detailKindCancelled},
			otherDetail:    map[string]any{"kind": "NotExecuted"},
		},
		{
			name:           "IsParseError",
			fn:             IsParseError,
			kind:           KindValidation,
			matchingDetail: map[string]any{"kind": "Parse"},
			otherDetail:    map[string]any{"kind": "InvalidParams"},
		},
		{
			name:           "IsDeserialization",
			fn:             IsDeserialization,
			kind:           KindSerialization,
			matchingDetail: map[string]any{"kind": "Deserialization"},
			otherDetail:    map[string]any{"kind": "Serialization"},
		},
		{
			name:           "IsLiveQueryNotSupported",
			fn:             IsLiveQueryNotSupported,
			kind:           KindConfiguration,
			matchingDetail: map[string]any{"kind": "LiveQueryNotSupported"},
			otherDetail:    map[string]any{"kind": "BadGraphqlConfig"},
		},
		{
			name:           "IsScriptingBlocked",
			fn:             IsScriptingBlocked,
			kind:           KindNotAllowed,
			matchingDetail: map[string]any{"kind": "Scripting"},
			otherDetail:    map[string]any{"kind": "Method", "details": map[string]any{"name": "foo"}},
		},
		{
			name: "IsTokenExpired",
			fn:   IsTokenExpired,
			kind: KindNotAllowed,
			matchingDetail: map[string]any{
				"kind":    "Auth",
				"details": map[string]any{"kind": "TokenExpired"},
			},
			otherDetail: map[string]any{
				"kind":    "Auth",
				"details": map[string]any{"kind": "InvalidAuth"},
			},
		},
		{
			name: "IsInvalidAuth",
			fn:   IsInvalidAuth,
			kind: KindNotAllowed,
			matchingDetail: map[string]any{
				"kind":    "Auth",
				"details": map[string]any{"kind": "InvalidAuth"},
			},
			otherDetail: map[string]any{
				"kind":    "Auth",
				"details": map[string]any{"kind": "TokenExpired"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("matching kind and detail", func(t *testing.T) {
				err := &ServerError{Kind: tt.kind, Details: tt.matchingDetail}
				assert.True(t, tt.fn(err), "expected true for matching Kind+Details")
			})

			t.Run("matching kind, different detail", func(t *testing.T) {
				err := &ServerError{Kind: tt.kind, Details: tt.otherDetail}
				assert.False(t, tt.fn(err), "expected false when Details doesn't match")
			})

			t.Run("different kind entirely", func(t *testing.T) {
				otherKind := KindInternal
				if tt.kind == KindInternal {
					otherKind = KindThrown
				}
				err := &ServerError{Kind: otherKind, Details: tt.matchingDetail}
				assert.False(t, tt.fn(err), "expected false when top-level Kind doesn't match")
			})

			t.Run("non-ServerError", func(t *testing.T) {
				assert.False(t, tt.fn(errors.New("boom")))
			})

			t.Run("nil", func(t *testing.T) {
				assert.False(t, tt.fn(nil))
			})
		})
	}
}

// TestIsTokenExpired_and_IsInvalidAuth_AreDistinct proves the two auth
// helpers actually discriminate on the nested Auth sub-kind, rather than
// both simply matching any NotAllowed/Auth error indiscriminately.
func TestIsTokenExpired_and_IsInvalidAuth_AreDistinct(t *testing.T) {
	tokenExpired := &ServerError{
		Kind:    KindNotAllowed,
		Details: map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
	}
	invalidAuth := &ServerError{
		Kind:    KindNotAllowed,
		Details: map[string]any{"kind": "Auth", "details": map[string]any{"kind": "InvalidAuth"}},
	}

	assert.True(t, IsTokenExpired(tokenExpired))
	assert.False(t, IsInvalidAuth(tokenExpired))

	assert.True(t, IsInvalidAuth(invalidAuth))
	assert.False(t, IsTokenExpired(invalidAuth))
}

// TestKindHelpers_ServerErrorValue confirms the helpers also work when the
// *ServerError is reached indirectly (e.g. as the Cause of an outer
// ServerError), matching the cause-chain traversal already exercised in
// wire_error_test.go.
func TestKindHelpers_ServerErrorValue(t *testing.T) {
	outer := &ServerError{
		Kind:    KindInternal,
		Message: "outer",
		Cause: &ServerError{
			Kind:    KindQuery,
			Message: "inner",
			Details: map[string]any{"kind": "TransactionConflict"},
		},
	}

	assert.False(t, IsTransactionConflict(outer), "outer error itself is Internal, not Query")
	assert.True(t, IsTransactionConflict(errors.Unwrap(outer)), "unwrapped cause is the TransactionConflict")
}
