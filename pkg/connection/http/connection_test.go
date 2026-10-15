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

	"github.com/surrealdb/surrealdb.go/internal/codec"
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

// recordingConnection returns a connection to a fake server which answers
// every request with an empty result, and the last request and headers it got.
func recordingConnection(t *testing.T, marshaler codec.Marshaler) (*Connection, *connection.RPCRequest, *http.Header) {
	t.Helper()
	c := surrealcbor.New()
	ok, err := c.Marshal(map[string]any{"id": "abc", "result": []any{}})
	require.NoError(t, err)

	sent := &connection.RPCRequest{}
	header := &http.Header{}
	con := New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: marshaler, Unmarshaler: c})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		*header = req.Header
		body, _ := io.ReadAll(req.Body)
		*sent = connection.RPCRequest{}
		_ = c.Unmarshal(body, sent)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(ok)), Header: http.Header{}}
	}))
	return con, sent, header
}

// A raw reset forgets the namespace, database, token and variables the
// connection keeps, without a request.
func TestReset_clearsConnectionState(t *testing.T) {
	for _, params := range [][]any{nil, {"ignored"}} {
		con, sent, header := recordingConnection(t, surrealcbor.New())

		ctx := context.Background()
		require.NoError(t, con.Use(ctx, "ns", "db"))
		require.NoError(t, con.Let(ctx, "x", 1))
		require.NoError(t, con.Authenticate(ctx, "the-token"))

		*sent = connection.RPCRequest{}
		_, err := con.Send(ctx, "reset", params...)
		require.NoError(t, err)
		assert.Empty(t, sent.Method, "reset needs no request")

		_, err = con.Send(ctx, "query", "RETURN $x")
		require.NoError(t, err)
		assert.Empty(t, header.Get("Authorization"))
		assert.Empty(t, header.Get("Surreal-NS"))
		assert.Empty(t, header.Get("Surreal-DB"))
		assert.Len(t, sent.Params, 1, "no variables are sent")
	}
}

// use takes, for the namespace and for the database, a string, nil to unset
// it, or NONE to leave it as it is.
func TestUse_noneLeavesScopeUnchanged(t *testing.T) {
	con, _, header := recordingConnection(t, surrealcbor.New())

	ctx := context.Background()
	scope := func() (string, string) {
		t.Helper()
		_, err := con.Send(ctx, "query", "RETURN 1")
		require.NoError(t, err)
		return header.Get("Surreal-NS"), header.Get("Surreal-DB")
	}

	require.NoError(t, con.Use(ctx, "ns", "db"))

	_, err := con.Send(ctx, "use", models.None, "db2")
	require.NoError(t, err)
	ns, db := scope()
	assert.Equal(t, "ns", ns)
	assert.Equal(t, "db2", db)

	_, err = con.Send(ctx, "use", "ns2", &models.None)
	require.NoError(t, err)
	ns, db = scope()
	assert.Equal(t, "ns2", ns)
	assert.Equal(t, "db2", db)

	_, err = con.Send(ctx, "use", models.None, nil)
	require.NoError(t, err)
	ns, db = scope()
	assert.Equal(t, "ns2", ns)
	assert.Empty(t, db)
}

// Let keeps a value the deprecated fxamacker marshaler encodes within a
// request, though it fails to, or panics on, the same value on its own.
func TestLet_deprecatedMarshaler(t *testing.T) {
	con, sent, _ := recordingConnection(t, &models.CborMarshaler{}) //nolint:staticcheck // the deprecated marshaler is still supported

	ctx := context.Background()
	require.NoError(t, con.Let(ctx, "n", nil))
	require.NoError(t, con.Let(ctx, "m", map[string]string{"a": "b"}))
	_, err := con.Send(ctx, "set", "s", "text")
	require.NoError(t, err)

	_, err = con.Send(ctx, "query", "RETURN [$n, $m, $s]")
	require.NoError(t, err)
	require.Len(t, sent.Params, 2)
	vars, isMap := sent.Params[1].(map[string]any)
	require.True(t, isMap, "%T", sent.Params[1])
	assert.Contains(t, vars, "n")
	assert.Nil(t, vars["n"])
	assert.Equal(t, map[string]any{"a": "b"}, vars["m"])
	assert.Equal(t, "text", vars["s"])
}

