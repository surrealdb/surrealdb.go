package surrealdb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// ================================================================= //
//  V3 format: { "kind": "...", "details": ... } (internally-tagged) //
// ================================================================= //

func TestV3_NotAllowed_TokenExpired(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32002,
		Kind:    "NotAllowed",
		Message: "Token has expired",
		Details: map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
	})

	assert.Equal(t, "NotAllowed", err.Kind())
	assert.Equal(t, -32002, err.Code())
	assert.Equal(t, "Token has expired", err.Error())
	assert.Nil(t, err.ServerCause())

	assert.True(t, err.IsTokenExpired())
	assert.False(t, err.IsInvalidAuth())
	assert.False(t, err.IsScriptingBlocked())
	assert.Equal(t, "", err.MethodName())
	assert.Equal(t, "", err.FunctionName())
}

func TestV3_NotAllowed_InvalidAuth(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32002,
		Kind:    "NotAllowed",
		Message: "There was a problem with authentication",
		Details: map[string]any{"kind": "Auth", "details": map[string]any{"kind": "InvalidAuth"}},
	})

	assert.True(t, err.IsInvalidAuth())
	assert.False(t, err.IsTokenExpired())
}

func TestV3_NotAllowed_Method(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Method not allowed",
		Details: map[string]any{"kind": "Method", "details": map[string]any{"name": "begin"}},
	})

	assert.Equal(t, "begin", err.MethodName())
	assert.False(t, err.IsTokenExpired())
}

func TestV3_NotAllowed_Scripting(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Scripting is blocked",
		Details: map[string]any{"kind": "Scripting"},
	})

	assert.True(t, err.IsScriptingBlocked())
}

func TestV3_NotAllowed_Function(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Function not allowed",
		Details: map[string]any{"kind": "Function", "details": map[string]any{"name": "fn::custom"}},
	})

	assert.Equal(t, "fn::custom", err.FunctionName())
}

func TestV3_NotAllowed_Target(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Target not allowed",
		Details: map[string]any{"kind": "Target", "details": map[string]any{"name": "some_target"}},
	})

	assert.Equal(t, "some_target", err.TargetName())
}

func TestV3_NotFound_Table(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Table not found",
		Details: map[string]any{"kind": "Table", "details": map[string]any{"name": "users"}},
	})

	assert.Equal(t, "NotFound", err.Kind())
	assert.Equal(t, "users", err.TableName())
	assert.Equal(t, "", err.RecordID())
	assert.Equal(t, "", err.MethodName())
}

func TestV3_NotFound_Record(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Record not found",
		Details: map[string]any{"kind": "Record", "details": map[string]any{"id": "users:123"}},
	})

	assert.Equal(t, "users:123", err.RecordID())
	assert.Equal(t, "", err.TableName())
}

func TestV3_NotFound_Method(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32601,
		Kind:    "NotFound",
		Message: "Method not found",
		Details: map[string]any{"kind": "Method", "details": map[string]any{"name": "unknown_method"}},
	})

	assert.Equal(t, "unknown_method", err.MethodName())
}

func TestV3_NotFound_Namespace(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Namespace not found",
		Details: map[string]any{"kind": "Namespace", "details": map[string]any{"name": "test"}},
	})

	assert.Equal(t, "test", err.NamespaceName())
}

func TestV3_NotFound_Database(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Database not found",
		Details: map[string]any{"kind": "Database", "details": map[string]any{"name": "test"}},
	})

	assert.Equal(t, "test", err.DatabaseName())
}

func TestV3_NotFound_Session(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Session not found",
		Details: map[string]any{"kind": "Session", "details": map[string]any{"id": "abc-123"}},
	})

	assert.Equal(t, "NotFound", err.Kind())
	assert.Equal(t, "abc-123", err.SessionID())
}

func TestV3_AlreadyExists_Record(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Database record `person:dup` already exists",
		Details: map[string]any{"kind": "Record", "details": map[string]any{"id": "person:dup"}},
	})

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "person:dup", err.RecordID())
	assert.Equal(t, "", err.TableName())
}

func TestV3_AlreadyExists_Table(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Table already exists",
		Details: map[string]any{"kind": "Table", "details": map[string]any{"name": "users"}},
	})

	assert.Equal(t, "users", err.TableName())
}

func TestV3_Validation_Parse(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32700,
		Kind:    "Validation",
		Message: "Parse error",
		Details: map[string]any{"kind": "Parse"},
	})

	assert.Equal(t, "Validation", err.Kind())
	assert.True(t, err.IsParseError())
	assert.Equal(t, "", err.ParameterName())
}

