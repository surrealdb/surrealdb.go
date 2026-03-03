// structured error integration tests for SurrealDB v3.
//
// These tests run against a live SurrealDB v3 server and verify the SDK's
// error handling architecture:
//
// Error type hierarchy:
//
//	RPCError  (v2 backward compat — Code, Message, Description)
//	   └── Unwrap() → ServerError  (v3 rich info — Kind, Details, Cause chain)
//
//	QueryError  (per-statement errors from Query results — Message only)
//
// When should you use which?
//
//   - RPCError is kept ONLY for v2 backward compatibility. On SurrealDB v3,
//     RPCError still works via errors.As/Is, but it carries fewer fields than
//     ServerError (no Kind, no Details, no Cause chain). The Description field
//     is always empty on v3 servers.
//
//   - ServerError is the v3 replacement. Extract it with errors.As(err, &se)
//     from any *RPCError. It provides Kind (e.g. "NotAllowed", "Validation"),
//     Details (structured info like table names, record IDs), and a Cause chain.
//
//   - QueryError is returned for per-statement failures in multi-statement
//     queries (e.g. THROW, duplicate records). It contains only Message.
//     Query-level parse errors on v3 are RPC-level errors (*RPCError), not
//     QueryError, because v3 rejects the entire RPC call.
package testenv

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

// TestStructuredErrors_InvalidCredentials demonstrates that RPC-level auth
// failures are returned as *RPCError, but *ServerError (via Unwrap) carries
// strictly more information: Kind, and Details with auth-specific context.
//
// RPCError gives you: Code (-32002), Message, Description (empty on v3).
// ServerError gives you: Code, Message, Kind ("NotAllowed"), Details ({Auth: InvalidAuth}), Cause (nil).
func TestStructuredErrors_InvalidCredentials(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := db.SignIn(context.Background(), surrealdb.Auth{
		Username: "invalid",
		Password: "invalid",
	})

	require.Error(t, err)

	// --- v2 backward compat: RPCError still works, but has less information ---
	// RPCError exposes only Code, Message, and Description (v2-only, empty on v3).
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	var rpcErr *surrealdb.RPCError
	require.True(t, errors.As(err, &rpcErr), "RPCError should be extractable (v2 compat)")
	assert.Equal(t, -32002, rpcErr.Code)
	assert.Equal(t, "There was a problem with authentication", rpcErr.Message)
	// On v3, Description is always empty — it's a v2-only field.
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.Empty(t, rpcErr.Description, "Description is a v2-only field, empty on v3")

	// --- v3 migration path: ServerError has strictly more information ---
	// ServerError exposes all five public fields: Message, Kind, Code, Details, Cause.
	var se surrealdb.ServerError
	require.True(t, errors.As(err, &se), "ServerError should be extractable via Unwrap chain")

	// Message and Code: same values as RPCError.
	assert.Equal(t, rpcErr.Code, se.Code, "ServerError.Code == RPCError.Code")
	assert.Equal(t, rpcErr.Message, se.Message, "ServerError.Message == RPCError.Message")

	// Kind: "NotAllowed" — categorizes the error without string parsing.
	// RPCError does NOT expose Kind.
	assert.Equal(t, "NotAllowed", se.Kind)

	// Details: auth-specific structured info ({kind: "Auth", details: {kind: "InvalidAuth"}}).
	// RPCError does NOT expose Details.
	assert.Equal(t, map[string]any{
		"kind":    "Auth",
		"details": map[string]any{"kind": "InvalidAuth"},
	}, se.Details)

	// Cause: nil for this error — no nested cause chain.
	assert.Nil(t, se.Cause, "no cause chain for auth errors")
}

