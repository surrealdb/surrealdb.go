package example

import (
	"encoding/json"
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Person is a Go struct that represents an example
// database record for a person.
//
// Note that this SDK uses CBOR for serialization, although
// struct fields (can) have `json` tags.
// If you want to be more specific, you should use `cbor` tags instead.
// `json` tags work only because the our CBOR implementation supports
// both types of tags for your convenience.
type Person struct {
	// ID is the unique identifier for the person record.
	//
	// Any SurrealDB record has ID.
	// The SurrealDB Go SDK uses models.RecordID to represent record IDs.
	ID *models.RecordID `json:"id,omitempty"`

	// Name is the person's name.
	//
	// Many Go primitive types that can be serialized using CBOR
	// are supported.
	Name    string `json:"name"`
	Surname string `json:"surname"`

	// Location is the person's location.
	//
	// Some SurrealDB-specific data types require custom structs
	// provided by this SDK.
	Location models.GeometryPoint `json:"location"`
}

type CustomRecordID struct {
	models.RecordID
}

func (r CustomRecordID) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%s:%v", r.Table, r.ID))
}

type PersonWithCustomID struct {
	ID       CustomRecordID       `json:"id,omitempty"`
	Name     string               `json:"name"`
	Surname  string               `json:"surname"`
	Location models.GeometryPoint `json:"location"`
}
