package surrealrestore_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealdump"
	"github.com/surrealdb/surrealdb.go/contrib/surrealrestore"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// Product represents a product in our test database
type Product struct {
	ID          string    `json:"id,omitempty"`
	Name        string    `json:"name"`
	Price       float64   `json:"price"`
	Category    string    `json:"category"`
	InStock     bool      `json:"in_stock"`
	LastUpdated time.Time `json:"last_updated"`
}

// Order represents an order in our test database
type Order struct {
	ID         string    `json:"id,omitempty"`
	ProductID  string    `json:"product_id"`
	Quantity   int       `json:"quantity"`
	Total      float64   `json:"total"`
	OrderDate  time.Time `json:"order_date"`
	CustomerID string    `json:"customer_id"`
}

func setupTestDB(t *testing.T) (*surrealdb.DB, context.Context) {
	ctx := context.Background()

	// Setup connection with surrealcbor
	conf := connection.NewConfig(testenv.MustParseSurrealDBWSURL())
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec
	conf.Logger = nil

	// Use gws connection
	conn := gws.New(conf)
	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	return db, ctx
}

//nolint:gocyclo // End-to-end test requires comprehensive validation
func TestDumpAndRestore(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close(ctx)

	// Initialize source database
	sourceDB, err := testenv.Init(db, "e2e_test", "source_db", "products", "orders")
	if err != nil {
		t.Fatalf("Failed to init source db: %v", err)
	}

	// Create test data
	products := []Product{
		{Name: "Laptop", Price: 999.99, Category: "Electronics", InStock: true, LastUpdated: time.Now()},
		{Name: "Mouse", Price: 29.99, Category: "Electronics", InStock: true, LastUpdated: time.Now()},
		{Name: "Keyboard", Price: 79.99, Category: "Electronics", InStock: false, LastUpdated: time.Now()},
		{Name: "Monitor", Price: 299.99, Category: "Electronics", InStock: true, LastUpdated: time.Now()},
		{Name: "Desk", Price: 499.99, Category: "Furniture", InStock: true, LastUpdated: time.Now()},
	}

	var insertedProducts []Product
	for _, p := range products {
		result, insertErr := surrealdb.Insert[Product](ctx, sourceDB, "products", p)
		if insertErr != nil {
			t.Fatalf("Failed to insert product: %v", insertErr)
		}
		if result != nil && len(*result) > 0 {
			insertedProducts = append(insertedProducts, (*result)[0])
		}
	}

	// Create orders referencing products
	orders := []Order{
		{ProductID: insertedProducts[0].ID, Quantity: 2, Total: 1999.98, OrderDate: time.Now(), CustomerID: "customer:1"},
		{ProductID: insertedProducts[1].ID, Quantity: 5, Total: 149.95, OrderDate: time.Now(), CustomerID: "customer:2"},
		{ProductID: insertedProducts[2].ID, Quantity: 1, Total: 79.99, OrderDate: time.Now(), CustomerID: "customer:1"},
	}

	for _, o := range orders {
		_, orderErr := surrealdb.Insert[Order](ctx, sourceDB, "orders", o)
		if orderErr != nil {
			t.Fatalf("Failed to insert order: %v", orderErr)
		}
	}

	// Perform full dump (current namespace/database was set by testenv.Init)
	tempDir := t.TempDir()
	dumpPath := filepath.Join(tempDir, "dump.cbor")
	dumper := surrealdump.New(sourceDB, "e2e_test", "source_db")
	if fullErr := dumper.Full(ctx, dumpPath); fullErr != nil {
		t.Fatalf("Full dump failed: %v", fullErr)
	}

	// Check dump size for logging
	if fileInfo, statErr := os.Stat(dumpPath); statErr == nil {
		t.Logf("Full dump size: %d bytes", fileInfo.Size())
	}

	// Create a new connection for target database
	targetDB, ctx2 := setupTestDB(t)
	defer targetDB.Close(ctx2)

	// Initialize target database (different namespace/database to ensure it's clean)
	targetDB, err = testenv.Init(targetDB, "e2e_restore_test", "restored_db")
	if err != nil {
		t.Fatalf("Failed to init target db: %v", err)
	}

	// Clean any existing data in the namespace we're about to restore
	if useErr := targetDB.Use(ctx2, "e2e_test", "source_db"); useErr == nil {
		// If it exists, clean all tables
		targetDB, _ = testenv.Init(targetDB, "e2e_test", "source_db")
	}

	// Restore from dump file - this is the new simpler API
	restorer := surrealrestore.New(targetDB)

	if restoreErr := restorer.Full(ctx2, dumpPath); restoreErr != nil {
		t.Fatalf("Full restore failed: %v", restoreErr)
	}

	stats := restorer.Stats()
	t.Logf("Restore stats: Records=%d, Tables=%d", stats.RecordsRestored, stats.TablesRestored)

	// Verify restored data with SELECT queries
	// Switch to restored namespace/database
	if switchErr := targetDB.Use(ctx2, "e2e_test", "source_db"); switchErr != nil {
		t.Fatalf("Failed to use restored db: %v", switchErr)
	}

	// Query 1: Count products
	type CountResult struct {
		Count int `json:"count"`
	}
	countResult, err := surrealdb.Query[[]CountResult](ctx2, targetDB, "SELECT count() as count FROM products GROUP ALL", nil)
	if err != nil {
		t.Fatalf("Failed to count products: %v", err)
	}
	if len(*countResult) > 0 && len((*countResult)[0].Result) > 0 {
		count := (*countResult)[0].Result[0].Count
		if count != len(products) {
			t.Errorf("Product count mismatch: expected %d, got %d", len(products), count)
		}
	} else {
		t.Error("No count result returned")
	}

	// Query 2: Select all products
	productsResult, err := surrealdb.Query[[]Product](ctx2, targetDB, "SELECT * FROM products", nil)
	if err != nil {
		t.Fatalf("Failed to select products: %v", err)
	}
	if len(*productsResult) > 0 && len((*productsResult)[0].Result) != len(products) {
		t.Errorf("Products mismatch: expected %d, got %d", len(products), len((*productsResult)[0].Result))
	}

	// Query 3: Check specific product
	laptopResult, err := surrealdb.Query[[]Product](ctx2, targetDB,
		"SELECT * FROM products WHERE name = 'Laptop'", nil)
	if err != nil {
		t.Fatalf("Failed to query laptop: %v", err)
	}
	if len(*laptopResult) > 0 && len((*laptopResult)[0].Result) > 0 {
		laptop := (*laptopResult)[0].Result[0]
		if laptop.Price != 999.99 {
			t.Errorf("Laptop price mismatch: expected 999.99, got %f", laptop.Price)
		}
		if laptop.Category != "Electronics" {
			t.Errorf("Laptop category mismatch: expected Electronics, got %s", laptop.Category)
		}
	}

	// Query 4: Count orders
	orderCountResult, err := surrealdb.Query[[]CountResult](ctx2, targetDB, "SELECT count() as count FROM orders GROUP ALL", nil)
	if err != nil {
		t.Fatalf("Failed to count orders: %v", err)
	}
	if len(*orderCountResult) > 0 && len((*orderCountResult)[0].Result) > 0 {
		count := (*orderCountResult)[0].Result[0].Count
		if count != len(orders) {
			t.Errorf("Order count mismatch: expected %d, got %d", len(orders), count)
		}
	}

	// Query 5: Check relationships
	orderResult, err := surrealdb.Query[[]Order](ctx2, targetDB,
		"SELECT * FROM orders WHERE customer_id = 'customer:1'", nil)
	if err != nil {
		t.Fatalf("Failed to query customer orders: %v", err)
	}
	if len(*orderResult) > 0 && len((*orderResult)[0].Result) != 2 {
		t.Errorf("Customer 1 orders mismatch: expected 2, got %d", len((*orderResult)[0].Result))
	}

	// Query 6: Aggregate query
	type CategoryCount struct {
		Category string `json:"category"`
		Count    int    `json:"count"`
	}
	aggResult, err := surrealdb.Query[[]CategoryCount](ctx2, targetDB,
		"SELECT category, count() as count FROM products GROUP BY category", nil)
	if err != nil {
		t.Fatalf("Failed to run aggregate query: %v", err)
	}
	if len(*aggResult) > 0 {
		for _, cc := range (*aggResult)[0].Result {
			t.Logf("Category %s has %d products", cc.Category, cc.Count)
			if cc.Category == "Electronics" && cc.Count != 4 {
				t.Errorf("Electronics count mismatch: expected 4, got %d", cc.Count)
			}
			if cc.Category == "Furniture" && cc.Count != 1 {
				t.Errorf("Furniture count mismatch: expected 1, got %d", cc.Count)
			}
		}
	}
}

