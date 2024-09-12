package connection

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/pkg/model"
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

func TestEngine_HttpMakeRequest(t *testing.T) {
	//httpClient := NewTestClient(func(req *http.Request) *http.Response {
	//	assert.Equal(t, req.URL.String(), "http://test.surreal/rpc")
	//
	//	return &http.Response{
	//		StatusCode: 400,
	//		// Send response to be tested
	//		Body: ioutil.NopCloser(bytes.NewBufferString(`OK`)),
	//		// Must be set to non-nil value or it panics
	//		Header: make(http.Header),
	//	}
	//})
	//
	//httpEngine := (NewHttp(p)).(*Http)
	//httpEngine.SetHttpClient(httpClient)
	//
	//resp, err := httpEngine.MakeRequest(http.MethodGet, "http://test.surreal/rpc", nil)
	//assert.Error(t, err, "should return error for status code 400")
	//
	//fmt.Println(resp)

	p := NewConnectionParams{
		BaseURL:     "http://localhost:8000",
		Marshaler:   model.CborMarshaler{},
		Unmarshaler: model.CborUnmashaler{},
	}
	con := NewHttp(p)
	err := con.Use("test", "test")
	assert.Nil(t, err, "no error returned when setting namespace and database")

	con, err = con.Connect("http://127.0.0.1:8000")
	assert.Nil(t, err, "no error returned when initializing engine connection")

	token, err := con.SignIn(model.Auth{Username: "pass", Password: "pass"})
	assert.Nil(t, err, "no error returned when signing in")
	fmt.Println(token)

	params := []interface{}{
		"SELECT marketing, count() FROM $tb GROUP BY marketing",
		map[string]interface{}{
			"datetime": time.Now(),
			"testnil":  nil,
			//"duration": Duration(340),
		},
	}
	res, err := con.Send("query", params)
	assert.Nil(t, err, "no error returned when sending a query")
	fmt.Println(res)
}