func TestV3_Validation_InvalidParameter(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32603,
		Kind:    "Validation",
		Message: "Invalid parameter",
		Details: map[string]any{"kind": "InvalidParameter", "details": map[string]any{"name": "limit"}},
	})

	assert.Equal(t, "limit", err.ParameterName())
	assert.False(t, err.IsParseError())
}

func TestV3_Query_NotExecuted(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32003,
		Kind:    "Query",
		Message: "Query not executed",
		Details: map[string]any{"kind": "NotExecuted"},
	})

	assert.Equal(t, "Query", err.Kind())
	assert.True(t, err.IsNotExecuted())
	assert.False(t, err.IsTimedOut())
	assert.False(t, err.IsCancelled())
	_, _, ok := err.Timeout()
	assert.False(t, ok)
}

func TestV3_Query_TimedOut(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32004,
		Kind:    "Query",
		Message: "Query timed out",
		Details: map[string]any{
			"kind": "TimedOut",
			"details": map[string]any{
				"duration": map[string]any{"secs": 5, "nanos": 0},
			},
		},
	})

	assert.True(t, err.IsTimedOut())
	secs, nanos, ok := err.Timeout()
	assert.True(t, ok)
	assert.Equal(t, 5, secs)
	assert.Equal(t, 0, nanos)
}

func TestV3_Query_Cancelled(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32005,
		Kind:    "Query",
		Message: "Query cancelled",
		Details: map[string]any{"kind": "Cancelled"},
	})

	assert.True(t, err.IsCancelled())
}

func TestV3_Configuration_LiveQuery(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32604,
		Kind:    "Configuration",
		Message: "Live queries not supported",
		Details: map[string]any{"kind": "LiveQueryNotSupported"},
	})

	assert.Equal(t, "Configuration", err.Kind())
	assert.True(t, err.IsLiveQueryNotSupported())
}

func TestV3_Serialization_Deserialization(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32008,
		Kind:    "Serialization",
		Message: "Deserialization failed",
		Details: map[string]any{"kind": "Deserialization"},
	})

	assert.Equal(t, "Serialization", err.Kind())
	assert.True(t, err.IsDeserialization())
}

func TestV3_Thrown(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32006,
		Kind:    "Thrown",
		Message: "Custom user error",
	})

	assert.Equal(t, "Thrown", err.Kind())
	assert.Equal(t, "Custom user error", err.Error())
}

func TestV3_Internal(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "Internal",
		Message: "Something went wrong",
	})

	assert.Equal(t, "Internal", err.Kind())
}

func TestV3_NoDetails(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Not found",
	})

	assert.Equal(t, "NotFound", err.Kind())
	assert.Nil(t, err.Details())
	assert.Equal(t, "", err.TableName())
	assert.Equal(t, "", err.RecordID())
}

func TestV3_NilDetails(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "Internal",
		Message: "Error",
		Details: nil,
	})

	assert.Nil(t, err.Details())
}

// ================================================================= //
//  V3 format: query result errors                                    //
// ================================================================= //

func TestV3_QueryError_NotFound(t *testing.T) {
	err := parseQueryError(
		"Table not found",
		"NotFound",
		map[string]any{"kind": "Table", "details": map[string]any{"name": "users"}},
		nil,
	)

	assert.Equal(t, "NotFound", err.Kind())
	assert.Equal(t, 0, err.Code())
	assert.Equal(t, "Table not found", err.Error())
	assert.Equal(t, "users", err.TableName())
}

func TestV3_QueryError_AlreadyExists(t *testing.T) {
	err := parseQueryError(
		"Database record `person:dup` already exists",
		"AlreadyExists",
		map[string]any{"kind": "Record", "details": map[string]any{"id": "person:dup"}},
		nil,
	)

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "person:dup", err.RecordID())
}

func TestV3_QueryError_Thrown(t *testing.T) {
	err := parseQueryError(
		"An error occurred: custom user error",
		"Thrown",
		nil,
		nil,
	)

	assert.Equal(t, "Thrown", err.Kind())
	assert.Equal(t, 0, err.Code())
	assert.Equal(t, "An error occurred: custom user error", err.Error())
	assert.Nil(t, err.Details())
}

func TestV3_QueryError_OldFormat_MessageOnly(t *testing.T) {
	err := parseQueryError(
		"There was a problem with the database: Table not found",
		"",
		nil,
		nil,
	)

	assert.Equal(t, "Internal", err.Kind())
	assert.Equal(t, 0, err.Code())
	assert.Equal(t, "There was a problem with the database: Table not found", err.Error())
	assert.Nil(t, err.Details())
}

