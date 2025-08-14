package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/fxamacker/cbor/v2"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleCborUnmarshaler_DecOptions_defaultLimit demonstrates that the default
// CBOR decoder configuration works fine with small arrays that are well within
// the default limit of 131,072 elements.
func ExampleCborUnmarshaler_DecOptions_defaultLimit() {
	// Parse the SurrealDB WebSocket URL
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL: %v", err))
	}

	// Setup connection with default configuration
	conf := connection.NewConfig(u)
	conf.Logger = nil
	conn := gorillaws.New(conf)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect: %v", err))
	}
	defer db.Close(context.Background())

	err = db.Use(ctx, "example", "test")
	if err != nil {
		panic(fmt.Sprintf("Failed to use namespace/database: %v", err))
	}

	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	// Setup table and ensure it's clean before test
	tableName := "test_default_limit"
	setupTable(db, tableName)

	// Default settings work with small arrays
	createRecords(db, tableName, 10)
	selectRecords(db, tableName)

	// Output:
	// Table test_default_limit cleaned up
	// Successfully created record with 10 items
	// Successfully retrieved record with 10 items
}

// ExampleCborUnmarshaler_DecOptions_customSmallLimit demonstrates what happens
// when a custom MaxArrayElements limit is set too low and the actual data
// exceeds that limit. The unmarshal operation fails with a clear error message.
func ExampleCborUnmarshaler_DecOptions_customSmallLimit() {
	// Parse the SurrealDB WebSocket URL
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL: %v", err))
	}

	// First, create the record using default connection settings
	{
		conf := connection.NewConfig(u)
		conf.Logger = nil
		conn := gws.New(conf)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		db, err := surrealdb.FromConnection(ctx, conn)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect: %v", err))
		}
		defer db.Close(context.Background())

		err = db.Use(ctx, "example", "test")
		if err != nil {
			panic(fmt.Sprintf("Failed to use namespace/database: %v", err))
		}

		_, err = db.SignIn(ctx, surrealdb.Auth{
			Username: "root",
			Password: "root",
		})
		if err != nil {
			panic(fmt.Sprintf("SignIn failed: %v", err))
		}

		// Setup table and ensure it's clean before test
		tableName := "test_small_limit"
		setupTable(db, tableName)

		createRecords(db, tableName, 20)
	}

	// Now try to retrieve with a connection that has a small array limit
	{
		conf := connection.NewConfig(u)
		// Use TestLogHandler to see unmarshal errors but ignore debug and close errors
		handler := testenv.NewTestLogHandlerWithOptions(
			testenv.WithIgnoreErrorPrefixes("failed to close"),
			testenv.WithIgnoreDebug(),
		)
		conf.Logger = logger.New(handler)
		// Set a custom small limit that will be exceeded
		// Note: fxamacker/cbor requires MaxArrayElements to be at least 16
		conf.Unmarshaler = &models.CborUnmarshaler{
			DecOptions: cbor.DecOptions{
				MaxArrayElements: 16, // Set to minimum allowed value
			},
		}
		conn := gws.New(conf)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		db, err := surrealdb.FromConnection(ctx, conn)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect: %v", err))
		}
		defer db.Close(context.Background())

		err = db.Use(ctx, "example", "test")
		if err != nil {
			panic(fmt.Sprintf("Failed to use namespace/database: %v", err))
		}

		_, err = db.SignIn(ctx, surrealdb.Auth{
			Username: "root",
			Password: "root",
		})
		if err != nil {
			panic(fmt.Sprintf("SignIn failed: %v", err))
		}

		// This should fail due to array limit
		tableName := "test_small_limit"
		selectRecords(db, tableName)
	}

	// Output:
	// Table test_small_limit cleaned up
	// Successfully created record with 20 items
	// [0] ERROR: Failed to unmarshal response error=cbor: exceeded max number of elements 16 for CBOR array
	// Error retrieving record: context deadline exceeded
}

// ExampleCborUnmarshaler_DecOptions_customLargeLimit demonstrates how to
// configure a custom MaxArrayElements limit that is higher than needed,
// allowing successful retrieval of data that would fail with a smaller limit.
func ExampleCborUnmarshaler_DecOptions_customLargeLimit() {
	// Parse the SurrealDB WebSocket URL
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL: %v", err))
	}

	// Setup connection with custom CBOR unmarshaler that has a larger limit
	conf := connection.NewConfig(u)
	conf.Logger = nil
	// Set a custom larger limit that accommodates the data
	conf.Unmarshaler = &models.CborUnmarshaler{
		DecOptions: cbor.DecOptions{
			// Note that the default value is 131072.
			// We use smaller value just to make test run quickly.
			MaxArrayElements: 100,
		},
	}
	conn := gorillaws.New(conf)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	db, err := surrealdb.FromConnection(ctx, conn)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect: %v", err))
	}
	defer db.Close(context.Background())

	err = db.Use(ctx, "example", "test")
	if err != nil {
		panic(fmt.Sprintf("Failed to use namespace/database: %v", err))
	}

	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	// Setup table and ensure it's clean before test
	tableName := "test_large_limit"
	setupTable(db, tableName)

	createRecords(db, tableName, 20)

	// This should work with the larger limit
	selectRecords(db, tableName)

	// Output:
	// Table test_large_limit cleaned up
	// Successfully created record with 20 items
	// Successfully retrieved record with 20 items
}

// setupTable prepares a clean table for testing by deleting any existing records
func setupTable(db *surrealdb.DB, tableName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Clean up the table
	_, _ = surrealdb.Query[any](ctx, db, fmt.Sprintf("DELETE %s", tableName), nil)
	fmt.Printf("Table %s cleaned up\n", tableName)
}

// createRecords creates a test record in the specified table
func createRecords(db *surrealdb.DB, tableName string, arraySize int) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Create array with specified number of elements
	items := make([]string, arraySize)
	for i := range arraySize {
		items[i] = fmt.Sprintf("item_%d", i)
	}

	// Create the record
	_, err := surrealdb.Query[any](ctx, db, fmt.Sprintf("CREATE %s SET items = $items", tableName), map[string]any{
		"items": items,
	})

	if err != nil {
		fmt.Printf("Error creating record: %v\n", err)
	} else {
		fmt.Printf("Successfully created record with %d items\n", arraySize)
	}
}

func selectRecords(db *surrealdb.DB, tableName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Create test data structure
	type TestRecord struct {
		ID    any      `json:"id"`
		Items []string `json:"items"`
	}

	// Try to retrieve the record
	// This is where the MaxArrayElements limit will be enforced during decoding
	result, err := surrealdb.Query[[]TestRecord](ctx, db, fmt.Sprintf("SELECT * FROM %s", tableName), nil)

	if err != nil {
		// Log the error directly - it will be verified in the expected output
		fmt.Printf("Error retrieving record: %v\n", err)
		return
	}

	// Check if we got results
	if result != nil && len(*result) > 0 && len((*result)[0].Result) > 0 {
		recordCount := len((*result)[0].Result)
		if recordCount > 0 && (*result)[0].Result[0].Items != nil {
			fmt.Printf("Successfully retrieved record with %d items\n", len((*result)[0].Result[0].Items))
		}
	}
}
