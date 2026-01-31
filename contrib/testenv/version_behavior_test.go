package testenv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// This file contains integration tests that document behavioral differences
// between SurrealDB 2.x and 3.x. Each test pair (TestXxx_v2 and TestXxx_v3)
// shows the exact difference in behavior between versions.
//
// Run these tests against specific versions using:
//   SURREALDB_VERSION=v2.6.0 go test -run "TestBehavior.*_v2" ./contrib/testenv/
//   SURREALDB_VERSION=v3.0.0-beta.2 go test -run "TestBehavior.*_v3" ./contrib/testenv/

func setupVersionTest(t *testing.T, version string) (db *surrealdb.DB, cleanup func()) {
	_, db, cleanup = setupVersionTestWithHTTPURL(t, version)
	return db, cleanup
}

func setupVersionTestWithArgs(t *testing.T, version string, extraArgs ...string) (db *surrealdb.DB, cleanup func()) {
	_, db, cleanup = setupVersionTestWithHTTPURL(t, version, extraArgs...)
	return db, cleanup
}

func setupVersionTestWithHTTPURL(t *testing.T, version string, extraArgs ...string) (wsURL string, db *surrealdb.DB, cleanup func()) {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping version behavior test")
	}

	ctx := context.Background()

	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		t.Skip("Docker daemon not running, skipping version behavior test")
	}

	containerName := fmt.Sprintf("surrealdb-behavior-test-%s-%d", version, time.Now().UnixNano())

	// Cleanup any existing container
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	// Build command arguments
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", "0:8000",
		fmt.Sprintf("surrealdb/surrealdb:%s", version),
		"start", "--user", "root", "--pass", "root",
	}
	args = append(args, extraArgs...)

	// Start container with dynamic port allocation (port 0 lets Docker choose)
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to start container: %s", string(output))

	containerCleanup := func() {
		cleanupCtx := context.Background()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", containerName).Run()
	}

	// Get the dynamically allocated port
	portCmd := exec.CommandContext(ctx, "docker", "port", containerName, "8000")
	portOutput, err := portCmd.CombinedOutput()
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to get container port: %v, output: %s", err, string(portOutput))
	}
	// Output format: "0.0.0.0:12345\n" - extract port number
	portStr := string(portOutput)
	// Find the last colon and extract port
	for i := len(portStr) - 1; i >= 0; i-- {
		if portStr[i] == ':' {
			portStr = portStr[i+1:]
			break
		}
	}
	portStr = portStr[:len(portStr)-1] // Remove trailing newline
	wsURL = fmt.Sprintf("ws://localhost:%s/rpc", portStr)

	// Wait for container to be ready
	for i := 0; i < 30; i++ {
		db, err = surrealdb.FromEndpointURLString(ctx, wsURL)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to connect to SurrealDB: %v", err)
	}

	// Sign in as root
	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to sign in: %v", err)
	}

	// Use a test namespace/database
	err = db.Use(ctx, "test_ns", "test_db")
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to use database: %v", err)
	}

	cleanup = func() {
		db.Close(ctx)
		containerCleanup()
	}
	return wsURL, db, cleanup
}

// =============================================================================
// Test: SELECT from non-existent table
// =============================================================================

func TestBehavior_SelectNonExistentTable_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 2.x, SELECT from a non-existent table returns empty result, no error
	result, err := surrealdb.Query[[]map[string]any](ctx, db, "SELECT * FROM nonexistent_table", nil)

	require.NoError(t, err, "v2.x should NOT return error for SELECT from non-existent table")
	require.NotNil(t, result)
	// Result contains one query result with empty array
	require.Len(t, *result, 1)
}

func TestBehavior_SelectNonExistentTable_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 3.x, SELECT from a non-existent table returns an error
	_, err := surrealdb.Query[[]map[string]any](ctx, db, "SELECT * FROM nonexistent_table", nil)

	require.Error(t, err, "v3.x SHOULD return error for SELECT from non-existent table")
	require.Contains(t, err.Error(), "does not exist")
}

// =============================================================================
// Test: DELETE from non-existent table
// =============================================================================