func TestV3_QueryError_WithCause(t *testing.T) {
	err := parseQueryError(
		"Permission denied",
		"NotAllowed",
		map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
		&connection.RPCError{
			Code:    -32000,
			Kind:    "Internal",
			Message: "Session expired",
		},
	)

	assert.Equal(t, "NotAllowed", err.Kind())
	assert.True(t, err.IsTokenExpired())
	assert.Equal(t, "Permission denied: Session expired", err.Error())
	assert.Equal(t, "Permission denied", err.Message())

	cause := err.ServerCause()
	require.NotNil(t, cause)
	assert.Equal(t, "Internal", cause.Kind())
	assert.Equal(t, "Session expired", cause.Error())
	assert.Equal(t, "Session expired", cause.Message())
}

// ================================================================= //
//  V3 format: double-wrapped details unwrapping                      //
// ================================================================= //

func TestV3_QueryError_DoubleWrappedDetails(t *testing.T) {
	err := parseQueryError(
		"Record already exists",
		"AlreadyExists",
		map[string]any{
			"kind": "AlreadyExists",
			"details": map[string]any{
				"kind":    "Record",
				"details": map[string]any{"id": "person:dup"},
			},
		},
		nil,
	)

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "person:dup", err.RecordID())
}

func TestV3_QueryError_DoubleWrappedDetails_NotFound(t *testing.T) {
	err := parseQueryError(
		"Table not found",
		"NotFound",
		map[string]any{
			"kind": "NotFound",
			"details": map[string]any{
				"kind":    "Table",
				"details": map[string]any{"name": "users"},
			},
		},
		nil,
	)

	assert.Equal(t, "NotFound", err.Kind())
	assert.Equal(t, "users", err.TableName())
}

func TestV3_QueryError_DoubleWrappedDetails_KindMismatchNotUnwrapped(t *testing.T) {
	details := map[string]any{
		"kind": "NotFound",
		"details": map[string]any{
			"kind":    "Table",
			"details": map[string]any{"name": "users"},
		},
	}

	err := parseQueryError("Error", "AlreadyExists", details, nil)

	assert.Equal(t, "AlreadyExists", err.Kind())
	// Should NOT unwrap because outer kind "NotFound" != error kind "AlreadyExists"
	assert.Equal(t, "", err.TableName())
}

// ================================================================= //
//  Old format backward compatibility (externally-tagged)             //
// ================================================================= //

func TestV3_AlreadyExists_Namespace(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Namespace already exists",
		Details: map[string]any{"kind": "Namespace", "details": map[string]any{"name": "test_ns"}},
	})

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "test_ns", err.NamespaceName())
}

func TestV3_AlreadyExists_Database(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Database already exists",
		Details: map[string]any{"kind": "Database", "details": map[string]any{"name": "test_db"}},
	})

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "test_db", err.DatabaseName())
}

func TestV3_AlreadyExists_Session(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Session already exists",
		Details: map[string]any{"kind": "Session", "details": map[string]any{"id": "sess-456"}},
	})

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "sess-456", err.SessionID())
}

func TestOldFormat_NotAllowed_TokenExpired(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32002,
		Kind:    "NotAllowed",
		Message: "Token has expired",
		Details: map[string]any{"Auth": "TokenExpired"},
	})

	assert.True(t, err.IsTokenExpired())
	assert.False(t, err.IsInvalidAuth())
}

func TestOldFormat_NotAllowed_InvalidAuth(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32002,
		Kind:    "NotAllowed",
		Message: "Invalid credentials",
		Details: map[string]any{"Auth": "InvalidAuth"},
	})

	assert.True(t, err.IsInvalidAuth())
	assert.False(t, err.IsTokenExpired())
}

func TestOldFormat_NotAllowed_Method(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Method not allowed",
		Details: map[string]any{"Method": map[string]any{"name": "begin"}},
	})

	assert.Equal(t, "begin", err.MethodName())
}

func TestOldFormat_NotAllowed_Scripting(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Scripting is blocked",
		Details: map[string]any{"Scripting": map[string]any{}},
	})

	assert.True(t, err.IsScriptingBlocked())
}

func TestOldFormat_NotAllowed_Function(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Function not allowed",
		Details: map[string]any{"Function": map[string]any{"name": "fn::custom"}},
	})

	assert.Equal(t, "fn::custom", err.FunctionName())
}

func TestOldFormat_NotAllowed_Target(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32602,
		Kind:    "NotAllowed",
		Message: "Target not allowed",
		Details: map[string]any{"Target": map[string]any{"name": "some_target"}},
	})

	assert.Equal(t, "some_target", err.TargetName())
}

func TestOldFormat_NotFound_Table(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Table not found",
		Details: map[string]any{"Table": map[string]any{"name": "users"}},
	})

	assert.Equal(t, "users", err.TableName())
	assert.Equal(t, "", err.RecordID())
}

