package testenv

import (
	"context"
	"fmt"
	"os/exec"
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

	// Start container on a different port to avoid conflicts
	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", containerName,
		"-p", VersionBehaviorTestPortMapping,
		fmt.Sprintf("surrealdb/surrealdb:%s", version),
		"start", "--user", "root", "--pass", "root",
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to start container: %s", string(output))

	containerCleanup := func() {
		cleanupCtx := context.Background()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", containerName).Run()
	}

	// Wait for container to be ready
	for i := 0; i < 30; i++ {
		db, err = surrealdb.FromEndpointURLString(ctx, VersionBehaviorTestWSURL)
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
	return db, cleanup
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
