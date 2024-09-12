package connection

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/model"
	"testing"
	"time"
)

func TestEngine_WsMakeRequest(t *testing.T) {
	p := NewConnectionParams{
		Marshaler:   model.CborMarshaler{},
		Unmarshaler: model.CborUnmashaler{},
		BaseURL:     "ws://127.0.0.1:8000",
	}
	con := NewWebSocket(p)

	err := con.Connect()
	assert.Nil(t, err, "no error returned when initializing engine connection")

	err = con.Use("test", "test")
	assert.Nil(t, err, "no error returned when setting namespace and database")

	token, err := con.Send("signin", []interface{}{model.Auth{Username: "pass", Password: "pass"}})
	assert.Nil(t, err, "no error returned when signing in")
	fmt.Println(token)

	params := []interface{}{
		"SELECT marketing, count() FROM $tb GROUP BY marketing",
		map[string]interface{}{
			"datetime": time.Now(),
			"testnil":  nil,
		},
	}
	res, err := con.Send("query", params)
	assert.Nil(t, err, "no error returned when sending a query")
	fmt.Println(res)
}
