package surrealdb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestTransaction_Begin tests transaction creation on WebSocket connections.
func TestTransaction_Begin(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	db := mustNewWS("txn_test", "begin_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err, "Begin should succeed on WebSocket connection")
	require.NotNil(t, tx, "Transaction should not be nil")
	require.NotNil(t, tx.ID(), "Transaction ID should not be nil")
	assert.Nil(t, tx.SessionID(), "Transaction on default session should have nil SessionID")
	assert.False(t, tx.IsClosed(), "Transaction should not be closed initially")

	t.Logf("Created transaction with ID: %s", tx.ID().String())

	// Clean up
	err = tx.Cancel(ctx)
	require.NoError(t, err, "Cancel should succeed")
	assert.True(t, tx.IsClosed(), "Transaction should be closed after Cancel")
}

// TestTransaction_BeginHTTPError tests that Begin returns an error on HTTP connections.
func TestTransaction_BeginHTTPError(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	db := testenv.MustNewHTTP("begin_http_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.Error(t, err, "Begin should fail on HTTP connection")
	assert.ErrorIs(t, err, constants.ErrTransactionsNotSupported, "Error should be ErrTransactionsNotSupported")
	assert.Nil(t, tx, "Transaction should be nil on error")
}

// TestTransaction_Commit tests successful transaction commit.
func TestTransaction_Commit(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_test", "commit_test", "items")

	db := mustNewWS("txn_test", "commit_test", "items")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)

	// Create an item within the transaction
	type Item struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}

	_, err = surrealdb.Query[[]Item](ctx, tx, "CREATE items:commit_item SET value = 'committed'", nil)
	require.NoError(t, err)

	// Commit the transaction
	err = tx.Commit(ctx)
	require.NoError(t, err, "Commit should succeed")
	assert.True(t, tx.IsClosed(), "Transaction should be closed after Commit")

	// Verify data persisted using the main connection (not transaction)
	results, err := surrealdb.Query[[]Item](ctx, db, "SELECT * FROM items WHERE id = items:commit_item", nil)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Len(t, (*results)[0].Result, 1, "Committed data should be visible")
	assert.Equal(t, "committed", (*results)[0].Result[0].Value)
}

// TestTransaction_Cancel tests successful transaction cancellation (rollback).
func TestTransaction_Cancel(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_test", "cancel_test", "items")

	db := mustNewWS("txn_test", "cancel_test", "items")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)

	type Item struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}

	// Create an item within the transaction
	_, err = surrealdb.Query[[]Item](ctx, tx, "CREATE items:cancel_item SET value = 'will be rolled back'", nil)
	require.NoError(t, err)

	// Cancel the transaction
	err = tx.Cancel(ctx)
	require.NoError(t, err, "Cancel should succeed")
	assert.True(t, tx.IsClosed(), "Transaction should be closed after Cancel")

	// Verify data was NOT persisted
	// Note: In SurrealDB v3, the table may not exist after cancellation, so we use IF EXISTS
	results, err := surrealdb.Query[[]Item](ctx, db, "SELECT * FROM items WHERE id = items:cancel_item", nil)
	// Ignore table doesn't exist error - that's expected after cancel if the table was created in the transaction
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		// Table doesn't exist, which means the transaction was rolled back correctly
		return
	}
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Empty(t, (*results)[0].Result, "Cancelled transaction data should be rolled back")
}

// TestTransaction_DoubleCommit tests that committing a committed transaction returns an error.
func TestTransaction_DoubleCommit(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	db := mustNewWS("txn_test", "double_commit_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)

	// First commit should succeed
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Second commit should fail
	err = tx.Commit(ctx)
	require.Error(t, err, "Second Commit should fail")
	assert.ErrorIs(t, err, constants.ErrTransactionClosed, "Error should be ErrTransactionClosed")
}

// TestTransaction_DoubleCancel tests that cancelling a cancelled transaction returns an error.
func TestTransaction_DoubleCancel(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	db := mustNewWS("txn_test", "double_cancel_test", "test_table")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)

	// First cancel should succeed
	err = tx.Cancel(ctx)
	require.NoError(t, err)

	// Second cancel should fail
	err = tx.Cancel(ctx)
	require.Error(t, err, "Second Cancel should fail")
	assert.ErrorIs(t, err, constants.ErrTransactionClosed, "Error should be ErrTransactionClosed")
}

