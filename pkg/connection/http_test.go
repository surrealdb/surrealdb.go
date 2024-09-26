package connection

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"io"
	"net/http"
	"testing"
)

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestEngine_MakeRequest(t *testing.T) {
	httpClient := NewTestClient(func(req *http.Request) *http.Response {
		assert.Equal(t, req.URL.String(), "http://test.surreal/rpc")

		return &http.Response{
			StatusCode: 400,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBufferString(`OK`)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	})

	p := NewConnectionParams{
		BaseURL:     "http://test.surreal",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
	}
	httpEngine := NewHTTPConnection(p)
	httpEngine.SetHTTPClient(httpClient)

	req, _ := http.NewRequest(http.MethodGet, "http://test.surreal/rpc", http.NoBody)
	resp, err := httpEngine.MakeRequest(req)
	assert.Error(t, err, "should return error for status code 400")

	fmt.Println(resp)
}

type TestQueryResult struct {
	count     int64
	marketing string
}

type User struct {
	//ID      models.RecordID `json:"id,omitempty"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

func TestEngine_HttpMakeRequest(t *testing.T) {
	p := NewConnectionParams{
		BaseURL:     "http://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
	}
	con := NewHTTPConnection(p)
	err := con.Use("test", "test")
	assert.Nil(t, err, "no error returned when setting namespace and database")

	err = con.Connect() // implement a "is ready"
	assert.Nil(t, err, "no error returned when initializing engine connection")

	var bearer string
	//token, err := con.Send(&bearer, "signin", []interface{}{models.Auth{Username: "pass", Password: "pass"}})
	err = con.Send(&bearer, "signin", []interface{}{models.Auth{Username: "pass", Password: "pass"}})
	assert.Nil(t, err, "no error returned when signing in")
	//fmt.Println(token)
	//fmt.Println(bearer)

	err = con.Send(nil, "info")

	// Insert user
	//user := User{
	//	ID:      models.RecordID{ID: "343", Table: "user"},
	//	Name:    "John",
	//	Surname: "Doe",
	//}
	//
	//var resUser User
	//data, err := con.Send(&user, "create", []interface{}{"users", user})
	//fmt.Println(err)
	//fmt.Println(data)
	//fmt.Println(resUser)

	//params := []interface{}{
	//	"SELECT * FROM $t",
	//	map[string]interface{}{
	//		"tb": models.Table("users"),
	//	},
	//}

	var selectRes []User
	err = con.Send(&selectRes, "select", []interface{}{"users"})
	assert.Nil(t, err, "no error returned when sending a query")
	fmt.Println(err)
	fmt.Println(selectRes)
}