func TestOldFormat_NotFound_Record(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Record not found",
		Details: map[string]any{"Record": map[string]any{"id": "users:123"}},
	})

	assert.Equal(t, "users:123", err.RecordID())
}

func TestOldFormat_NotFound_Namespace(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Namespace not found",
		Details: map[string]any{"Namespace": map[string]any{"name": "test"}},
	})

	assert.Equal(t, "test", err.NamespaceName())
}

func TestOldFormat_NotFound_Database(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Database not found",
		Details: map[string]any{"Database": map[string]any{"name": "test"}},
	})

	assert.Equal(t, "test", err.DatabaseName())
}

func TestOldFormat_NotFound_Session(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "NotFound",
		Message: "Session not found",
		Details: map[string]any{"Session": map[string]any{"id": "abc-123"}},
	})

	assert.Equal(t, "abc-123", err.SessionID())
}

func TestOldFormat_AlreadyExists_Record(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Record already exists",
		Details: map[string]any{"Record": map[string]any{"id": "users:123"}},
	})

	assert.Equal(t, "AlreadyExists", err.Kind())
	assert.Equal(t, "users:123", err.RecordID())
	assert.Equal(t, "", err.TableName())
}

func TestOldFormat_AlreadyExists_Table(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "AlreadyExists",
		Message: "Table already exists",
		Details: map[string]any{"Table": map[string]any{"name": "users"}},
	})

	assert.Equal(t, "users", err.TableName())
}

func TestOldFormat_Validation_Parse(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32700,
		Kind:    "Validation",
		Message: "Parse error",
		Details: "Parse",
	})

	assert.Equal(t, "Validation", err.Kind())
	assert.True(t, err.IsParseError())
	assert.Equal(t, "", err.ParameterName())
}

func TestOldFormat_Validation_InvalidParameter(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32603,
		Kind:    "Validation",
		Message: "Invalid parameter",
		Details: map[string]any{"InvalidParameter": map[string]any{"name": "limit"}},
	})

	assert.Equal(t, "limit", err.ParameterName())
	assert.False(t, err.IsParseError())
}

func TestOldFormat_Query_NotExecuted(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32003,
		Kind:    "Query",
		Message: "Query not executed",
		Details: map[string]any{"NotExecuted": map[string]any{}},
	})

	assert.Equal(t, "Query", err.Kind())
	assert.True(t, err.IsNotExecuted())
	assert.False(t, err.IsTimedOut())
	assert.False(t, err.IsCancelled())
}

func TestOldFormat_Query_TimedOut(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32004,
		Kind:    "Query",
		Message: "Query timed out",
		Details: map[string]any{
			"TimedOut": map[string]any{
				"duration": map[string]any{"secs": 5, "nanos": 0},
			},
		},
	})

	assert.True(t, err.IsTimedOut())
	secs, nanos, ok := err.Timeout()
	assert.True(t, ok)
	assert.Equal(t, 5, secs)
	assert.Equal(t, 0, nanos)
}

func TestOldFormat_Query_Cancelled(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32005,
		Kind:    "Query",
		Message: "Query cancelled",
		Details: map[string]any{"Cancelled": map[string]any{}},
	})

	assert.True(t, err.IsCancelled())
}

func TestOldFormat_Configuration_LiveQuery(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32604,
		Kind:    "Configuration",
		Message: "Live queries not supported",
		Details: map[string]any{"LiveQueryNotSupported": map[string]any{}},
	})

	assert.Equal(t, "Configuration", err.Kind())
	assert.True(t, err.IsLiveQueryNotSupported())
}

func TestOldFormat_Serialization_Deserialization(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32008,
		Kind:    "Serialization",
		Message: "Deserialization failed",
		Details: map[string]any{"Deserialization": map[string]any{}},
	})

	assert.Equal(t, "Serialization", err.Kind())
	assert.True(t, err.IsDeserialization())
}

func TestOldFormat_QueryError(t *testing.T) {
	err := parseQueryError(
		"Table not found",
		"NotFound",
		map[string]any{"Table": map[string]any{"name": "users"}},
		nil,
	)

	assert.Equal(t, "NotFound", err.Kind())
	assert.Equal(t, "users", err.TableName())
}

func TestOldFormat_QueryError_WithCause(t *testing.T) {
	err := parseQueryError(
		"Permission denied",
		"NotAllowed",
		map[string]any{"Auth": "TokenExpired"},
		&connection.RPCError{
			Code:    -32000,
			Kind:    "Internal",
			Message: "Session expired",
		},
	)

	assert.Equal(t, "NotAllowed", err.Kind())
	assert.True(t, err.IsTokenExpired())

	cause := err.ServerCause()
	require.NotNil(t, cause)
	assert.Equal(t, "Internal", cause.Kind())
}