//nolint:gocyclo // End-to-end test requires comprehensive validation
func TestE2E_IncrementalDumpAndRestore(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close(ctx)

	// Initialize database with change feed
	sourceDB, err := testenv.Init(db, "e2e_incr", "source_db", "products")
	if err != nil {
		t.Fatalf("Failed to init source db: %v", err)
	}

	// Enable change feed
	_, err = surrealdb.Query[any](ctx, sourceDB, "DEFINE TABLE products CHANGEFEED 1h", nil)
	if err != nil {
		t.Fatalf("Failed to enable change feed: %v", err)
	}

	// Initial data
	initialProducts := []Product{
		{Name: "Phone", Price: 599.99, Category: "Electronics", InStock: true, LastUpdated: time.Now()},
		{Name: "Tablet", Price: 399.99, Category: "Electronics", InStock: true, LastUpdated: time.Now()},
	}

	for _, p := range initialProducts {
		_, insertErr := surrealdb.Insert[Product](ctx, sourceDB, "products", p)
		if insertErr != nil {
			t.Fatalf("Failed to insert product: %v", insertErr)
		}
	}

	// Perform initial full dump (current namespace/database was set by testenv.Init)
	tempDir := t.TempDir()
	fullDumpPath := filepath.Join(tempDir, "full.cbor")
	dumper := surrealdump.New(sourceDB, "e2e_incr", "source_db")
	if fullErr := dumper.Full(ctx, fullDumpPath); fullErr != nil {
		t.Fatalf("Full dump failed: %v", fullErr)
	}

	// Check what changes are in the full dump
	t.Logf("Full dump completed, checking for changes included in dump...")

	// Get the versionstamp from full dump manifest
	fullDumpData, err := os.ReadFile(fullDumpPath)
	if err != nil {
		t.Fatalf("Failed to read full dump: %v", err)
	}
	manifest, err := surrealdump.ReadManifest(fullDumpPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}
	baseVersionstamp := manifest.EndVersionstamp
	t.Logf("Base versionstamp from full dump: %d", baseVersionstamp)
	t.Logf("Full dump start versionstamp: %d, end versionstamp: %d", manifest.StartVersionstamp, manifest.EndVersionstamp)

	// Wait a moment to ensure change feed is ready
	time.Sleep(100 * time.Millisecond)

	// Make changes after the full dump
	// Update a product
	_, err = surrealdb.Query[any](ctx, sourceDB,
		"UPDATE products SET price = 549.99 WHERE name = 'Phone'", nil)
	if err != nil {
		t.Fatalf("Failed to update product: %v", err)
	}

	// Add a new product
	newProduct := Product{
		Name: "Smartwatch", Price: 299.99, Category: "Electronics", InStock: true, LastUpdated: time.Now(),
	}
	_, err = surrealdb.Insert[Product](ctx, sourceDB, "products", newProduct)
	if err != nil {
		t.Fatalf("Failed to insert new product: %v", err)
	}

	// Delete a product
	_, err = surrealdb.Query[any](ctx, sourceDB,
		"DELETE products WHERE name = 'Tablet'", nil)
	if err != nil {
		t.Fatalf("Failed to delete product: %v", err)
	}

	// Wait to ensure changes are committed
	time.Sleep(100 * time.Millisecond)

	// Get current versionstamp after making changes
	currentVs, _ := surrealdb.Query[[]map[string]any](ctx, sourceDB,
		"SHOW CHANGES FOR TABLE products SINCE 0 LIMIT 1", nil)
	if currentVs != nil && len(*currentVs) > 0 && len((*currentVs)[0].Result) > 0 {
		if vs, ok := (*currentVs)[0].Result[0]["versionstamp"].(float64); ok {
			t.Logf("Current versionstamp after changes: %d", uint64(vs))
		}
	}

	// Use a slightly earlier versionstamp to ensure overlap
	// This is safe as overlapping changes will be replayed correctly
	incrStartVersionstamp := baseVersionstamp
	if baseVersionstamp > 0 {
		// Go back slightly to ensure we capture all changes
		incrStartVersionstamp = baseVersionstamp - 1
	}

	// Create incremental dump
	incrDumpPath := filepath.Join(tempDir, "incr.cbor")
	t.Logf("Creating incremental dump starting from versionstamp: %d", incrStartVersionstamp)
	if incrErr := dumper.Incremental(ctx, incrDumpPath, incrStartVersionstamp); incrErr != nil {
		t.Fatalf("Incremental dump failed: %v", incrErr)
	}

	incrDumpData, err := os.ReadFile(incrDumpPath)
	if err != nil {
		t.Fatalf("Failed to read incremental dump: %v", err)
	}

	t.Logf("Full dump size: %d bytes", len(fullDumpData))
	t.Logf("Incremental dump size: %d bytes", len(incrDumpData))

	// Check what's in the incremental dump
	t.Logf("Checking for changes in products table since versionstamp %d", incrStartVersionstamp)
	result, err := surrealdb.Query[[]map[string]any](ctx, sourceDB,
		fmt.Sprintf("SHOW CHANGES FOR TABLE products SINCE %d", incrStartVersionstamp), nil)
	if err != nil {
		t.Logf("Error getting changes: %v", err)
	} else if result != nil && len(*result) > 0 {
		t.Logf("Changes found in table: %d entries", len((*result)[0].Result))
		for i, change := range (*result)[0].Result {
			if i < 5 { // Log first 5 changes
				t.Logf("  Change %d: %v", i, change)
			}
		}
	} else {
		t.Logf("No changes found in table")
	}

	// Create target database and restore
	targetDB, ctx2 := setupTestDB(t)
	defer targetDB.Close(ctx2)

	targetDB, err = testenv.Init(targetDB, "e2e_incr_restored", "target_db")
	if err != nil {
		t.Fatalf("Failed to init target db: %v", err)
	}

	// Clean any existing data in the namespace we're about to restore
	if useErr := targetDB.Use(ctx2, "e2e_incr", "source_db"); useErr == nil {
		// If it exists, clean all tables
		targetDB, _ = testenv.Init(targetDB, "e2e_incr", "source_db")
	}

	// First restore the full dump using the simplified API
	fullRestorer := surrealrestore.New(targetDB)
	if fullRestoreErr := fullRestorer.Full(ctx2, fullDumpPath); fullRestoreErr != nil {
		t.Fatalf("Full restore failed: %v", fullRestoreErr)
	}

	// Then apply incremental changes using the simplified API
	incrRestorer := surrealrestore.New(targetDB)
	incrRestorer.Verbose = true // Enable verbose to see what's happening
	if incrRestoreErr := incrRestorer.Incremental(ctx2, incrDumpPath); incrRestoreErr != nil {
		t.Fatalf("Incremental restore failed: %v", incrRestoreErr)
	}

	t.Logf("Changes applied: %d", incrRestorer.Stats().ChangesApplied)

	// Verify final state with SELECT queries
	if switchErr := targetDB.Use(ctx2, "e2e_incr", "source_db"); switchErr != nil {
		t.Fatalf("Failed to use restored db: %v", switchErr)
	}

	// Query 1: Count should be 2 (initial 2 + 1 new - 1 deleted)
	type CountResult struct {
		Count int `json:"count"`
	}
	countResult, err := surrealdb.Query[[]CountResult](ctx2, targetDB,
		"SELECT count() as count FROM products GROUP ALL", nil)
	if err != nil {
		t.Fatalf("Failed to count products: %v", err)
	}
	if len(*countResult) > 0 && len((*countResult)[0].Result) > 0 {
		count := (*countResult)[0].Result[0].Count
		if count != 2 {
			t.Errorf("Product count after incremental: expected 2, got %d", count)
		}
	}

	// Query 2: Phone should have updated price
	phoneResult, err := surrealdb.Query[[]Product](ctx2, targetDB,
		"SELECT * FROM products WHERE name = 'Phone'", nil)
	if err != nil {
		t.Fatalf("Failed to query phone: %v", err)
	}
	if len(*phoneResult) > 0 {
		t.Logf("Found %d phone records", len((*phoneResult)[0].Result))
		for _, phone := range (*phoneResult)[0].Result {
			t.Logf("  Phone ID: %s, Price: %.2f", phone.ID, phone.Price)
		}
		if len((*phoneResult)[0].Result) > 0 {
			phone := (*phoneResult)[0].Result[0]
			if phone.Price != 549.99 {
				t.Errorf("Phone price not updated: expected 549.99, got %f", phone.Price)
			}
		}
	}

	// Query 3: Tablet should be deleted
	tabletResult, err := surrealdb.Query[[]Product](ctx2, targetDB,
		"SELECT * FROM products WHERE name = 'Tablet'", nil)
	if err != nil {
		t.Fatalf("Failed to query tablet: %v", err)
	}
	if len(*tabletResult) > 0 && len((*tabletResult)[0].Result) > 0 {
		t.Error("Tablet should have been deleted")
	}

	// Query 4: Smartwatch should exist
	watchResult, err := surrealdb.Query[[]Product](ctx2, targetDB,
		"SELECT * FROM products WHERE name = 'Smartwatch'", nil)
	if err != nil {
		t.Fatalf("Failed to query smartwatch: %v", err)
	}
	if len(*watchResult) > 0 && len((*watchResult)[0].Result) != 1 {
		t.Error("Smartwatch should exist after incremental restore")
	}

	// Query 5: List all products
	allProducts, err := surrealdb.Query[[]Product](ctx2, targetDB,
		"SELECT name, price FROM products ORDER BY name", nil)
	if err != nil {
		t.Fatalf("Failed to list all products: %v", err)
	}
	if len(*allProducts) > 0 {
		t.Log("Final products after incremental restore:")
		for _, p := range (*allProducts)[0].Result {
			t.Logf("  - %s: $%.2f", p.Name, p.Price)
		}
	}
}