func TestBehavior_DeleteNonExistentTable_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 2.x, DELETE from a non-existent table succeeds (no-op)
	_, err := surrealdb.Query[any](ctx, db, "DELETE nonexistent_table", nil)

	require.NoError(t, err, "v2.x should NOT return error for DELETE from non-existent table")
}

func TestBehavior_DeleteNonExistentTable_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 3.x, DELETE from a non-existent table returns an error
	_, err := surrealdb.Query[any](ctx, db, "DELETE nonexistent_table", nil)

	require.Error(t, err, "v3.x SHOULD return error for DELETE from non-existent table")
	require.Contains(t, err.Error(), "does not exist")
}

// =============================================================================
// Test: LIVE SELECT on non-existent table
// =============================================================================

func TestBehavior_LiveSelectNonExistentTable_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 2.x, LIVE SELECT on a non-existent table succeeds
	live, err := surrealdb.Live(ctx, db, "nonexistent_table", false)

	require.NoError(t, err, "v2.x should NOT return error for LIVE SELECT on non-existent table")
	require.NotNil(t, live)

	// Cleanup
	_ = surrealdb.Kill(ctx, db, live.String())
}

func TestBehavior_LiveSelectNonExistentTable_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 3.x, LIVE SELECT on a non-existent table returns an error
	_, err := surrealdb.Live(ctx, db, "nonexistent_table", false)

	require.Error(t, err, "v3.x SHOULD return error for LIVE SELECT on non-existent table")
	require.Contains(t, err.Error(), "does not exist")
}

// =============================================================================
// Test: CREATE with Table target (not specific RecordID)
// =============================================================================

type testUser struct {
	ID       *models.RecordID `json:"id,omitempty"`
	Username string           `json:"username"`
	Password string           `json:"password"`
}

func TestBehavior_CreateWithTable_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// First define the table
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// In SurrealDB 2.x, CREATE with a table returns a single object
	user, err := surrealdb.Create[testUser](ctx, db, models.Table("users"), testUser{
		Username: "johnny",
		Password: "123",
	})

	require.NoError(t, err, "v2.x CREATE with table should succeed")
	require.NotNil(t, user)
	require.Equal(t, "johnny", user.Username)
}

func TestBehavior_CreateWithTable_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// First define the table
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// In SurrealDB 3.x, CREATE with a table returns an array (even for single record)
	// This causes unmarshaling error when expecting a single object
	_, err = surrealdb.Create[testUser](ctx, db, models.Table("users"), testUser{
		Username: "johnny",
		Password: "123",
	})

	// Document the actual behavior - this currently fails with unmarshaling error
	// because the SDK expects a single object but 3.x returns an array
	if err != nil {
		require.Contains(t, err.Error(), "cannot decode array",
			"v3.x CREATE with table returns array, causing unmarshal error")
	} else {
		t.Log("v3.x CREATE with table succeeded - SDK may have been updated to handle arrays")
	}
}

func TestBehavior_CreateWithRecordID_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// First define the table
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// CREATE with specific RecordID works in both versions
	recordID := models.NewRecordID("users", "user1")
	user, err := surrealdb.Create[testUser](ctx, db, recordID, testUser{
		Username: "johnny",
		Password: "123",
	})

	require.NoError(t, err, "v2.x CREATE with RecordID should succeed")
	require.NotNil(t, user)
	require.Equal(t, "johnny", user.Username)
}

func TestBehavior_CreateWithRecordID_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// First define the table
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// CREATE with specific RecordID works in both versions
	recordID := models.NewRecordID("users", "user1")
	user, err := surrealdb.Create[testUser](ctx, db, recordID, testUser{
		Username: "johnny",
		Password: "123",
	})

	require.NoError(t, err, "v3.x CREATE with RecordID should succeed")
	require.NotNil(t, user)
	require.Equal(t, "johnny", user.Username)
}

// =============================================================================
// Test: Authentication error message format
// =============================================================================

func TestBehavior_AuthErrorMessage_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 2.x, auth error includes "There was a problem with the database:"
	_, err := db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "wrong_password",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "There was a problem with the database:",
		"v2.x auth error should include 'There was a problem with the database:'")
	require.Contains(t, err.Error(), "problem with authentication")
}