// TestStructuredErrors_InvalidSyntax demonstrates that on SurrealDB v3,
// parse errors are RPC-level failures (*RPCError → *ServerError), NOT
// per-statement *QueryError. The server rejects the entire RPC call because
// the SurrealQL cannot be parsed.
//
// This is different from v2 behavior where parse errors might be per-statement.
func TestStructuredErrors_InvalidSyntax(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := surrealdb.Query[any](context.Background(), db, "SEL ECT * FORM person", nil) //nolint:misspell // intentionally malformed SurrealQL

	require.Error(t, err)

	// On v3, parse errors are RPC-level: *RPCError wrapping *ServerError.
	// They are NOT *QueryError because the server rejects the entire call.
	var qe *surrealdb.QueryError
	assert.False(t, errors.As(err, &qe), "parse errors on v3 are RPC-level, not QueryError")

	// --- v2 backward compat: RPCError works but gives minimal info ---
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	var rpcErr *surrealdb.RPCError
	require.True(t, errors.As(err, &rpcErr), "RPCError should be extractable")
	assert.Equal(t, "Parse error: Unexpected token `an identifier`, expected Eof\n --> [1:5]\n  |\n1 | SEL ECT * FORM person\n  |     ^^^\n", rpcErr.Message) //nolint:misspell // intentionally malformed SurrealQL
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.Empty(t, rpcErr.Description, "Description is a v2-only field, empty on v3")

	// --- v3 migration path: ServerError gives all five fields ---
	var se surrealdb.ServerError
	require.True(t, errors.As(err, &se), "ServerError should be extractable via Unwrap chain")

	// Message: the full parse error with source location, same as RPCError.Message.
	assert.Equal(t, rpcErr.Message, se.Message)

	// Kind: "Validation" — categorizes this as a validation/parse error.
	assert.Equal(t, "Validation", se.Kind)

	// Code: the RPC error code.
	assert.Equal(t, -32000, se.Code)

	// Details: nil for parse errors (the message itself is descriptive enough).
	assert.Nil(t, se.Details, "parse errors have no structured Details")

	// Cause: nil — no nested cause chain for parse errors.
	assert.Nil(t, se.Cause, "no cause chain for parse errors")
}

// TestStructuredErrors_UserThrow demonstrates per-statement errors from THROW.
// These come back as *QueryError (per-statement results), not RPC-level errors.
// QueryError intentionally has only Message — it does NOT unwrap to ServerError.
//
// This means per-statement errors have NO access to Kind, Details, or Cause.
// Only RPC-level errors (from Upsert, Create, etc.) carry *ServerError.
func TestStructuredErrors_UserThrow(t *testing.T) {
	db := setupStructuredErrorTest(t)

	res, err := surrealdb.Query[any](context.Background(), db, `THROW "custom user error"`, nil)

	require.Error(t, err)

	// Per-statement errors are *QueryError, which carries only Message.
	// QueryError does NOT unwrap to ServerError — it's a simpler error type.
	var qe *surrealdb.QueryError
	require.True(t, errors.As(err, &qe), "THROW produces a per-statement QueryError")
	assert.Equal(t, "An error occurred: custom user error", qe.Error())

	// QueryError is NOT a ServerError and NOT an RPCError.
	// This is the key limitation: per-statement errors lack Kind, Details, Cause.
	assert.False(t, errors.Is(err, &surrealdb.ServerError{}), "QueryError should not match ServerError")
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.False(t, errors.Is(err, &surrealdb.RPCError{}), "QueryError should not match RPCError")

	// The per-statement result also carries the error as *QueryError.
	require.Equal(t, 1, len(*res))
	assert.Equal(t, "ERR", (*res)[0].Status)
	assert.Equal(t, "An error occurred: custom user error", (*res)[0].Error.Message)
}

// TestStructuredErrors_DuplicateRecord demonstrates that CREATE on an existing
// record produces a per-statement *QueryError (same as THROW).
// Like THROW, per-statement errors have no ServerError and thus no Kind/Details/Cause.
func TestStructuredErrors_DuplicateRecord(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := surrealdb.Query[any](context.Background(), db,
		`CREATE person:dup SET name = "first"`, nil)
	require.NoError(t, err)

	res, err := surrealdb.Query[any](context.Background(), db,
		`CREATE person:dup SET name = "second"`, nil)

	require.Error(t, err)
	assert.Equal(t, "Database record `person:dup` already exists", err.Error())

	// Per-statement error — same as THROW: QueryError only, no ServerError.
	var qe *surrealdb.QueryError
	assert.True(t, errors.As(err, &qe), "duplicate record is a per-statement QueryError")
	assert.Equal(t, "Database record `person:dup` already exists", qe.Message)

	// No ServerError or RPCError available for per-statement errors.
	assert.False(t, errors.Is(err, &surrealdb.ServerError{}), "QueryError does not carry ServerError")
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.False(t, errors.Is(err, &surrealdb.RPCError{}), "QueryError is not RPCError")

	// The per-statement result also carries the error as *QueryError.
	require.Equal(t, 1, len(*res))
	assert.Equal(t, "ERR", (*res)[0].Status)
	assert.Equal(t, "Database record `person:dup` already exists", (*res)[0].Error.Message)
}

