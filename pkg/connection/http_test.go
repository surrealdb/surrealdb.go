package connection

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

type HTTPTestSuite struct {
	suite.Suite
	name string
}

func TestHttpTestSuite(t *testing.T) {
	ts := new(HTTPTestSuite)
	ts.name = "HTTP Test Suite"

	suite.Run(t, ts)
}

// SetupSuite is called before the s starts running
func (s *HTTPTestSuite) SetupSuite() {

}

func (s *HTTPTestSuite) TearDownSuite() {

}

func (s *HTTPTestSuite) TestMockClientEngine_MakeRequest() {
	ctx := context.TODO()

	httpClient := NewTestClient(func(req *http.Request) *http.Response {
		s.Assert().Equal(req.URL.String(), "http://test.surreal/rpc")

		respBody, _ := hex.DecodeString("a26269647030484b6746566c657153427563625a44656572726f72a264636f6465186f676d6573736167657354686572652077617320612070726f626c656d")
		return &http.Response{
			StatusCode: 400,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewReader(respBody)),
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

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://test.surreal/rpc", http.NoBody)
	_, err := httpEngine.MakeRequest(req)
	s.Require().Error(err, "should return error for status code 400")
}
