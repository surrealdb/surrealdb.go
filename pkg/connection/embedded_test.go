package connection

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"testing"
)

func TestEmbedded_SendRequest(t *testing.T) {
	con := NewEmbeddedConnection(NewConnectionParams{
		BaseURL:     "memory",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
	})

	err := con.Connect()
	assert.NoError(t, err)

	err = con.Use("test", "test")
	assert.NoError(t, err)

	var signInRes RPCResponse[string]
	err = con.Send(&signInRes, "signin", map[string]string{
		"user": "root",
		"pass": "root",
	})
	assert.NoError(t, err)
	fmt.Println(signInRes)

	//var res RPCResponse[any]
	//err = con.Send(&res, "info")
	//assert.NoError(t, err)
	//
	//fmt.Sprintf(res)
}