// TestStructuredErrors_SchemaViolation_RPC demonstrates that Upsert (an RPC
// method) returns *RPCError when the schema is violated. This test shows the
// key difference: RPCError has 3 fields, ServerError has 5.
//
//	RPCError:    Code + Message + Description(empty on v3)
//	ServerError: Code + Message + Kind + Details + Cause
func TestStructuredErrors_SchemaViolation_RPC(t *testing.T) {
	db := setupStructuredErrorTest(t)

	_, err := surrealdb.Query[any](context.Background(), db,
		`DEFINE TABLE person SCHEMAFUL;
		 DEFINE FIELD age ON person TYPE int;`, nil)
	require.NoError(t, err)

	// Upsert via RPC returns *RPCError on failure.
	_, err = surrealdb.Upsert[map[string]any](context.Background(), db,
		models.NewRecordID("person", "schema_test"),
		map[string]any{"age": "not a number"},
	)

	require.Error(t, err)

	// --- v2 backward compat: RPCError accessible with limited info ---
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	var rpcErr *surrealdb.RPCError
	require.True(t, errors.As(err, &rpcErr), "RPCError should be extractable (v2 compat)")
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.True(t, errors.Is(err, &surrealdb.RPCError{}))
	assert.Equal(t, -32000, rpcErr.Code)
	assert.Equal(t, "Couldn't coerce value for field `age` of `person:schema_test`: Expected `int` but found `'not a number'`", rpcErr.Message)
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.Empty(t, rpcErr.Description, "Description is a v2-only field, empty on v3")

	// --- v3 migration path: ServerError has strictly more information ---
	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se), "ServerError should be extractable via Unwrap chain")

	// Message: describes the coercion failure, same as RPCError.Message.
	assert.Equal(t, rpcErr.Message, se.Message)

	// Kind: "Internal" — RPCError does NOT expose this.
	assert.Equal(t, "Internal", se.Kind)

	// Code: same as RPCError.Code.
	assert.Equal(t, rpcErr.Code, se.Code)
	assert.Equal(t, -32000, se.Code)

	// Details: nil for schema coercion errors.
	assert.Nil(t, se.Details, "schema coercion errors have no structured Details")

	// Cause: nil — no nested cause chain for this error.
	assert.Nil(t, se.Cause, "no cause chain for schema coercion errors")
}

// TestStructuredErrors_AlreadyExists_RPC demonstrates Kind="AlreadyExists"
// paired with surrealdb.IsAlreadyExists. Triggered by creating the same record
// twice via the RPC Create method (not Query).
//
// This is distinct from the DuplicateRecord test above, which uses Query and
// gets a per-statement *QueryError. Here, Create is an RPC method, so the
// error is *RPCError wrapping *ServerError with full structured info.
func TestStructuredErrors_AlreadyExists_RPC(t *testing.T) {
	db := setupStructuredErrorTest(t)

	// First Create succeeds.
	_, err := surrealdb.Create[map[string]any](context.Background(), db,
		models.NewRecordID("person", "already_exists_test"),
		map[string]any{"name": "first"},
	)
	require.NoError(t, err)

	// Second Create with the same record ID fails at RPC level.
	_, err = surrealdb.Create[map[string]any](context.Background(), db,
		models.NewRecordID("person", "already_exists_test"),
		map[string]any{"name": "second"},
	)
	require.Error(t, err)

	// --- v2 backward compat: RPCError ---
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	var rpcErr *surrealdb.RPCError
	require.True(t, errors.As(err, &rpcErr), "RPCError should be extractable (v2 compat)")
	assert.Equal(t, -32000, rpcErr.Code)
	assert.Equal(t, "Database record `person:already_exists_test` already exists", rpcErr.Message)
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.Empty(t, rpcErr.Description, "Description is a v2-only field, empty on v3")

	// --- v3 migration path: ServerError ---
	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se), "ServerError should be extractable via Unwrap chain")

	// Message and Code: same as RPCError.
	assert.Equal(t, rpcErr.Message, se.Message)
	assert.Equal(t, rpcErr.Code, se.Code)

	// Kind: "AlreadyExists" — paired with surrealdb.IsAlreadyExists helper.
	assert.Equal(t, "AlreadyExists", se.Kind)

	// Details: record-specific structured info ({kind: "Record", details: {id: "person:already_exists_test"}}).
	assert.Equal(t, map[string]any{
		"kind":    "Record",
		"details": map[string]any{"id": "person:already_exists_test"},
	}, se.Details)

	// Cause: nil — no nested cause chain.
	assert.Nil(t, se.Cause)
}