func TestBehavior_AuthErrorMessage_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// In SurrealDB 3.x, auth error is shorter (no "There was a problem with the database:")
	_, err := db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "wrong_password",
	})

	require.Error(t, err)
	// v3.x has a shorter error message
	require.Contains(t, err.Error(), "problem with authentication")
	// v3.x does NOT have the longer prefix
	require.NotContains(t, err.Error(), "There was a problem with the database:",
		"v3.x auth error should NOT include 'There was a problem with the database:' prefix")
}

// =============================================================================
// Test: type::thing vs type::record function availability
// =============================================================================

func TestBehavior_TypeThingFunction_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// Define table first
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// Create a user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:test SET name = "test"`, nil)
	require.NoError(t, err)

	// In SurrealDB 2.x, type::thing() is available
	result, err := surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM type::thing("users", "test")`, nil)

	require.NoError(t, err, "v2.x should support type::thing()")
	require.NotNil(t, result)
}

func TestBehavior_TypeThingFunction_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// Define table first
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// Create a user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:test SET name = "test"`, nil)
	require.NoError(t, err)

	// In SurrealDB 3.x, type::thing() is removed
	_, err = surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM type::thing("users", "test")`, nil)

	require.Error(t, err, "v3.x should NOT support type::thing()")
}

func TestBehavior_TypeRecordFunction_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// Define table first
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// Create a user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:test SET name = "test"`, nil)
	require.NoError(t, err)

	// In SurrealDB 2.x, type::record() exists but with different semantics than v3.x
	// v2.x: type::record(value) - converts a value to a record (takes 1 argument)
	// v3.x: type::record(table, id) - creates a record ID from table and id (takes 2 arguments)

	// First verify the correct v2.x usage: type::record with 1 argument
	result, err := surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM type::record("users:test")`, nil)
	require.NoError(t, err, "v2.x type::record(value) with 1 argument should work")
	require.NotNil(t, result)
	require.Len(t, *result, 1, "should have 1 query result")
	require.Equal(t, "OK", (*result)[0].Status)
	require.Len(t, (*result)[0].Result, 1, "should have 1 record")
	require.Equal(t, "test", (*result)[0].Result[0]["name"], "record should have correct name")

	// Now verify the v3.x-style usage fails: type::record with 2 arguments
	_, err = surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM type::record("users", "test")`, nil)
	require.Error(t, err, "v2.x type::record() takes 1 argument, not 2 like v3.x")
}

func TestBehavior_TypeRecordFunction_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// Define table first
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// Create a user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:test SET name = "test"`, nil)
	require.NoError(t, err)

	// In SurrealDB 3.x, type::record(table, id) should work
	result, err := surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM type::record("users", "test")`, nil)

	require.NoError(t, err, "v3.x should support type::record(table, id)")
	require.NotNil(t, result)
	require.Len(t, *result, 1, "should have 1 query result")
	require.Equal(t, "OK", (*result)[0].Status)
	require.Len(t, (*result)[0].Result, 1, "should have 1 record")
	require.Equal(t, "test", (*result)[0].Result[0]["name"], "record should have correct name")
}

// =============================================================================
// Test: RecordID as query variable (universal approach)
// =============================================================================

func TestBehavior_RecordIDAsVariable_v2(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v2.6.0")
	defer cleanup()

	ctx := context.Background()

	// Define table first
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// Create a user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:test SET name = "test"`, nil)
	require.NoError(t, err)

	// Using RecordID as a query variable works in both versions
	recordID := models.NewRecordID("users", "test")
	result, err := surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM $id`, map[string]any{
		"id": recordID,
	})

	require.NoError(t, err, "v2.x should support RecordID as query variable")
	require.NotNil(t, result)
}

func TestBehavior_RecordIDAsVariable_v3(t *testing.T) {
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// Define table first
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE users", nil)
	require.NoError(t, err)

	// Create a user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:test SET name = "test"`, nil)
	require.NoError(t, err)

	// Using RecordID as a query variable works in both versions
	recordID := models.NewRecordID("users", "test")
	result, err := surrealdb.Query[[]map[string]any](ctx, db, `SELECT * FROM $id`, map[string]any{
		"id": recordID,
	})

	require.NoError(t, err, "v3.x should support RecordID as query variable")
	require.NotNil(t, result)
}

