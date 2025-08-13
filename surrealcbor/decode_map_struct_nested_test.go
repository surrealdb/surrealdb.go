package surrealcbor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestDecode_map_structNested tests unmarshaling of complex nested structures
func TestDecode_map_structNested(t *testing.T) {
	type Address struct {
		Street  string  `json:"street"`
		City    string  `json:"city"`
		ZipCode *string `json:"zip_code"`
	}

	type Person struct {
		ID        models.RecordID  `json:"id"`
		Name      string           `json:"name"`
		Age       *int             `json:"age"`
		Email     *string          `json:"email"`
		Address   *Address         `json:"address"`
		Tags      []string         `json:"tags"`
		Metadata  map[string]any   `json:"metadata"`
		CreatedAt time.Time        `json:"created_at"`
		UpdatedAt *time.Time       `json:"updated_at"`
		DeletedAt models.CustomNil `json:"deleted_at"`
	}

	now := time.Now().Truncate(time.Second)
	age := 30
	email := "test@example.com"

	original := Person{
		ID:    models.NewRecordID("persons", "123"),
		Name:  "John Doe",
		Age:   &age,
		Email: &email,
		Address: &Address{
			Street:  "123 Main St",
			City:    "Springfield",
			ZipCode: nil, // This should become None
		},
		Tags: []string{"developer", "golang"},
		Metadata: map[string]any{
			"level":    "senior",
			"verified": true,
			"score":    95.5,
			"notes":    nil, // This should become None
		},
		CreatedAt: now,
		UpdatedAt: nil,         // This should become None
		DeletedAt: models.None, // Explicitly None
	}

	// Marshal the struct
	data, err := Marshal(original)
	require.NoError(t, err, "Marshal failed")

	// Unmarshal back
	var decoded Person
	err = Unmarshal(data, &decoded)
	require.NoError(t, err, "Unmarshal failed")

	// Verify the decoded struct
	assert.Equal(t, "persons", decoded.ID.Table, "ID.Table mismatch")
	assert.Equal(t, "123", decoded.ID.ID, "ID.ID mismatch")
	assert.Equal(t, original.Name, decoded.Name, "Name mismatch")

	require.NotNil(t, decoded.Age, "Age should not be nil")
	assert.Equal(t, *original.Age, *decoded.Age, "Age value mismatch")

	require.NotNil(t, decoded.Email, "Email should not be nil")
	assert.Equal(t, *original.Email, *decoded.Email, "Email value mismatch")

	require.NotNil(t, decoded.Address, "Address should not be nil")
	assert.Equal(t, original.Address.Street, decoded.Address.Street, "Street mismatch")
	assert.Equal(t, original.Address.City, decoded.Address.City, "City mismatch")
	assert.Nil(t, decoded.Address.ZipCode, "ZipCode should be nil")

	assert.Equal(t, original.Tags, decoded.Tags, "Tags mismatch")
	assert.Nil(t, decoded.UpdatedAt, "UpdatedAt should be nil")
}
