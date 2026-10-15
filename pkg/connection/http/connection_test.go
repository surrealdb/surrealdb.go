package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

func TestLetUnset_keptApartFromConnectionState(t *testing.T) {
	c := surrealcbor.New()
	ok, err := c.Marshal(map[string]any{"id": "abc", "result": []any{}})
	if err != nil {
		t.Fatal(err)
	}

	var sent connection.RPCRequest
	var header http.Header
	con := New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: c, Unmarshaler: c})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		header = req.Header
		body, _ := io.ReadAll(req.Body)
		sent = connection.RPCRequest{}
		_ = c.Unmarshal(body, &sent)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(ok)), Header: http.Header{}}
	}))

	ctx := context.Background()
	require.NoError(t, con.Use(ctx, "ns", "db"))
	require.NoError(t, con.Authenticate(ctx, "the-token"))
	require.NoError(t, con.Let(ctx, "namespace", "elsewhere"))
	require.NoError(t, con.Let(ctx, "auth_token", "nope"))
	require.NoError(t, con.Let(ctx, "x", 1))

	_, err = con.Send(ctx, "query", "RETURN $x", map[string]any{"y": 2})
	require.NoError(t, err)

	assert.Equal(t, "ns", header.Get("Surreal-NS"))
	assert.Equal(t, "db", header.Get("Surreal-DB"))
	assert.Equal(t, "Bearer the-token", header.Get("Authorization"))
	require.Len(t, sent.Params, 2)
	vars, isMap := sent.Params[1].(map[string]any)
	require.True(t, isMap, "%T", sent.Params[1])
	assert.Equal(t, "elsewhere", vars["namespace"])
	assert.Equal(t, "nope", vars["auth_token"])
	assert.EqualValues(t, 1, vars["x"])
	assert.EqualValues(t, 2, vars["y"])

	require.NoError(t, con.Unset(ctx, "x"))
	_, err = con.Send(ctx, "query", "RETURN $x")
	require.NoError(t, err)
	vars, _ = sent.Params[1].(map[string]any)
	assert.NotContains(t, vars, "x")
	assert.Equal(t, "ns", header.Get("Surreal-NS"))

	// let, set, unset and use sent as raw RPCs change the same state, without a request.
	sent = connection.RPCRequest{}
	_, err = con.Send(ctx, "let", "z", 3)
	require.NoError(t, err)
	_, err = con.Send(ctx, "use", "ns2", "db2")
	require.NoError(t, err)
	assert.Empty(t, sent.Method, "no request is made for let or use")

	_, err = con.Send(ctx, "query", "RETURN $z")
	require.NoError(t, err)
	vars, _ = sent.Params[1].(map[string]any)
	assert.EqualValues(t, 3, vars["z"])
	assert.Equal(t, "ns2", header.Get("Surreal-NS"))

	// let, set and unset with missing parameters fail, as they do over a
	// WebSocket, and change nothing.
	for _, params := range [][]any{{}, {"w"}, {1, 2}, {"w", 1, 2}} {
		for _, method := range []string{"let", "set"} {
			_, err = con.Send(ctx, method, params...)
			var serverErr *connection.ServerError
			require.ErrorAs(t, err, &serverErr, "%s %v", method, params)
			assert.Equal(t, -32602, serverErr.Code)
			var rpcErr *connection.RPCError //nolint:staticcheck // callers still match the older error type
			require.ErrorAs(t, err, &rpcErr, "%s %v", method, params)
			assert.Equal(t, -32602, rpcErr.Code)
		}
	}
	for _, params := range [][]any{{}, {1}, {"z", "extra"}} {
		_, err = con.Send(ctx, "unset", params...)
		require.Error(t, err, "unset %v", params)
	}
	_, err = con.Send(ctx, "query", "RETURN $w")
	require.NoError(t, err)
	vars, _ = sent.Params[1].(map[string]any)
	assert.NotContains(t, vars, "w")
	assert.EqualValues(t, 3, vars["z"])

	// use with missing or non-string parameters fails and changes nothing.
	for _, params := range [][]any{{}, {"ns3"}, {1, 2}, {"ns3", 2}, {"ns3", "db3", "x"}} {
		_, err = con.Send(ctx, "use", params...)
		require.Error(t, err, "use %v", params)
	}
	_, err = con.Send(ctx, "query", "RETURN 1")
	require.NoError(t, err)
	assert.Equal(t, "ns2", header.Get("Surreal-NS"))

	// A value is kept as it was when it was bound, and one which cannot be
	// encoded is refused rather than breaking every later query.
	m := map[string]any{"a": 1}
	require.NoError(t, con.Let(ctx, "m", m))
	m["a"] = 2
	require.Error(t, con.Let(ctx, "bad", unencodable{}))
	_, err = con.Send(ctx, "query", "RETURN $m")
	require.NoError(t, err)
	vars, _ = sent.Params[1].(map[string]any)
	assert.EqualValues(t, map[string]any{"a": uint64(1)}, vars["m"])
	assert.NotContains(t, vars, "bad")

	// A canceled context changes nothing.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = con.Send(canceled, "use", "ns4", "db4")
	require.ErrorIs(t, err, context.Canceled)
	_, err = con.Send(canceled, "let", "c", 1)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, con.Let(canceled, "c", 1), context.Canceled)
	require.ErrorIs(t, con.Unset(canceled, "z"), context.Canceled)
	_, err = con.Send(ctx, "query", "RETURN 1")
	require.NoError(t, err)
	assert.Equal(t, "ns2", header.Get("Surreal-NS"))
	vars, _ = sent.Params[1].(map[string]any)
	assert.NotContains(t, vars, "c")
	assert.Contains(t, vars, "z")

	// invalidate forgets the token.
	_, err = con.Send(ctx, "invalidate")
	require.NoError(t, err)
	_, err = con.Send(ctx, "query", "RETURN 1")
	require.NoError(t, err)
	assert.Empty(t, header.Get("Authorization"))
}