// =============================================================================
// Test: Bearer Access Method for System Users
//
// Bearer access methods allow generating bearer grants with an associated key
// that can be used to authenticate as a specific system user or record user.
//
// Findings from these tests:
//
// 1. Grant Result Structure (same for v2 and v3):
//    {
//      ac: "bearer_api",              // access method name
//      creation: {Time: ...},         // grant creation time
//      expiration: {Time: ...},       // grant expiration time (default 30 days)
//      grant: {
//        id: "abc123...",             // grant ID
//        key: "surreal-bearer-...",   // the bearer key for signin
//      },
//      id: "abc123...",               // same as grant.id
//      revocation: nil,               // null if not revoked
//      subject: {
//        user: "testuser"             // for system user grants
//        // OR
//        record: {Table: "...", ID: "..."}  // for record user grants
//      },
//      type: "bearer"
//    }
//
// 2. SignIn Response: Both v2 and v3 return a string JWT token
//
// 3. Experimental Flag:
//    - v2.x: requires --allow-experimental bearer_access
//    - v3.x: bearer access is enabled by default (no experimental flag needed)
// =============================================================================

func TestBehavior_BearerAccessSystemUser_v2(t *testing.T) {
	// In v2.x, bearer access is an experimental feature that must be explicitly enabled
	db, cleanup := setupVersionTestWithArgs(t, "v2.6.0", "--allow-experimental", "bearer_access")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a database-level user
	_, err := surrealdb.Query[any](ctx, db, `DEFINE USER testuser ON DATABASE PASSWORD 'testpass' ROLES EDITOR`, nil)
	require.NoError(t, err)

	// 2. Define bearer access method for users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	require.NoError(t, err)

	// 3. Grant bearer key for the user (returns a single object, not an array)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER testuser`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 4. Extract bearer key from grant result (key is nested in grant.key)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 5. Sign in with bearer key - v2 returns string JWT token directly
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v2.x signin should return string token")

	// 6. SignIn established the authenticated session on the WebSocket connection.
	// Verify we can perform operations as the authenticated user.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin")
}

func TestBehavior_BearerAccessSystemUser_v3(t *testing.T) {
	// In v3.x, bearer access is enabled by default (no experimental flag needed)
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a database-level user
	_, err := surrealdb.Query[any](ctx, db, `DEFINE USER testuser ON DATABASE PASSWORD 'testpass' ROLES EDITOR`, nil)
	require.NoError(t, err)

	// 2. Define bearer access method for users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	require.NoError(t, err)

	// 3. Grant bearer key for the user (returns a single object, not an array)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER testuser`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 4. Extract bearer key from grant result (key is nested in grant.key, same as v2)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 5. Sign in with bearer key - v3 returns string JWT token (same as v2)
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v3.x signin should return string token")

	// 6. SignIn established the authenticated session. Verify we can perform operations.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin")
}

// =============================================================================
// Test: Bearer Access Method for Record Users
//
// Record user bearer grants work similarly to system user grants, but the
// subject contains a record reference instead of a user name:
//   subject: { record: { Table: "users", ID: "testrecord" } }
// =============================================================================

func TestBehavior_BearerAccessRecordUser_v2(t *testing.T) {
	// In v2.x, bearer access is an experimental feature that must be explicitly enabled
	db, cleanup := setupVersionTestWithArgs(t, "v2.6.0", "--allow-experimental", "bearer_access")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a table for record users
	_, err := surrealdb.Query[any](ctx, db, `DEFINE TABLE users SCHEMAFULL`, nil)
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD name ON users TYPE string`, nil)
	require.NoError(t, err)

	// 2. Create a record user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:testrecord SET name = 'Test Record User'`, nil)
	require.NoError(t, err)

	// 3. Define bearer access method for records
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_record_api ON DATABASE TYPE BEARER FOR RECORD`, nil)
	require.NoError(t, err)

	// 4. Grant bearer key for the record (returns single object with grant.key)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_record_api GRANT FOR RECORD users:testrecord`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 5. Extract bearer key from grant result (key is nested in grant.key)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 6. Sign in with bearer key - v2 returns string JWT token
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_record_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v2.x signin should return string token")

	// 7. SignIn established the authenticated session. Verify we can perform operations.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin as record user")
}