// ================================================================= //
//  Legacy code-to-kind mapping (no kind field, derive from code)     //
// ================================================================= //

func TestLegacy_CodeToKindMapping(t *testing.T) {
	tests := []struct {
		code         int
		expectedKind string
	}{
		{-32700, "Validation"},
		{-32600, "Validation"},
		{-32603, "Validation"},
		{-32601, "NotFound"},
		{-32602, "NotAllowed"},
		{-32002, "NotAllowed"},
		{-32604, "Configuration"},
		{-32605, "Configuration"},
		{-32606, "Configuration"},
		{-32000, "Internal"},
		{-32001, "Connection"},
		{-32003, "Query"},
		{-32004, "Query"},
		{-32005, "Query"},
		{-32006, "Thrown"},
		{-32007, "Serialization"},
		{-32008, "Serialization"},
	}

	for _, tt := range tests {
		err := parseRpcError(&connection.RPCError{Code: tt.code, Message: "test"})
		assert.Equal(t, tt.expectedKind, err.Kind())
	}
}

func TestLegacy_UnknownCodeMapsToInternal(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -99999, Message: "Unknown"})
	assert.Equal(t, "Internal", err.Kind())
}

func TestLegacy_PreservesCodeAndMessage(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32002, Message: "Invalid credentials"})

	assert.Equal(t, -32002, err.Code())
	assert.Equal(t, "Invalid credentials", err.Error())
	assert.Nil(t, err.Details())
	assert.Nil(t, err.ServerCause())
}

// ================================================================= //
//  Cause chain traversal                                             //
// ================================================================= //

func TestCauseChain_DeepParsedRecursively(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code: -32000, Kind: "NotAllowed", Message: "Top level",
		Cause: &connection.RPCError{
			Code: -32000, Kind: "NotFound", Message: "Middle",
			Cause: &connection.RPCError{
				Code: -32000, Kind: "Internal", Message: "Root cause",
			},
		},
	})

	assert.Equal(t, "NotAllowed", err.Kind())
	assert.Equal(t, "Top level: Middle: Root cause", err.Error())
	assert.Equal(t, "Top level", err.Message())

	middle := err.ServerCause()
	require.NotNil(t, middle)
	assert.Equal(t, "NotFound", middle.Kind())
	assert.Equal(t, "Middle: Root cause", middle.Error())
	assert.Equal(t, "Middle", middle.Message())

	root := middle.ServerCause()
	require.NotNil(t, root)
	assert.Equal(t, "Internal", root.Kind())
	assert.Equal(t, "Root cause", root.Error())
	assert.Equal(t, "Root cause", root.Message())
	assert.Nil(t, root.ServerCause())
}

func TestCauseChain_HasKindTraversesChain(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code: -32000, Kind: "NotAllowed", Message: "Top",
		Cause: &connection.RPCError{
			Code: -32000, Kind: "NotFound", Message: "Nested",
		},
	})

	assert.True(t, err.HasKind("NotAllowed"))
	assert.True(t, err.HasKind("NotFound"))
	assert.False(t, err.HasKind("Internal"))
}

func TestCauseChain_FindCauseReturnsMatchingError(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code: -32000, Kind: "NotAllowed", Message: "Top",
		Cause: &connection.RPCError{
			Code: -32000, Kind: "NotFound", Message: "Nested not found",
			Details: map[string]any{"kind": "Table", "details": map[string]any{"name": "users"}},
		},
	})

	found := err.FindCause("NotFound")
	require.NotNil(t, found)
	assert.Equal(t, "NotFound", found.Kind())
	assert.Equal(t, "Nested not found", found.Error())
	assert.Equal(t, "users", found.TableName())
}

func TestCauseChain_FindCauseReturnsSelfIfMatch(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "NotFound", Message: "Self"})
	assert.Equal(t, err, err.FindCause("NotFound"))
}

func TestCauseChain_FindCauseReturnsNilWhenNotFound(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "NotFound", Message: "No match"})
	assert.Nil(t, err.FindCause("AlreadyExists"))
}

func TestCauseChain_UnwrapTraversal(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code: -32000, Kind: "NotAllowed", Message: "Top",
		Cause: &connection.RPCError{Code: -32000, Kind: "Internal", Message: "Bottom"},
	})

	unwrapped := errors.Unwrap(err)
	require.NotNil(t, unwrapped)

	var se *ServerError
	require.True(t, errors.As(unwrapped, &se))
	assert.Equal(t, "Internal", se.Kind())
}

// ================================================================= //
//  Forward compatibility: unknown kinds                              //
// ================================================================= //