type namedVars map[string]any

// Variables bound with Let are merged into a query's variables of any map
// type with string keys, which is encoded as an object as a map[string]any is.
func TestWithVariables_otherStringKeyedMaps(t *testing.T) {
	con, sent, _ := recordingConnection(t, surrealcbor.New())

	ctx := context.Background()
	require.NoError(t, con.Let(ctx, "x", 1))

	for _, given := range []any{map[string]string{"y": "2"}, namedVars{"y": "2"}} {
		_, err := con.Send(ctx, "query", "RETURN [$x, $y]", given)
		require.NoError(t, err)
		require.Len(t, sent.Params, 2)
		vars, isMap := sent.Params[1].(map[string]any)
		require.True(t, isMap, "%T", sent.Params[1])
		assert.EqualValues(t, 1, vars["x"], "%T", given)
		assert.Equal(t, "2", vars["y"], "%T", given)
	}
}

type namedString string

// Values of a defined string type, encoded as strings, are taken as strings:
// the token given to authenticate, the names given to let and unset, and the
// namespace and database given to use.
func TestNamedStrings(t *testing.T) {
	con, sent, header := recordingConnection(t, surrealcbor.New())

	ctx := context.Background()
	_, err := con.Send(ctx, "authenticate", namedString("the-token"))
	require.NoError(t, err)
	_, err = con.Send(ctx, "let", namedString("x"), 1)
	require.NoError(t, err)
	_, err = con.Send(ctx, "let", namedString("y"), 2)
	require.NoError(t, err)
	_, err = con.Send(ctx, "unset", namedString("y"))
	require.NoError(t, err)
	db := "db"
	_, err = con.Send(ctx, "use", namedString("ns"), &db)
	require.NoError(t, err)

	_, err = con.Send(ctx, "query", "RETURN $x")
	require.NoError(t, err)
	assert.Equal(t, "Bearer the-token", header.Get("Authorization"))
	assert.Equal(t, "ns", header.Get("Surreal-NS"))
	assert.Equal(t, "db", header.Get("Surreal-DB"))
	vars, _ := sent.Params[1].(map[string]any)
	assert.EqualValues(t, 1, vars["x"])
	assert.NotContains(t, vars, "y")

	var none *string
	_, err = con.Send(ctx, "use", models.None, none)
	require.NoError(t, err)
	_, err = con.Send(ctx, "query", "RETURN 1")
	require.NoError(t, err)
	assert.Equal(t, "ns", header.Get("Surreal-NS"))
	assert.Empty(t, header.Get("Surreal-DB"), "a nil pointer is null")
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

// The answer to a refresh canceled while it arrived is not applied, so that a
// renewal stopped by Close does not change the token.
func TestCall_canceledRefreshChangesNoToken(t *testing.T) {
	c := surrealcbor.New()
	pair, err := c.Marshal(map[string]any{"id": "abc", "result": map[string]any{"access": "new-access", "refresh": "new-refresh"}})
	require.NoError(t, err)
	ok, err := c.Marshal(map[string]any{"id": "abc", "result": []any{}})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var header http.Header
	con := New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: c, Unmarshaler: c})
	con.SetHTTPClient(NewTestClient(func(req *http.Request) *http.Response {
		header = req.Header
		var sent connection.RPCRequest
		body, _ := io.ReadAll(req.Body)
		_ = c.Unmarshal(body, &sent)
		res := ok
		if sent.Method == "refresh" {
			cancel()
			res = pair
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(res)), Header: http.Header{}}
	}))

	require.NoError(t, con.Authenticate(context.Background(), "old-access"))
	_, err = con.Send(ctx, "refresh", connection.Tokens{Access: "old-access", Refresh: "old-refresh"})
	require.ErrorIs(t, err, context.Canceled)

	_, err = con.Send(context.Background(), "query", "RETURN 1")
	require.NoError(t, err)
	assert.Equal(t, "Bearer old-access", header.Get("Authorization"))
}