func TestBehavior_BearerAccessRecordUser_v3(t *testing.T) {
	// In v3.x, bearer access is enabled by default (no experimental flag needed)
	db, cleanup := setupVersionTest(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a table for record users
	_, err := surrealdb.Query[any](ctx, db, `DEFINE TABLE users SCHEMAFULL`, nil)
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, `DEFINE FIELD name ON users TYPE string`, nil)
	require.NoError(t, err)

	// 2. Create a record user
	_, err = surrealdb.Query[any](ctx, db, `CREATE users:testrecord SET name = 'Test Record User'`, nil)
	require.NoError(t, err)

	// 3. Define bearer access method for records
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_record_api ON DATABASE TYPE BEARER FOR RECORD`, nil)
	require.NoError(t, err)

	// 4. Grant bearer key for the record (returns single object with grant.key, same as v2)
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_record_api GRANT FOR RECORD users:testrecord`, nil)
	require.NoError(t, err)
	require.NotNil(t, grantResult)
	require.Len(t, *grantResult, 1, "should have 1 query result")

	// 5. Extract bearer key from grant result (key is nested in grant.key)
	grantData := (*grantResult)[0].Result
	grantInfo, ok := grantData["grant"].(map[string]any)
	require.True(t, ok, "grant result should contain 'grant' field as map")
	bearerKey, ok := grantInfo["key"].(string)
	require.True(t, ok, "grant.key should be a string")
	require.NotEmpty(t, bearerKey)

	// 6. Sign in with bearer key - v3 returns string JWT token (same as v2)
	token, err := db.SignIn(ctx, map[string]any{
		"NS":  "test_ns",
		"DB":  "test_db",
		"AC":  "bearer_record_api",
		"key": bearerKey,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token, "v3.x signin should return string token")

	// 7. SignIn established the authenticated session. Verify we can perform operations.
	_, err = surrealdb.Query[any](ctx, db, `RETURN 1`, nil)
	require.NoError(t, err, "should be able to query after bearer signin as record user")
}

// =============================================================================
// Test: Bearer Access Method - HTTP API Response Format
//
// These tests verify the HTTP /signin endpoint response format, which may differ
// from the WebSocket RPC response. The HTTP API is tested using Go's net/http
// client to match the curl example from SurrealDB documentation:
//
//	curl -X POST \
//	  -H "Accept: application/json" \
//	  -d '{"NS":"test", "DB":"test", "AC":"api", "key":"surreal-bearer-..."}' \
//	  http://localhost:8000/signin
// =============================================================================

