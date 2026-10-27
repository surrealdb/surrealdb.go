package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newStaticResponseConnection(status int, contentType, body string) *Connection {
	c := surrealcbor.New()
	con := New(&connection.Config{
		BaseURL:     "http://test.surreal",
		Marshaler:   c,
		Unmarshaler: c,
	})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		header := http.Header{}
		if contentType != "" {
			header.Set("Content-Type", contentType)
		}
		return &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     header,
		}
	}))
	return con
}

// TestMakeRequest_nonCBORErrorBody checks that a non-2xx response whose body is
// not CBOR, such as an HTML page from a proxy, is returned as an error rather
// than panicking.
func TestMakeRequest_nonCBORErrorBody(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{"html from a proxy", "text/html; charset=utf-8", "<html><body>Bad Gateway</body></html>"},
		{"plain text", "text/plain", "There was a problem with the database"},
		{"claims cbor but is not", "application/cbor", "not cbor at all"},
		{"no content type", "", "Bad Gateway"},
		{"empty body", "application/cbor", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			con := newStaticResponseConnection(http.StatusBadGateway, tt.contentType, tt.body)
			require.NoError(t, con.Use(context.Background(), "ns", "db"))

			var err error
			require.NotPanics(t, func() {
				_, err = con.Send(context.Background(), "version")
			})

			var statusErr *StatusError
			require.ErrorAs(t, err, &statusErr)
			assert.Equal(t, http.StatusBadGateway, statusErr.StatusCode)
			assert.Equal(t, tt.body, statusErr.Body)
			assert.Contains(t, err.Error(), "502 Bad Gateway")
		})
	}
}

func TestMakeRequest_cborErrorBody(t *testing.T) {
	c := surrealcbor.New()
	body, err := c.Marshal(map[string]any{
		"id":    "abc",
		"error": map[string]any{"code": -32000, "message": "There was a problem with authentication"},
	})
	require.NoError(t, err)

	con := newStaticResponseConnection(http.StatusUnauthorized, "application/cbor", string(body))
	require.NoError(t, con.Use(context.Background(), "ns", "db"))

	_, err = con.Send(context.Background(), "version")

	var serverErr *connection.ServerError
	require.ErrorAs(t, err, &serverErr)
	assert.Equal(t, "There was a problem with authentication", serverErr.Message)
}
