package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
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

		respBody, _ := base64.StdEncoding.DecodeString("omJpZHAwSEtnRlZsZXFTQnVjYlpEZWVycm9yomRjb2RlGG9nbWVzc2FnZXNUaGVyZSB3YXMgYSBwcm9ibGVt")
		return &http.Response{
			StatusCode: 400,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewReader(respBody)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	})

	c := surrealcbor.New()
	p := &connection.Config{
		BaseURL:     "http://test.surreal",
		Marshaler:   c,
		Unmarshaler: c,
	}

	httpEngine := New(p)
	httpEngine.SetHTTPClient(httpClient)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://test.surreal/rpc", http.NoBody)
	_, err := httpEngine.MakeRequest(req)
	s.Require().Error(err, "should return error for status code 400")
}

func TestCall_withoutUse(t *testing.T) {
	c := surrealcbor.New()
	body, err := c.Marshal(map[string]any{"id": "abc", "result": "surrealdb-3.0.0"})
	if err != nil {
		t.Fatal(err)
	}

	var gotNS, gotDB []string
	con := New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: c, Unmarshaler: c})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		gotNS = req.Header.Values("Surreal-NS")
		gotDB = req.Header.Values("Surreal-DB")
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}
	}))

	_, err = con.Send(context.Background(), "version")
	if err != nil {
		t.Fatalf("version before Use: %v", err)
	}
	if len(gotNS) != 0 || len(gotDB) != 0 {
		t.Fatalf("expected no namespace or database headers, got %v %v", gotNS, gotDB)
	}

	if err := con.Use(context.Background(), "ns", "db"); err != nil {
		t.Fatal(err)
	}
	if _, err := con.Send(context.Background(), "version"); err != nil {
		t.Fatal(err)
	}
	if len(gotNS) != 1 || gotNS[0] != "ns" || len(gotDB) != 1 || gotDB[0] != "db" {
		t.Fatalf("expected namespace and database headers, got %v %v", gotNS, gotDB)
	}
}