// TestTransaction_Isolation tests that uncommitted changes are not visible to other connections.
func TestTransaction_Isolation(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_test", "isolation_test", "records")

	db1 := mustNewWS("txn_test", "isolation_test", "records")
	defer db1.Close(context.Background())

	db2 := mustNewWS("txn_test", "isolation_test", "records")
	defer db2.Close(context.Background())

	ctx := context.Background()

	// Start transaction on db1
	tx, err := db1.Begin(ctx)
	require.NoError(t, err)

	type Record struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}

	// Create record within transaction
	_, err = surrealdb.Query[[]Record](ctx, tx, "CREATE records:isolated SET value = 'in transaction'", nil)
	require.NoError(t, err)

	// Check from db2 - should NOT see uncommitted data
	// Note: In SurrealDB v3, the table may not exist yet since it's only created inside the uncommitted transaction
	results, err := surrealdb.Query[[]Record](ctx, db2, "SELECT * FROM records WHERE id = records:isolated", nil)
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		// Table doesn't exist yet, which is correct - the transaction hasn't committed
		t.Log("Table doesn't exist yet (uncommitted transaction) - this is expected isolation behavior")
	} else {
		require.NoError(t, err)
		require.NotNil(t, results)
		require.Len(t, *results, 1)
		assert.Empty(t, (*results)[0].Result, "Uncommitted data should not be visible from other connection")
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Now db2 should see the data
	results, err = surrealdb.Query[[]Record](ctx, db2, "SELECT * FROM records WHERE id = records:isolated", nil)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Len(t, (*results)[0].Result, 1, "Committed data should be visible from other connection")
	assert.Equal(t, "in transaction", (*results)[0].Result[0].Value)
}

// TestTransaction_CRUD_Query tests Create, Select, Update, Delete operations within a transaction using Query.
func TestTransaction_CRUD_Query(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_crud", "crud_test", "products")

	db := mustNewWS("txn_crud", "crud_test", "products")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	// Note: we don't defer tx.Cancel here because we want to commit

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	// Create - using Query for reliable behavior in transactions
	createResults, err := surrealdb.Query[[]Product](ctx, tx, "CREATE products:txn_widget SET name = 'Transaction Widget', price = 29.99", nil)
	require.NoError(t, err, "Create within transaction should succeed")
	require.NotNil(t, createResults)
	require.Len(t, *createResults, 1)
	require.Len(t, (*createResults)[0].Result, 1)
	assert.Equal(t, "Transaction Widget", (*createResults)[0].Result[0].Name)

	// Select - using Query for reliable behavior in transactions
	selectResults, err := surrealdb.Query[[]Product](ctx, tx, "SELECT * FROM products:txn_widget", nil)
	require.NoError(t, err, "Select within transaction should succeed")
	require.NotNil(t, selectResults)
	require.Len(t, *selectResults, 1)
	require.Len(t, (*selectResults)[0].Result, 1)
	assert.Equal(t, "Transaction Widget", (*selectResults)[0].Result[0].Name)

	// Update - using Query for reliable behavior in transactions
	updateResults, err := surrealdb.Query[[]Product](ctx, tx, "UPDATE products:txn_widget SET name = 'Super Transaction Widget', price = 49.99", nil)
	require.NoError(t, err, "Update within transaction should succeed")
	require.NotNil(t, updateResults)
	require.Len(t, *updateResults, 1)
	require.Len(t, (*updateResults)[0].Result, 1)
	assert.Equal(t, "Super Transaction Widget", (*updateResults)[0].Result[0].Name)
	assert.Equal(t, 49.99, (*updateResults)[0].Result[0].Price)

	// Delete - using Query for reliable behavior in transactions
	deleteResults, err := surrealdb.Query[[]Product](ctx, tx, "DELETE products:txn_widget RETURN BEFORE", nil)
	require.NoError(t, err, "Delete within transaction should succeed")
	require.NotNil(t, deleteResults)
	require.Len(t, *deleteResults, 1)
	require.Len(t, (*deleteResults)[0].Result, 1)
	assert.Equal(t, "Super Transaction Widget", (*deleteResults)[0].Result[0].Name)

	// Commit
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify deleted after commit
	results, err := surrealdb.Query[[]Product](ctx, db, "SELECT * FROM products WHERE id = products:txn_widget", nil)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Empty(t, (*results)[0].Result, "Deleted product should not exist after commit")
}

