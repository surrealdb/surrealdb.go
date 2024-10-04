package connection

import (
	"bytes"
	"context"
	"encoding/hex"
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/v2/pkg/logger"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
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
	name                string
	connImplementations map[string]*HTTPConnection
	logBuffer           *bytes.Buffer
}

func TestHttpTestSuite(t *testing.T) {
	ts := new(HTTPTestSuite)
	ts.connImplementations = make(map[string]*HTTPConnection)

	// Default
	ts.connImplementations["http"] = NewHTTPConnection(NewConnectionParams{
		BaseURL:     "http://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})

	RunHTTPMap(t, ts)
}

func RunHTTPMap(t *testing.T, s *HTTPTestSuite) {
	for wsName := range s.connImplementations {
		// Run the test suite
		t.Run(wsName, func(t *testing.T) {
			s.name = wsName
			suite.Run(t, s)
		})
	}
}

// SetupSuite is called before the s starts running
func (s *HTTPTestSuite) SetupSuite() {
	con := s.connImplementations[s.name]

	err := con.Connect()
	s.Require().NoError(err)

	err = con.Use("test", "test")
	s.Require().NoError(err)

	err = con.Send(nil, "signin", map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	s.Require().NoError(err)
}

func (s *HTTPTestSuite) TearDownSuite() {
	con := s.connImplementations[s.name]
	err := con.Close()
	s.Require().NoError(err)
}

func (s *HTTPTestSuite) TestEngine_HttpMakeRequest() {
	p := NewConnectionParams{
		BaseURL:     "http://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
	}
	con := NewHTTPConnection(p)
	err := con.Use("test", "test")
	s.Require().NoError(err, "no error returned when setting namespace and database")

	err = con.Connect() // implement a "is ready"
	s.Require().NoError(err, "no error returned when initializing engine connection")

	var bearerRes RPCResponse[string]
	err = con.Send(&bearerRes, "signin", map[string]interface{}{"user": "root", "pass": "root"})
	s.Require().NoError(err, "no error returned when signing in")
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
