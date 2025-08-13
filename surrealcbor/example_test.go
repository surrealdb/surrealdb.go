package surrealcbor_test

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// nolint: gocyclo
func Example_integration() {
	type Product struct {
		ID        *models.RecordID `json:"id,omitempty"`
		Name      string           `json:"name,omitempty"`
		Price     float64          `json:"price,omitempty"`
		CreatedAt time.Time        `json:"created_at,omitempty"`
		UpdatedAt *time.Time       `json:"updated_at,omitempty"`
	}

	id := models.NewRecordID("product", "123")

	createdAt, err := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	if err != nil {
		panic(err)
	}

	// Create a new product
	product := Product{
		ID:        &id,
		Name:      "Test Product",
		Price:     19.99,
		CreatedAt: createdAt,
		UpdatedAt: nil,
	}

	// Marshal the product to CBOR
	data, err := surrealcbor.Marshal(product)
	if err != nil {
		panic(err)
	}

	// Unmarshal the CBOR back to a product
	var decoded Product
	err = surrealcbor.Unmarshal(data, &decoded)
	if err != nil {
		panic(err)
	}

	// Verify the decoded product
	if decoded.ID == nil || *decoded.ID != *product.ID {
		panic("ID mismatch")
	}
	if decoded.Name != product.Name {
		panic("Name mismatch")
	}
	if decoded.Price != product.Price {
		panic("Price mismatch")
	}
	if decoded.CreatedAt != product.CreatedAt {
		panic("CreatedAt mismatch")
	}
	if decoded.UpdatedAt != nil {
		panic("UpdatedAt should be nil")
	}

	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL: %v", err))
	}

	conf := connection.NewConfig(u)
	conf.Logger = nil // Disable logging for this example
	codec := surrealcbor.New()
	conf.Marshaler = codec
	conf.Unmarshaler = codec

	conn := gws.New(conf)

	db, err := surrealdb.FromConnection(context.Background(), conn)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect: %v", err))
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if closeErr := db.Close(ctx); closeErr != nil {
			log.Printf("Failed to close database: %v", closeErr)
		}
	}()

	err = db.Use(context.Background(), "testNS", "testDB")
	if err != nil {
		fmt.Println("Use error:", err)
	}

	// Now let's try with the correct credentials
	// This should succeed if the database is set up correctly.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	_, err = surrealdb.Query[any](context.Background(), db, "REMOVE TABLE IF EXISTS product", nil)
	if err != nil {
		panic(fmt.Sprintf("Query failed: %v", err))
	}

	created, err := surrealdb.Query[[]Product](
		context.Background(),
		db,
		"CREATE product CONTENT $product",
		map[string]any{
			"product": product,
		})
	if err != nil {
		panic(fmt.Sprintf("Query failed: %v", err))
	}

	for _, p := range (*created)[0].Result {
		fmt.Printf("Created product: ID=%s, Name=%s, Price=%.2f, CreatedAt=%s\n",
			p.ID, p.Name, p.Price, p.CreatedAt.Format(time.RFC3339))
	}

	selected, err := surrealdb.Query[[]Product](
		context.Background(),
		db,
		"SELECT * FROM product",
		nil,
	)
	if err != nil {
		panic(fmt.Sprintf("Query failed: %v", err))
	}

	for _, p := range (*selected)[0].Result {
		fmt.Printf("Selected product: ID=%s, Name=%s, Price=%.2f, CreatedAt=%s\n",
			p.ID, p.Name, p.Price, p.CreatedAt.Format(time.RFC3339))
	}

	// Output:
	// Created product: ID=product:⟨123⟩, Name=Test Product, Price=19.99, CreatedAt=2023-10-01T12:00:00Z
	// Selected product: ID=product:⟨123⟩, Name=Test Product, Price=19.99, CreatedAt=2023-10-01T12:00:00Z
}
