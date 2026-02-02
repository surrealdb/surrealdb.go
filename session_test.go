package surrealdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// getVersion gets and checks the SurrealDB version via a separate DB connection
func getVersion(t *testing.T) *testenv.SurrealDBVersion {
	t.Helper()
	db := testenv.MustNew("version_check", "version_check", "dummy")
	v, err := testenv.GetVersion(context.Background(), db)
	require.NoError(t, err)
	_ = db.Close(context.Background())
	return v
}

// mustNewWS creates a WebSocket connection for testing.
// It uses GetSurrealDBWSURL() to always get a WebSocket URL,
// even if SURREALDB_URL is set to an HTTP URL (e.g., in CI).
// It also defines the specified tables for SurrealDB v3 compatibility.
func mustNewWS(namespace, database string, tables ...string) *surrealdb.DB {
	wsURL := testenv.GetSurrealDBWSURL()
	db, err := surrealdb.FromEndpointURLString(context.Background(), wsURL)
	if err != nil {
		panic("Failed to create WebSocket connection: " + err.Error())
	}

	db, err = testenv.Init(db, namespace, database, tables...)
	if err != nil {
		panic("Failed to initialize test environment: " + err.Error())
	}

	// SurrealDB v3 requires explicit table definitions before use
	if err = testenv.DefineSchemalessTables(db, tables...); err != nil {
		panic("Failed to define tables: " + err.Error())
	}

	return db
}