// TestTransaction_CRUD_RPCs tests Create, Select, Update, Delete operations within a transaction using RPC methods.
func TestTransaction_CRUD_RPCs(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_crud_rpc", "crud_rpc_test", "products")

	db := mustNewWS("txn_crud_rpc", "crud_rpc_test", "products")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	// Note: we don't defer tx.Cancel here because we want to commit

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	// Use RecordID for RPC methods
	recordID := models.NewRecordID("products", "txn_rpc_widget")

	// Create using surrealdb.Create
	product, err := surrealdb.Create[Product](ctx, tx, recordID, map[string]any{
		"name":  "Transaction RPC Widget",
		"price": 29.99,
	})
	require.NoError(t, err, "Create within transaction should succeed")
	require.NotNil(t, product)
	assert.Equal(t, "Transaction RPC Widget", product.Name)
	assert.Equal(t, 29.99, product.Price)

	// Select using surrealdb.Select
	selectedProduct, err := surrealdb.Select[Product](ctx, tx, recordID)
	require.NoError(t, err, "Select within transaction should succeed")
	require.NotNil(t, selectedProduct)
	assert.Equal(t, "Transaction RPC Widget", selectedProduct.Name)

	// Upsert using surrealdb.Upsert
	// Note: In SurrealDB v3 transactions, the Update RPC may fail with
	// "Expected a single result output when using the ONLY keyword" even when
	// the record exists. Upsert works reliably in both session and transaction contexts.
	updatedProduct, err := surrealdb.Upsert[Product](ctx, tx, recordID, map[string]any{
		"name":  "Super Transaction RPC Widget",
		"price": 49.99,
	})
	require.NoError(t, err, "Upsert within transaction should succeed")
	require.NotNil(t, updatedProduct)
	assert.Equal(t, "Super Transaction RPC Widget", updatedProduct.Name)
	assert.Equal(t, 49.99, updatedProduct.Price)

	// Delete using surrealdb.Delete
	deletedProduct, err := surrealdb.Delete[Product](ctx, tx, recordID)
	require.NoError(t, err, "Delete within transaction should succeed")
	require.NotNil(t, deletedProduct)
	assert.Equal(t, "Super Transaction RPC Widget", deletedProduct.Name)

	// Commit
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify deleted after commit using surrealdb.Select
	verifyProduct, err := surrealdb.Select[Product](ctx, db, recordID)
	require.NoError(t, err, "Select of deleted record should not error")
	assert.Nil(t, verifyProduct, "Deleted product should not exist after commit")
}

// TestTransaction_QueryWithVariables tests Query with variables within a transaction.
func TestTransaction_QueryWithVariables(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support interactive transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_test", "query_vars_test", "users")

	db := mustNewWS("txn_test", "query_vars_test", "users")
	defer db.Close(context.Background())

	ctx := context.Background()

	tx, err := db.Begin(ctx)
	require.NoError(t, err)
	defer tx.Cancel(ctx)

	type User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// Create user with variables
	results, err := surrealdb.Query[[]User](ctx, tx,
		"CREATE users SET name = $name, age = $age",
		map[string]any{
			"name": "Alice",
			"age":  30,
		})
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Equal(t, "OK", (*results)[0].Status)
	assert.Len(t, (*results)[0].Result, 1)
	assert.Equal(t, "Alice", (*results)[0].Result[0].Name)
	assert.Equal(t, 30, (*results)[0].Result[0].Age)
}

// TestTransaction_InSession tests transactions started within a session.
func TestTransaction_InSession(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("Skipping: SurrealDB version %s does not support sessions/transactions (requires v3+)", v)
	}

	_ = mustNewWS("txn_session", "session_txn_test", "items")

	db := mustNewWS("txn_session", "session_txn_test", "items")
	defer db.Close(context.Background())

	ctx := context.Background()

	// Create a session
	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer session.Detach(ctx)

	// Authenticate and select namespace/database
	_, err = session.SignIn(ctx, map[string]any{"user": "root", "pass": "root"})
	require.NoError(t, err)

	err = session.Use(ctx, "txn_session", "session_txn_test")
	require.NoError(t, err)

	// Start a transaction in the session
	tx, err := session.Begin(ctx)
	require.NoError(t, err)
	require.NotNil(t, tx.SessionID(), "Transaction should have SessionID when started from session")
	assert.Equal(t, session.ID().String(), tx.SessionID().String())

	type Item struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}

	// Create item in transaction
	_, err = surrealdb.Query[[]Item](ctx, tx, "CREATE items:session_txn_item SET value = 'from session txn'", nil)
	require.NoError(t, err)

	// Commit
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Verify data persisted using the session
	results, err := surrealdb.Query[[]Item](ctx, session, "SELECT * FROM items WHERE id = items:session_txn_item", nil)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, *results, 1)
	assert.Len(t, (*results)[0].Result, 1)
	assert.Equal(t, "from session txn", (*results)[0].Result[0].Value)
}
