package surrealdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

// TestErrorKindHelpers_ReExports is a pure unit test (no live server needed)
// confirming that the top-level surrealdb package re-exports the
// pkg/connection Kind* constants and Is* helper functions, and that they
// behave identically when used through the surrealdb package alias.
func TestErrorKindHelpers_ReExports(t *testing.T) {
	notFoundErr := &surrealdb.ServerError{Kind: surrealdb.KindNotFound, Message: "no such table"}
	notAllowedErr := &surrealdb.ServerError{Kind: surrealdb.KindNotAllowed, Message: "denied"}
	conflictErr := &surrealdb.ServerError{
		Kind:    surrealdb.KindQuery,
		Details: map[string]any{"kind": "TransactionConflict"},
	}
	tokenExpiredErr := &surrealdb.ServerError{
		Kind:    surrealdb.KindNotAllowed,
		Details: map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
	}

	assert.True(t, surrealdb.IsNotFound(notFoundErr))
	assert.False(t, surrealdb.IsNotFound(notAllowedErr))

	assert.True(t, surrealdb.IsNotAllowed(notAllowedErr))
	assert.False(t, surrealdb.IsNotAllowed(notFoundErr))

	assert.True(t, surrealdb.IsTransactionConflict(conflictErr))
	assert.False(t, surrealdb.IsTransactionConflict(notFoundErr))

	assert.True(t, surrealdb.IsTokenExpired(tokenExpiredErr))
	assert.False(t, surrealdb.IsInvalidAuth(tokenExpiredErr))

	assert.False(t, surrealdb.IsNotFound(nil))
	assert.False(t, surrealdb.IsNotFound(assert.AnError))
}