// TestSession_Attach tests session creation on WebSocket connections.
func TestSession_Attach(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	db := mustNewWS("session_test", "attach_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err, "Attach should succeed on WebSocket connection")
	require.NotNil(t, session, "Session should not be nil")
	require.NotNil(t, session.ID(), "Session ID should not be nil")

	t.Logf("Created session with ID: %s", session.ID().String())

	// Clean up
	err = session.Detach(ctx)
	require.NoError(t, err, "Detach should succeed")
}

// TestSession_AttachHTTPError tests that Attach returns an error on HTTP connections.
func TestSession_AttachHTTPError(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	db := testenv.MustNewHTTP("session_test", "attach_http_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.Error(t, err, "Attach should fail on HTTP connection")
	assert.ErrorIs(t, err, constants.ErrSessionsNotSupported, "Error should be ErrSessionsNotSupported")
	assert.Nil(t, session, "Session should be nil on error")
}

// TestSession_Detach tests session cleanup.
func TestSession_Detach(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	db := mustNewWS("session_test", "detach_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err)

	err = session.Detach(ctx)
	require.NoError(t, err, "Detach should succeed")
}

// TestSession_DoubleDetach tests that double detach returns an error.
func TestSession_DoubleDetach(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	db := mustNewWS("session_test", "double_detach_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err)

	// First detach should succeed
	err = session.Detach(ctx)
	require.NoError(t, err)

	// Second detach should fail
	err = session.Detach(ctx)
	require.Error(t, err, "Second Detach should fail")
	assert.ErrorIs(t, err, constants.ErrSessionClosed, "Error should be ErrSessionClosed")
}

// TestSession_BeginTransaction tests starting a transaction within a session.
func TestSession_BeginTransaction(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	db := mustNewWS("session_test", "begin_txn_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session.Detach(ctx)

	// Authenticate and select namespace/database on the session
	_, err = session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)

	err = session.Use(ctx, "session_test", "begin_txn_test")
	require.NoError(t, err)

	// Begin a transaction
	tx, err := session.Begin(ctx)
	require.NoError(t, err, "Begin should succeed")
	require.NotNil(t, tx, "Transaction should not be nil")
	require.NotNil(t, tx.ID(), "Transaction ID should not be nil")
	require.NotNil(t, tx.SessionID(), "Transaction SessionID should not be nil")
	assert.Equal(t, session.ID().String(), tx.SessionID().String(), "Transaction should be associated with the session")

	t.Logf("Created transaction with ID: %s in session: %s", tx.ID().String(), tx.SessionID().String())

	// Cancel the transaction
	err = tx.Cancel(ctx)
	require.NoError(t, err)
}

// TestSession_Query tests executing queries within a session.
func TestSession_Query(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = mustNewWS("session_test", "query_test", "users")

	db := mustNewWS("session_test", "query_test", "users")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session.Detach(ctx)

	// Authenticate and select namespace/database
	_, err = session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)

	err = session.Use(ctx, "session_test", "query_test")
	require.NoError(t, err)

	type User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// Create a user using the session
	results, err := surrealdb.Query[[]User](ctx, session, "CREATE users:session_user SET name = 'SessionUser'", nil)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Equal(t, "OK", (*results)[0].Status)

	// Select the user
	selectResults, err := surrealdb.Query[[]User](ctx, session, "SELECT * FROM users", nil)
	require.NoError(t, err)
	require.NotNil(t, selectResults)
	require.Len(t, *selectResults, 1)
	assert.Equal(t, "OK", (*selectResults)[0].Status)
	assert.Len(t, (*selectResults)[0].Result, 1)
	assert.Equal(t, "SessionUser", (*selectResults)[0].Result[0].Name)
}

// TestSession_Isolation_Namespace tests that sessions can have independent namespace/database selections.
func TestSession_Isolation_Namespace(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	// Create two different databases
	_ = mustNewWS("session_isolation", "db1", "items")
	_ = mustNewWS("session_isolation", "db2", "items")

	db := mustNewWS("session_isolation", "db1", "items")
	defer db.Close(context.Background())

	ctx := context.Background()

	// Create item in db1 using the main connection
	_, err := surrealdb.Query[[]any](ctx, db, "CREATE items:item1 SET source = 'db1'", nil)
	require.NoError(t, err)

	// Create session 1 pointing to db1
	session1, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session1.Detach(ctx)

	_, err = session1.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)
	err = session1.Use(ctx, "session_isolation", "db1")
	require.NoError(t, err)

	// Create session 2 pointing to db2
	session2, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session2.Detach(ctx)

	_, err = session2.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)
	err = session2.Use(ctx, "session_isolation", "db2")
	require.NoError(t, err)

	// Create item in db2 using session2
	_, err = surrealdb.Query[[]any](ctx, session2, "CREATE items:item2 SET source = 'db2'", nil)
	require.NoError(t, err)

	type Item struct {
		ID     string `json:"id"`
		Source string `json:"source"`
	}

	// Query from session1 - should only see db1 item
	results1, err := surrealdb.Query[[]Item](ctx, session1, "SELECT * FROM items", nil)
	require.NoError(t, err)
	require.NotNil(t, results1)
	require.Len(t, *results1, 1)
	assert.Len(t, (*results1)[0].Result, 1, "Session1 should only see 1 item from db1")
	assert.Equal(t, "db1", (*results1)[0].Result[0].Source)

	// Query from session2 - should only see db2 item
	results2, err := surrealdb.Query[[]Item](ctx, session2, "SELECT * FROM items", nil)
	require.NoError(t, err)
	require.NotNil(t, results2)
	require.Len(t, *results2, 1)
	assert.Len(t, (*results2)[0].Result, 1, "Session2 should only see 1 item from db2")
	assert.Equal(t, "db2", (*results2)[0].Result[0].Source)
}

// TestSession_Isolation_Variables tests that sessions can have independent variables.
func TestSession_Isolation_Variables(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = mustNewWS("session_isolation", "var_test", "test_table")

	db := mustNewWS("session_isolation", "var_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	// Create two sessions
	session1, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session1.Detach(ctx)

	_, err = session1.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)
	err = session1.Use(ctx, "session_isolation", "var_test")
	require.NoError(t, err)

	session2, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session2.Detach(ctx)

	_, err = session2.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)
	err = session2.Use(ctx, "session_isolation", "var_test")
	require.NoError(t, err)

	// Set variable $x in session1
	err = session1.Let(ctx, "x", "session1_value")
	require.NoError(t, err)

	// Set different variable $x in session2
	err = session2.Let(ctx, "x", "session2_value")
	require.NoError(t, err)

	// Query $x from session1
	results1, err := surrealdb.Query[string](ctx, session1, "RETURN $x", nil)
	require.NoError(t, err)
	require.NotNil(t, results1)
	require.Len(t, *results1, 1)
	assert.Equal(t, "session1_value", (*results1)[0].Result, "Session1 should have its own $x value")

	// Query $x from session2
	results2, err := surrealdb.Query[string](ctx, session2, "RETURN $x", nil)
	require.NoError(t, err)
	require.NotNil(t, results2)
	require.Len(t, *results2, 1)
	assert.Equal(t, "session2_value", (*results2)[0].Result, "Session2 should have its own $x value")
}

