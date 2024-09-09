package connection

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
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
			Body: ioutil.NopCloser(bytes.NewBufferString(`OK`)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	})

	p := NewConnectionParams{}
	httpEngine := (NewHttp(p)).(*Http)
	httpEngine.SetHttpClient(httpClient)

	resp, err := httpEngine.MakeRequest(http.MethodGet, "http://test.surreal/rpc", nil)
	assert.Error(t, err, "should return error for status code 400")

	fmt.Println(resp)
}
