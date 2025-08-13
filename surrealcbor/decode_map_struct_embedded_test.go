package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_map_structEmbedded tests unmarshaling with embedded structs
func TestDecode_map_structEmbedded(t *testing.T) {
	type Category struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}

	type Product struct {
		SKU   string  `json:"sku"`
		Title string  `json:"title"`
		Price float64 `json:"price"`
		Stock *int    `json:"stock"`
	}

	type CategoryWithProducts struct {
		Category
		Products []Product `json:"products"`
		Featured *bool     `json:"featured"`
	}

	t.Run("basic embedded struct", func(t *testing.T) {
		desc := "Electronics and gadgets"
		stock1 := 50
		stock2 := 30
		featured := true

		original := CategoryWithProducts{
			Category: Category{
				ID:          "cat-001",
				Name:        "Electronics",
				Description: &desc,
			},
			Products: []Product{
				{
					SKU:   "PROD-001",
					Title: "Laptop",
					Price: 999.99,
					Stock: &stock1,
				},
				{
					SKU:   "PROD-002",
					Title: "Mouse",
					Price: 29.99,
					Stock: &stock2,
				},
			},
			Featured: &featured,
		}

		data, err := Marshal(original)
		require.NoError(t, err, "Marshal failed")

		var decoded CategoryWithProducts
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Equal(t, original, decoded)
	})

	t.Run("embedded struct with nil fields", func(t *testing.T) {
		original := CategoryWithProducts{
			Category: Category{
				ID:          "cat-002",
				Name:        "Books",
				Description: nil,
			},
			Products: []Product{
				{
					SKU:   "BOOK-001",
					Title: "Go Programming",
					Price: 49.99,
					Stock: nil,
				},
			},
			Featured: nil,
		}

		data, err := Marshal(original)
		require.NoError(t, err, "Marshal failed")

		var decoded CategoryWithProducts
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Equal(t, original, decoded)
	})

	t.Run("multiple levels of embedding", func(t *testing.T) {
		type Base struct {
			BaseID   string `json:"base_id"`
			BaseName string `json:"base_name"`
		}

		type Middle struct {
			Base
			MiddleValue string `json:"middle_value"`
		}

		type Top struct {
			Middle
			TopValue string `json:"top_value"`
		}

		original := Top{
			Middle: Middle{
				Base: Base{
					BaseID:   "base-123",
					BaseName: "Base Name",
				},
				MiddleValue: "Middle Value",
			},
			TopValue: "Top Value",
		}

		data, err := Marshal(original)
		require.NoError(t, err, "Marshal failed")

		var decoded Top
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Equal(t, original, decoded)
	})

	t.Run("embedded struct with field name conflicts", func(t *testing.T) {
		// Define structs with potential field name conflicts
		type Inner struct {
			Value string `json:"value"`
			ID    string `json:"inner_id"`
		}

		type Outer struct {
			Inner
			Value string `json:"outer_value"` // Different json tag
			ID    string `json:"id"`          // Different json tag
		}

		original := Outer{
			Inner: Inner{
				Value: "inner value",
				ID:    "inner-123",
			},
			Value: "outer value",
			ID:    "outer-456",
		}

		data, err := Marshal(original)
		require.NoError(t, err, "Marshal failed")

		var decoded Outer
		err = Unmarshal(data, &decoded)
		require.NoError(t, err, "Unmarshal failed")

		assert.Equal(t, original, decoded)
	})
}