// TestSession_CRUD_Query tests Create, Select, Update, Delete operations within a session using Query.
func TestSession_CRUD_Query(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = mustNewWS("session_crud", "crud_test", "products")

	db := mustNewWS("session_crud", "crud_test", "products")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session.Detach(ctx)

	_, err = session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)
	err = session.Use(ctx, "session_crud", "crud_test")
	require.NoError(t, err)

	// SurrealDB v3 requires explicit table definition within the session context
	defineRes, err := surrealdb.Query[any](ctx, session, "DEFINE TABLE IF NOT EXISTS products SCHEMALESS", nil)
	require.NoError(t, err)
	require.NotNil(t, defineRes)
	require.Len(t, *defineRes, 1)
	require.Equal(t, "OK", (*defineRes)[0].Status, "DEFINE TABLE should succeed")

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	// Create - using Query for reliable behavior in sessions
	createResults, err := surrealdb.Query[[]Product](ctx, session, "CREATE products:widget SET name = 'Widget', price = 9.99", nil)
	require.NoError(t, err)
	require.NotNil(t, createResults)
	require.Len(t, *createResults, 1)
	require.Equal(t, "OK", (*createResults)[0].Status, "CREATE should succeed")
	require.Len(t, (*createResults)[0].Result, 1)
	assert.Equal(t, "Widget", (*createResults)[0].Result[0].Name)
	assert.Equal(t, 9.99, (*createResults)[0].Result[0].Price)

	// Select - using Query instead of Select for reliable behavior
	selectResults, err := surrealdb.Query[[]Product](ctx, session, "SELECT * FROM products:widget", nil)
	require.NoError(t, err)
	require.NotNil(t, selectResults)
	require.Len(t, *selectResults, 1)
	require.Equal(t, "OK", (*selectResults)[0].Status, "SELECT should succeed")
	require.Len(t, (*selectResults)[0].Result, 1)
	assert.Equal(t, "Widget", (*selectResults)[0].Result[0].Name)

	// Update - using Query instead of Update for reliable behavior
	updateResults, err := surrealdb.Query[[]Product](ctx, session, "UPDATE products:widget SET name = 'Super Widget', price = 19.99", nil)
	require.NoError(t, err)
	require.NotNil(t, updateResults)
	require.Len(t, *updateResults, 1)
	require.Len(t, (*updateResults)[0].Result, 1)
	assert.Equal(t, "Super Widget", (*updateResults)[0].Result[0].Name)
	assert.Equal(t, 19.99, (*updateResults)[0].Result[0].Price)

	// Delete - using Query instead of Delete for reliable behavior
	deleteResults, err := surrealdb.Query[[]Product](ctx, session, "DELETE products:widget RETURN BEFORE", nil)
	require.NoError(t, err)
	require.NotNil(t, deleteResults)
	require.Len(t, *deleteResults, 1)
	require.Len(t, (*deleteResults)[0].Result, 1)
	assert.Equal(t, "Super Widget", (*deleteResults)[0].Result[0].Name)

	// Verify deleted
	results, err := surrealdb.Query[[]Product](ctx, session, "SELECT * FROM products WHERE id = products:widget", nil)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Empty(t, (*results)[0].Result, "Product should be deleted")
}

// TestSession_CRUD_RPCs tests Create, Select, Update, Delete operations within a session using RPC methods.
func TestSession_CRUD_RPCs(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions (requires v3+)", v)
	}

	_ = mustNewWS("session_crud_rpc", "crud_rpc_test", "products")

	db := mustNewWS("session_crud_rpc", "crud_rpc_test", "products")
	defer db.Close(context.Background())

	ctx := context.Background()

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session.Detach(ctx)

	_, err = session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)
	err = session.Use(ctx, "session_crud_rpc", "crud_rpc_test")
	require.NoError(t, err)

	// SurrealDB v3 requires explicit table definition within the session context
	defineRes, err := surrealdb.Query[any](ctx, session, "DEFINE TABLE IF NOT EXISTS products SCHEMALESS", nil)
	require.NoError(t, err)
	require.NotNil(t, defineRes)
	require.Len(t, *defineRes, 1)
	require.Equal(t, "OK", (*defineRes)[0].Status, "DEFINE TABLE should succeed")

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	// Use RecordID for RPC methods
	recordID := models.NewRecordID("products", "rpc_widget")

	// Create using surrealdb.Create
	product, err := surrealdb.Create[Product](ctx, session, recordID, map[string]any{
		"name":  "RPC Widget",
		"price": 9.99,
	})
	require.NoError(t, err, "Create should succeed")
	require.NotNil(t, product)
	assert.Equal(t, "RPC Widget", product.Name)
	assert.Equal(t, 9.99, product.Price)

	// Select using surrealdb.Select
	selectedProduct, err := surrealdb.Select[Product](ctx, session, recordID)
	require.NoError(t, err, "Select should succeed")
	require.NotNil(t, selectedProduct)
	assert.Equal(t, "RPC Widget", selectedProduct.Name)

	// Update using surrealdb.Update
	updatedProduct, err := surrealdb.Update[Product](ctx, session, recordID, map[string]any{
		"name":  "Super RPC Widget",
		"price": 19.99,
	})
	require.NoError(t, err, "Update should succeed")
	require.NotNil(t, updatedProduct)
	assert.Equal(t, "Super RPC Widget", updatedProduct.Name)
	assert.Equal(t, 19.99, updatedProduct.Price)

	// Delete using surrealdb.Delete
	deletedProduct, err := surrealdb.Delete[Product](ctx, session, recordID)
	require.NoError(t, err, "Delete should succeed")
	require.NotNil(t, deletedProduct)
	assert.Equal(t, "Super RPC Widget", deletedProduct.Name)

	// Verify deleted using surrealdb.Select
	verifyProduct, err := surrealdb.Select[Product](ctx, session, recordID)
	require.NoError(t, err, "Select of deleted record should not error")
	assert.Nil(t, verifyProduct, "Product should be deleted")
}