func TestUnknownKinds_CreatesBaseServerError(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code:    -32000,
		Kind:    "FutureErrorKind",
		Message: "Some new error",
		Details: map[string]any{"kind": "SomeNewDetail", "details": map[string]any{"foo": "bar"}},
	})

	assert.Equal(t, "FutureErrorKind", err.Kind())
	assert.Equal(t, "Some new error", err.Error())
}

func TestUnknownKinds_DoesNotLoseInformation(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "BrandNew", Message: "Details preserved"})
	assert.Equal(t, "BrandNew", err.Kind())
}

// ================================================================= //
//  ErrorKind constants                                               //
// ================================================================= //

func TestErrorKindConstants(t *testing.T) {
	assert.Equal(t, "Validation", ErrorKindValidation)
	assert.Equal(t, "Configuration", ErrorKindConfiguration)
	assert.Equal(t, "Thrown", ErrorKindThrown)
	assert.Equal(t, "Query", ErrorKindQuery)
	assert.Equal(t, "Serialization", ErrorKindSerialization)
	assert.Equal(t, "NotAllowed", ErrorKindNotAllowed)
	assert.Equal(t, "NotFound", ErrorKindNotFound)
	assert.Equal(t, "AlreadyExists", ErrorKindAlreadyExists)
	assert.Equal(t, "Connection", ErrorKindConnection)
	assert.Equal(t, "Internal", ErrorKindInternal)
}

func TestErrorKindCanBeUsedForComparison(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "NotFound", Message: "Test"})
	assert.Equal(t, ErrorKindNotFound, err.Kind())
}

// ================================================================= //
//  ServerError properties                                            //
// ================================================================= //

func TestServerError_ImplementsError(t *testing.T) {
	var err error = &ServerError{kind: "Internal", message: "test"}
	assert.Equal(t, "test", err.Error())
}

func TestServerError_DefaultsCodeToZero(t *testing.T) {
	err := &ServerError{kind: "Internal", message: "test"}
	assert.Equal(t, 0, err.Code())
}

func TestServerError_DefaultsDetailsToNil(t *testing.T) {
	err := &ServerError{kind: "Internal", message: "test"}
	assert.Nil(t, err.Details())
}

func TestServerError_DefaultsCauseToNil(t *testing.T) {
	err := &ServerError{kind: "Internal", message: "test"}
	assert.Nil(t, err.ServerCause())
	assert.Nil(t, err.Unwrap())
}

func TestServerError_Is_MatchesAnyServerError(t *testing.T) {
	err := &ServerError{kind: "NotFound", message: "test"}
	assert.True(t, errors.Is(err, &ServerError{}))
}

func TestServerError_Is_MatchesSameKind(t *testing.T) {
	err := &ServerError{kind: "NotFound", message: "test"}
	assert.True(t, errors.Is(err, &ServerError{kind: "NotFound"}))
}

func TestServerError_Is_DoesNotMatchDifferentKind(t *testing.T) {
	err := &ServerError{kind: "NotFound", message: "test"}
	assert.False(t, errors.Is(err, &ServerError{kind: "NotAllowed"}))
}

func TestServerError_Is_DoesNotMatchOtherTypes(t *testing.T) {
	err := &ServerError{kind: "NotFound", message: "test"}
	assert.False(t, errors.Is(err, errors.New("test")))
}

func TestServerError_Message_WithoutCause(t *testing.T) {
	err := &ServerError{kind: "Internal", message: "test"}
	assert.Equal(t, "test", err.Message())
	assert.Equal(t, "test", err.Error())
}

func TestServerError_Message_WithCause(t *testing.T) {
	err := &ServerError{
		kind: "NotAllowed", message: "Top",
		cause: &ServerError{kind: "Internal", message: "Bottom"},
	}
	assert.Equal(t, "Top", err.Message())
	assert.Equal(t, "Top: Bottom", err.Error())
}

// ================================================================= //
//  Backward compatibility aliases                                    //
// ================================================================= //

func TestBackwardCompat_QueryErrorIsServerError(t *testing.T) {
	var qe *QueryError = &ServerError{kind: "Query", message: "test"}

	var se *ServerError
	assert.True(t, errors.As(qe, &se))
}

func TestBackwardCompat_ErrorsIs_QueryError(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32003, Kind: "Query", Message: "test"})
	assert.True(t, errors.Is(err, &QueryError{}))
}

// ================================================================= //
//  Top-level helper functions                                        //
// ================================================================= //

func TestHelpers_IsServerError(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "Internal", Message: "boom"})
	assert.True(t, IsServerError(err))
	assert.False(t, IsServerError(errors.New("not a server error")))
}

func TestHelpers_IsNotAllowed(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32002, Kind: "NotAllowed", Message: "Token expired"})
	assert.True(t, IsNotAllowed(err))
	assert.False(t, IsNotFound(err))
}