type unencodable struct{}

func (unencodable) MarshalCBOR() ([]byte, error) { return nil, errors.New("cannot be encoded") }

// The token of a sign-in which returns a token pair is kept whatever map type
// the unmarshaler decodes objects to.
func TestTrackAuth_tokenPairWithACustomMapType(t *testing.T) {
	c := surrealcbor.New()
	pair, err := c.Marshal(map[string]any{"id": "abc", "result": map[string]any{"access": "the-access", "refresh": "the-refresh"}})
	require.NoError(t, err)
	ok, err := c.Marshal(map[string]any{"id": "abc", "result": []any{}})
	require.NoError(t, err)

	var header http.Header
	//nolint:staticcheck // the case reported is the deprecated fxamacker unmarshaler with a custom map type
	unmarshaler := &models.CborUnmarshaler{DecOptions: cbor.DecOptions{DefaultMapType: reflect.TypeOf(map[string]string(nil))}}
	con := New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: c, Unmarshaler: unmarshaler})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		header = req.Header
		var sent connection.RPCRequest
		body, _ := io.ReadAll(req.Body)
		_ = c.Unmarshal(body, &sent)
		res := ok
		if sent.Method == "signin" {
			res = pair
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(res)), Header: http.Header{}}
	}))

	ctx := context.Background()
	_, err = con.Send(ctx, "signin", map[string]any{"user": "u", "pass": "p"})
	require.NoError(t, err)
	_, err = con.Send(ctx, "query", "RETURN 1")
	require.NoError(t, err)
	assert.Equal(t, "Bearer the-access", header.Get("Authorization"))
}

// TestRefreshAndReset_changeConnectionState checks that the token of a
// refreshed pair is sent from then on, and that reset forgets everything
// without a request.
func TestRefreshAndReset_changeConnectionState(t *testing.T) {
	c := surrealcbor.New()

	var sent connection.RPCRequest
	var header http.Header
	con := New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: c, Unmarshaler: c})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		header = req.Header
		body, _ := io.ReadAll(req.Body)
		sent = connection.RPCRequest{}
		_ = c.Unmarshal(body, &sent)

		var result any = []any{}
		if sent.Method == "refresh" {
			result = map[string]any{"access": "new-access", "refresh": "new-refresh"}
		}
		res, _ := c.Marshal(map[string]any{"id": sent.ID, "result": result})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(res)), Header: http.Header{}}
	}))

	ctx := context.Background()
	require.NoError(t, con.Use(ctx, "ns", "db"))
	require.NoError(t, con.Let(ctx, "x", 1))
	require.NoError(t, con.Authenticate(ctx, "old-access"))

	_, err := con.Send(ctx, "refresh", connection.Tokens{Access: "old-access", Refresh: "old-refresh"})
	require.NoError(t, err)
	_, err = con.Send(ctx, "query", "RETURN 1")
	require.NoError(t, err)
	assert.Equal(t, "Bearer new-access", header.Get("Authorization"))

	sent = connection.RPCRequest{}
	_, err = con.Send(ctx, "reset")
	require.NoError(t, err)
	assert.Empty(t, sent.Method, "reset needs no request")

	_, err = con.Send(ctx, "query", "RETURN $x")
	require.NoError(t, err)
	assert.Empty(t, header.Get("Authorization"))
	assert.Empty(t, header.Get("Surreal-NS"))
	assert.Empty(t, header.Get("Surreal-DB"))
	assert.Len(t, sent.Params, 1, "no variables are sent")
}