func TestBehavior_BearerAccessHTTP_v2(t *testing.T) {
	// In v2.x, bearer access is an experimental feature that must be explicitly enabled
	wsURL, db, cleanup := setupVersionTestWithHTTPURL(t, "v2.6.0", "--allow-experimental", "bearer_access")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a database-level user
	_, err := surrealdb.Query[any](ctx, db, `DEFINE USER testuser ON DATABASE PASSWORD 'testpass' ROLES EDITOR`, nil)
	require.NoError(t, err)

	// 2. Define bearer access method for users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	require.NoError(t, err)

	// 3. Grant bearer key for the user
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER testuser`, nil)
	require.NoError(t, err)
	grantData := (*grantResult)[0].Result
	grantInfo := grantData["grant"].(map[string]any)
	bearerKey := grantInfo["key"].(string)

	// 4. Get HTTP URL from the WebSocket URL (same port, different scheme)
	httpURL := strings.Replace(wsURL, "ws://", "http://", 1)
	httpURL = strings.TrimSuffix(httpURL, "/rpc")
	signinURL := httpURL + "/signin"

	// 5. Call HTTP /signin endpoint directly
	reqBody := fmt.Sprintf(`{"NS":"test_ns", "DB":"test_db", "AC":"bearer_api", "key":"%s"}`, bearerKey)
	req, err := http.NewRequestWithContext(ctx, "POST", signinURL, strings.NewReader(reqBody))
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, "HTTP signin should succeed")

	// HTTP /signin response format for v2.x bearer access:
	// {
	//   "code": 200,
	//   "details": "Authentication succeeded",
	//   "token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9...",
	//   "refresh": null
	// }
	// Note: v2 includes "refresh": null field (explicitly null)
	// This differs from WebSocket RPC signin which returns just the token string.

	var httpResponse map[string]any
	err = json.Unmarshal(body, &httpResponse)
	require.NoError(t, err)

	// Verify expected fields
	require.Equal(t, float64(200), httpResponse["code"], "should have code 200")
	require.Equal(t, "Authentication succeeded", httpResponse["details"])

	token, ok := httpResponse["token"].(string)
	require.True(t, ok, "token should be a string")
	require.NotEmpty(t, token, "token should not be empty")

	// v2 explicitly includes refresh field as null
	_, hasRefresh := httpResponse["refresh"]
	require.True(t, hasRefresh, "v2 HTTP response should include refresh field (as null)")
	require.Nil(t, httpResponse["refresh"], "v2 refresh should be null")
}

func TestBehavior_BearerAccessHTTP_v3(t *testing.T) {
	// In v3.x, bearer access is enabled by default
	wsURL, db, cleanup := setupVersionTestWithHTTPURL(t, "v3.0.0-beta.2")
	defer cleanup()

	ctx := context.Background()

	// 1. Define a database-level user
	_, err := surrealdb.Query[any](ctx, db, `DEFINE USER testuser ON DATABASE PASSWORD 'testpass' ROLES EDITOR`, nil)
	require.NoError(t, err)

	// 2. Define bearer access method for users
	_, err = surrealdb.Query[any](ctx, db, `DEFINE ACCESS bearer_api ON DATABASE TYPE BEARER FOR USER`, nil)
	require.NoError(t, err)

	// 3. Grant bearer key for the user
	grantResult, err := surrealdb.Query[map[string]any](ctx, db, `ACCESS bearer_api GRANT FOR USER testuser`, nil)
	require.NoError(t, err)
	grantData := (*grantResult)[0].Result
	grantInfo := grantData["grant"].(map[string]any)
	bearerKey := grantInfo["key"].(string)

	// 4. Get HTTP URL from the WebSocket URL (same port, different scheme)
	httpURL := strings.Replace(wsURL, "ws://", "http://", 1)
	httpURL = strings.TrimSuffix(httpURL, "/rpc")
	signinURL := httpURL + "/signin"

	// 5. Call HTTP /signin endpoint directly
	reqBody := fmt.Sprintf(`{"NS":"test_ns", "DB":"test_db", "AC":"bearer_api", "key":"%s"}`, bearerKey)
	req, err := http.NewRequestWithContext(ctx, "POST", signinURL, strings.NewReader(reqBody))
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, "HTTP signin should succeed")

	// HTTP /signin response format for v3.x bearer access:
	// {
	//   "code": 200,
	//   "details": "Authentication succeeded",
	//   "token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9..."
	// }
	// Note: v3 does NOT include the "refresh" field at all (unlike v2 which has "refresh": null).
	// This differs from WebSocket RPC signin which returns just the token string.
	//
	// Key insight: Both HTTP and WebSocket return only the JWT token (no refresh token).
	// The object-with-refresh-token format mentioned in some documentation may apply to
	// other access methods or future versions.

	var httpResponse map[string]any
	err = json.Unmarshal(body, &httpResponse)
	require.NoError(t, err)

	// Verify expected fields
	require.Equal(t, float64(200), httpResponse["code"], "should have code 200")
	require.Equal(t, "Authentication succeeded", httpResponse["details"])

	token, ok := httpResponse["token"].(string)
	require.True(t, ok, "token should be a string")
	require.NotEmpty(t, token, "token should not be empty")

	// v3 does NOT include refresh field (different from v2 which has refresh: null)
	_, hasRefresh := httpResponse["refresh"]
	require.False(t, hasRefresh, "v3 HTTP response should NOT include refresh field")
}