func TestHelpers_IsNotFound(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32601, Kind: "NotFound", Message: "Table not found"})
	assert.True(t, IsNotFound(err))
	assert.False(t, IsNotAllowed(err))
}

func TestHelpers_IsAlreadyExists(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "AlreadyExists", Message: "Record exists"})
	assert.True(t, IsAlreadyExists(err))
}

func TestHelpers_IsValidation(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32700, Kind: "Validation", Message: "Parse error"})
	assert.True(t, IsValidation(err))
}

func TestHelpers_IsConfiguration(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32604, Kind: "Configuration", Message: "Not supported"})
	assert.True(t, IsConfiguration(err))
}

func TestHelpers_IsThrown(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32006, Kind: "Thrown", Message: "User error"})
	assert.True(t, IsThrown(err))
}

func TestHelpers_IsQueryError(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32003, Kind: "Query", Message: "Query failed"})
	assert.True(t, IsQueryError(err))
}

func TestHelpers_IsSerialization(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32007, Kind: "Serialization", Message: "Serialization failed"})
	assert.True(t, IsSerialization(err))
}

func TestHelpers_IsConnectionError(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32001, Kind: "Connection", Message: "Connection failed"})
	assert.True(t, IsConnectionError(err))
}

func TestHelpers_IsInternal(t *testing.T) {
	err := parseRpcError(&connection.RPCError{Code: -32000, Kind: "Internal", Message: "Internal error"})
	assert.True(t, IsInternal(err))
}

func TestHelpers_HasKind(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code: -32000, Kind: "NotAllowed", Message: "Top",
		Cause: &connection.RPCError{Code: -32000, Kind: "NotFound", Message: "Nested"},
	})

	assert.True(t, HasKind(err, "NotAllowed"))
	assert.True(t, HasKind(err, "NotFound"))
	assert.False(t, HasKind(err, "Internal"))
	assert.False(t, HasKind(errors.New("not a server error"), "Internal"))
}

func TestHelpers_FindCause(t *testing.T) {
	err := parseRpcError(&connection.RPCError{
		Code: -32000, Kind: "NotAllowed", Message: "Top",
		Cause: &connection.RPCError{
			Code: -32000, Kind: "NotFound", Message: "Nested",
			Details: map[string]any{"kind": "Table", "details": map[string]any{"name": "users"}},
		},
	})

	found := FindCause(err, "NotFound")
	require.NotNil(t, found)
	assert.Equal(t, "users", found.TableName())

	assert.Nil(t, FindCause(err, "AlreadyExists"))
	assert.Nil(t, FindCause(errors.New("not a server error"), "Internal"))
}

// ================================================================= //
//  convertError                                                      //
// ================================================================= //

func TestConvertError_RPCError(t *testing.T) {
	rpcErr := &connection.RPCError{
		Code:    -32002,
		Kind:    "NotAllowed",
		Message: "Token expired",
		Details: map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
	}

	converted := convertError(rpcErr)

	var se *ServerError
	require.True(t, errors.As(converted, &se))
	assert.Equal(t, "NotAllowed", se.Kind())
	assert.True(t, se.IsTokenExpired())
}

func TestConvertError_NonRPCError_PassesThrough(t *testing.T) {
	original := errors.New("transport error")
	converted := convertError(original)
	assert.Equal(t, original, converted)
}

// ================================================================= //
//  Detail helpers: new format                                        //
// ================================================================= //

func TestDetailKind(t *testing.T) {
	assert.Equal(t, "Auth", detailKind(map[string]any{"kind": "Auth"}))
	assert.Equal(t, "Parse", detailKind(map[string]any{"kind": "Parse"}))
	assert.Equal(t, "", detailKind("Parse"))
	assert.Equal(t, "", detailKind(nil))
	assert.Equal(t, "", detailKind(map[string]any{"other": "value"}))
}

func TestDetailInner(t *testing.T) {
	inner := detailInner(map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}})
	assert.Equal(t, map[string]any{"kind": "TokenExpired"}, inner)

	assert.Nil(t, detailInner(map[string]any{"kind": "Parse"}))
	assert.Nil(t, detailInner("Parse"))
	assert.Nil(t, detailInner(nil))
}

func TestHasDetailKey_V3_UnitVariant(t *testing.T) {
	details := map[string]any{"kind": "Parse"}
	assert.True(t, hasDetailKey(details, "Parse"))
	assert.False(t, hasDetailKey(details, "Other"))
}

func TestHasDetailKey_V3_NewtypeVariant(t *testing.T) {
	details := map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}}
	assert.True(t, hasDetailKey(details, "Auth"))
	assert.False(t, hasDetailKey(details, "TokenExpired"))
}

