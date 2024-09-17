package connection

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"io"
	"net/http"
	"testing"
	"time"
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

	token, err := con.Send("signin", []interface{}{models.Auth{Username: "pass", Password: "pass"}})
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
