package connection

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"testing"
)

func TestEmbedded_SendRequest(t *testing.T) {
	con := NewEmbeddedConnection(NewConnectionParams{
		BaseURL:     "ws://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
	})

	err := con.Connect()
	fmt.Println(err)

	err = con.Use("test", "test")
	fmt.Println(err)

}