func TestGetDetailValue_V3(t *testing.T) {
	details := map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}}
	inner := getDetailValue(details, "Auth")
	assert.Equal(t, map[string]any{"kind": "TokenExpired"}, inner)
	assert.Nil(t, getDetailValue(details, "Missing"))
}

func TestGetDetailString_V3(t *testing.T) {
	details := map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}}
	assert.Equal(t, "TokenExpired", getDetailString(details, "Auth"))
	assert.Equal(t, "", getDetailString(details, "Missing"))
}

func TestGetDetailMapString_V3(t *testing.T) {
	details := map[string]any{"kind": "Table", "details": map[string]any{"name": "users"}}
	assert.Equal(t, "users", getDetailMapString(details, "Table", "name"))
	assert.Equal(t, "", getDetailMapString(details, "Table", "missing"))
	assert.Equal(t, "", getDetailMapString(details, "Missing", "name"))
}

// ================================================================= //
//  Detail helpers: old format (backward compat)                      //
// ================================================================= //

func TestHasDetailKey_Old_String(t *testing.T) {
	assert.True(t, hasDetailKey("Parse", "Parse"))
	assert.False(t, hasDetailKey("Parse", "Other"))
}

func TestHasDetailKey_Old_Map(t *testing.T) {
	details := map[string]any{"Table": map[string]any{"name": "users"}}
	assert.True(t, hasDetailKey(details, "Table"))
	assert.False(t, hasDetailKey(details, "Record"))
}

func TestHasDetailKey_Nil(t *testing.T) {
	assert.False(t, hasDetailKey(nil, "anything"))
}

func TestGetDetailValue_Old_Map(t *testing.T) {
	details := map[string]any{"Auth": "TokenExpired"}
	assert.Equal(t, "TokenExpired", getDetailValue(details, "Auth"))
	assert.Nil(t, getDetailValue(details, "Missing"))
}

func TestGetDetailValue_Old_String(t *testing.T) {
	assert.Nil(t, getDetailValue("Parse", "Parse"))
}

func TestGetDetailValue_Nil(t *testing.T) {
	assert.Nil(t, getDetailValue(nil, "anything"))
}

func TestGetDetailMapString_Old(t *testing.T) {
	details := map[string]any{"Table": map[string]any{"name": "users"}}
	assert.Equal(t, "users", getDetailMapString(details, "Table", "name"))
	assert.Equal(t, "", getDetailMapString(details, "Table", "missing"))
	assert.Equal(t, "", getDetailMapString(details, "Missing", "name"))
	assert.Equal(t, "", getDetailMapString(nil, "Table", "name"))
}

// ================================================================= //
//  toInt helper                                                      //
// ================================================================= //

func TestToInt(t *testing.T) {
	v, ok := toInt(5)
	assert.True(t, ok)
	assert.Equal(t, 5, v)

	v, ok = toInt(int64(10))
	assert.True(t, ok)
	assert.Equal(t, 10, v)

	v, ok = toInt(float64(3.0))
	assert.True(t, ok)
	assert.Equal(t, 3, v)

	v, ok = toInt(uint64(42))
	assert.True(t, ok)
	assert.Equal(t, 42, v)

	_, ok = toInt("not a number")
	assert.False(t, ok)

	_, ok = toInt(nil)
	assert.False(t, ok)
}

// ================================================================= //
//  Catch-all: errors.As with *ServerError                            //
// ================================================================= //

func TestCatchAll_ServerErrorViaErrorsAs(t *testing.T) {
	errs := []error{
		parseRpcError(&connection.RPCError{Kind: "Internal", Message: "a"}),
		parseRpcError(&connection.RPCError{Kind: "NotFound", Message: "b"}),
		parseQueryError("c", "Query", nil, nil),
	}

	for _, err := range errs {
		var se *ServerError
		assert.True(t, errors.As(err, &se), "expected errors.As to match *ServerError")
	}
}

func TestCatchAll_NonServerError_DoesNotMatch(t *testing.T) {
	err := errors.New("not a server error")
	var se *ServerError
	assert.False(t, errors.As(err, &se))
}

// ================================================================= //
//  resolveKind                                                       //
// ================================================================= //

func TestResolveKind_PrefersKindOverCode(t *testing.T) {
	assert.Equal(t, "NotFound", resolveKind("NotFound", -32000))
}

func TestResolveKind_FallsBackToCode(t *testing.T) {
	assert.Equal(t, "Internal", resolveKind("", -32000))
	assert.Equal(t, "NotFound", resolveKind("", -32601))
}

func TestResolveKind_FallsBackToInternal(t *testing.T) {
	assert.Equal(t, "Internal", resolveKind("", 0))
}
