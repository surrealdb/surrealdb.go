package testenv

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

func setupStructuredErrorTest(t *testing.T) *surrealdb.DB {
	t.Helper()

	db, err := New("test_errors", "structured_errors", "person")
	if err != nil {
		t.Skipf("SurrealDB not available: %v", err)
	}

	t.Cleanup(func() { db.Close(context.Background()) })

	v, err := GetVersion(context.Background(), db)
	if err != nil {
		t.Skipf("Could not get SurrealDB version: %v", err)
	}

	if !v.IsV3OrLater() {
		t.Skipf("Structured errors require SurrealDB v3+, got %s", v)
	}

	return db
}

func TestStructuredErrors_InvalidCredentials(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := db.SignIn(context.Background(), surrealdb.Auth{
		Username: "invalid",
		Password: "invalid",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")

	// SignIn returns the raw RPCError from the connection layer.
	// Verify the error carries the structured fields.
	var rpcErr *connection.RPCError
	if errors.As(err, &rpcErr) {
		assert.Equal(t, -32002, rpcErr.Code)
		assert.Equal(t, "NotAllowed", rpcErr.Kind)
	}
}

func TestStructuredErrors_InvalidSyntax(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := surrealdb.Query[any](context.Background(), db, "SEL ECT * FORM person", nil) //nolint:misspell // intentionally malformed SurrealQL

	require.Error(t, err)

	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se))
	assert.Contains(t, se.Error(), "Parse error")
}

func TestStructuredErrors_UserThrow(t *testing.T) {
	db := setupStructuredErrorTest(t)

	res, err := surrealdb.Query[any](context.Background(), db, `THROW "custom user error"`, nil)

	require.Error(t, err)

	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se))
	assert.Equal(t, surrealdb.ErrorKindThrown, se.Kind())
	assert.Contains(t, se.Error(), "custom user error")

	if res != nil && len(*res) > 0 {
		qr := (*res)[0]
		require.NotNil(t, qr.Error)
		assert.Equal(t, surrealdb.ErrorKindThrown, qr.Error.Kind())
	}
}

func TestStructuredErrors_DuplicateRecord(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := surrealdb.Query[any](context.Background(), db,
		`CREATE person:dup SET name = "first"`, nil)
	require.NoError(t, err)

	res, err := surrealdb.Query[any](context.Background(), db,
		`CREATE person:dup SET name = "second"`, nil)

	require.Error(t, err)

	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se))
	assert.Equal(t, surrealdb.ErrorKindAlreadyExists, se.Kind())
	assert.Contains(t, se.Error(), "person:dup")
	assert.Equal(t, "person:dup", se.RecordID())

	if res != nil && len(*res) > 0 {
		qr := (*res)[0]
		require.NotNil(t, qr.Error)
		assert.Equal(t, surrealdb.ErrorKindAlreadyExists, qr.Error.Kind())
		assert.Equal(t, "person:dup", qr.Error.RecordID())
	}
}

func TestStructuredErrors_SchemaViolation(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := surrealdb.Query[any](context.Background(), db,
		`DEFINE FIELD age ON person TYPE int`, nil)
	require.NoError(t, err)

	res, err := surrealdb.Query[any](context.Background(), db,
		`CREATE person:schema_test SET age = "not a number"`, nil)

	require.Error(t, err)

	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se))
	assert.True(t, surrealdb.IsServerError(err))

	if res != nil && len(*res) > 0 {
		qr := (*res)[0]
		require.NotNil(t, qr.Error)
		assert.Contains(t, qr.Error.Error(), "age")
	}
}

func TestStructuredErrors_MultiStatementMixed(t *testing.T) {
	db := setupStructuredErrorTest(t)

	res, err := surrealdb.Query[any](context.Background(), db,
		`RETURN 1; THROW "fail"; RETURN 3`, nil)

	require.Error(t, err, "multi-statement with THROW should return an error")
	require.NotNil(t, res)
	require.GreaterOrEqual(t, len(*res), 3)

	assert.Equal(t, "OK", (*res)[0].Status)
	assert.NotNil(t, (*res)[1].Error)
	assert.Equal(t, surrealdb.ErrorKindThrown, (*res)[1].Error.Kind())
	assert.Equal(t, "OK", (*res)[2].Status)
}