// TestStructuredErrors_NotFound_RPC demonstrates Kind="NotFound" paired with
// surrealdb.IsNotFound. Triggered by RPC Select on a table that does not exist.
//
// The ServerError.Details carries the table name via the TableName() accessor,
// showing structured info that RPCError does not provide.
func TestStructuredErrors_NotFound_RPC(t *testing.T) {
	db := setupStructuredErrorTest(t)

	// Select from a table that was never created.
	_, err := surrealdb.Select[map[string]any](context.Background(), db,
		models.Table("nonexistent_table_for_test"),
	)
	require.Error(t, err)

	// --- v2 backward compat: RPCError ---
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	var rpcErr *surrealdb.RPCError
	require.True(t, errors.As(err, &rpcErr), "RPCError should be extractable (v2 compat)")
	assert.Equal(t, -32000, rpcErr.Code)
	assert.Equal(t, "The table 'nonexistent_table_for_test' does not exist", rpcErr.Message)
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.Empty(t, rpcErr.Description, "Description is a v2-only field, empty on v3")

	// --- v3 migration path: ServerError ---
	var se *surrealdb.ServerError
	require.True(t, errors.As(err, &se), "ServerError should be extractable via Unwrap chain")

	// Message and Code: same as RPCError.
	assert.Equal(t, rpcErr.Message, se.Message)
	assert.Equal(t, rpcErr.Code, se.Code)

	// Kind: "NotFound" — paired with surrealdb.IsNotFound helper.
	assert.Equal(t, "NotFound", se.Kind)

	// Details: table-specific structured info ({kind: "Table", details: {name: "nonexistent_table_for_test"}}).
	assert.Equal(t, map[string]any{
		"kind":    "Table",
		"details": map[string]any{"name": "nonexistent_table_for_test"},
	}, se.Details)

	// Cause: nil — no nested cause chain.
	assert.Nil(t, se.Cause)
}

// TestStructuredErrors_MultiStatementMixed demonstrates that multi-statement
// queries return per-statement results. Successful statements have Status "OK",
// while failed ones (e.g. THROW) have *QueryError in their Error field.
//
// The joined error from Query is a *QueryError — no ServerError available.
func TestStructuredErrors_MultiStatementMixed(t *testing.T) {
	db := setupStructuredErrorTest(t)

	res, err := surrealdb.Query[any](context.Background(), db,
		`RETURN 1; THROW "fail"; RETURN 3`, nil)

	require.Error(t, err, "multi-statement with THROW should return an error")
	require.Equal(t, 3, len(*res))

	// Statement 0: RETURN 1 — succeeds, no error.
	assert.Equal(t, "OK", (*res)[0].Status)
	assert.Equal(t, "", (*res)[0].Error.Error(), "successful statement has nil-safe empty Error()")

	// Statement 1: THROW "fail" — per-statement *QueryError with exact message.
	assert.Equal(t, "ERR", (*res)[1].Status)
	assert.Equal(t, "An error occurred: fail", (*res)[1].Error.Message)

	// Statement 2: RETURN 3 — succeeds, no error.
	assert.Equal(t, "OK", (*res)[2].Status)
	assert.Equal(t, "", (*res)[2].Error.Error(), "successful statement has nil-safe empty Error()")

	// The joined error is a *QueryError (per-statement), not RPCError.
	var qe *surrealdb.QueryError
	assert.True(t, errors.As(err, &qe))
	assert.Equal(t, "An error occurred: fail", qe.Message)
	//nolint:staticcheck // intentionally testing v2 backward compat with RPCError
	assert.False(t, errors.Is(err, &surrealdb.RPCError{}), "per-statement errors are not RPCError")
	assert.False(t, errors.Is(err, &surrealdb.ServerError{}), "per-statement errors have no ServerError")
}
