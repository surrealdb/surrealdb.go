package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"

	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Connection is a connection to SurrealDB over HTTP /rpc.
//
// It negotiates the CBOR protocol version as it goes (see
// [connection.CBORVersion]): when its codec speaks cbor.2, it sends plain
// CBOR requests with "Accept: application/cbor; v=2" until the server answers
// one in cbor.2, and from then on sends cbor.2 requests. Servers before
// SurrealDB 4.0 answer in plain CBOR, so the connection stays on it. Each
// response is decoded by its own Content-Type.
type Connection struct {
	BaseURL     string
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler

	// DisableCBORv2 makes the connection speak only plain CBOR.
	// See [connection.Config.DisableCBORv2].
	DisableCBORv2 bool

	httpClient *http.Client

	// HTTP is stateless, so the state a WebSocket session keeps on the server
	// is kept here, and sent with every request.
	stateMu   sync.RWMutex
	namespace string
	database  string
	token     string
	vars      map[string]any

	// cborVersion becomes CBORv2 once the server has answered in cbor.2.
	cborVersion atomic.Int32
}

func New(p *connection.Config) *Connection {
	con := Connection{
		Marshaler:     p.Marshaler,
		Unmarshaler:   p.Unmarshaler,
		BaseURL:       p.BaseURL,
		DisableCBORv2: p.DisableCBORv2,
	}

	if con.httpClient == nil {
		con.httpClient = &http.Client{
			Timeout: constants.DefaultHTTPTimeout, // Set a default timeout to avoid hanging requests
		}
	}

	return &con
}

func (c *Connection) Connect(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", http.NoBody)
	if err != nil {
		return err
	}
	_, err = c.MakeRequest(httpReq)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) Close(ctx context.Context) error {
	return nil
}

func (c *Connection) SetTimeout(timeout time.Duration) *Connection {
	c.httpClient.Timeout = timeout
	return c
}

func (c *Connection) SetHTTPClient(client *http.Client) *Connection {
	c.httpClient = client
	return c
}

// GetUnmarshaler returns the unmarshaler for the CBOR version negotiated so far.
func (c *Connection) GetUnmarshaler() codec.Unmarshaler {
	_, u := c.codecs(c.CBORVersion())
	return u
}

// CBORVersion returns CBORv2 once the server has answered a request in
// cbor.2, else CBORv1.
func (c *Connection) CBORVersion() connection.CBORVersion {
	if connection.CBORVersion(c.cborVersion.Load()) == connection.CBORv2 {
		return connection.CBORv2
	}
	return connection.CBORv1
}

// offersCBORv2 reports whether requests ask for cbor.2 responses.
func (c *Connection) offersCBORv2() bool {
	return !c.DisableCBORv2 && connection.CanSpeakCBORv2(c.Marshaler, c.Unmarshaler)
}

// codecs returns the marshaler and unmarshaler for CBOR version v, or the
// plain CBOR ones when the codec does not speak v.
func (c *Connection) codecs(v connection.CBORVersion) (codec.Marshaler, codec.Unmarshaler) {
	if m, u, ok := connection.CBORCodecs(c.Marshaler, c.Unmarshaler, v); ok {
		return m, u
	}
	m, u, _ := connection.CBORCodecs(c.Marshaler, c.Unmarshaler, connection.CBORv1)
	return m, u
}

// requestFormat returns the marshaler and the Content-Type and Accept headers
// for the next request.
//
// It sends cbor.2 only once the server has answered in it: a server that
// speaks cbor.2 refuses a cbor.2 request that asks for a plain CBOR response,
// and one before SurrealDB 4.0 does not know cbor.2 at all. Until then, it
// sends plain CBOR and asks for cbor.2 back.
func (c *Connection) requestFormat() (marshaler codec.Marshaler, contentType, accept string) {
	version := c.CBORVersion()
	marshaler, _ = c.codecs(version)
	switch {
	case version == connection.CBORv2:
		return marshaler, connection.MIMECBOR2, connection.MIMECBOR2
	case c.offersCBORv2():
		return marshaler, connection.MIMECBOR, connection.MIMECBOR2
	default:
		return marshaler, connection.MIMECBOR, connection.MIMECBOR
	}
}

// responseCBORVersion returns the CBOR version to decode a response with the
// given Content-Type in. Anything but a cbor.2 response we asked for is
// decoded as plain CBOR.
func (c *Connection) responseCBORVersion(contentType string) connection.CBORVersion {
	if v, ok := connection.CBORVersionFromContentType(contentType); ok && v == connection.CBORv2 && c.offersCBORv2() {
		return connection.CBORv2
	}
	return connection.CBORv1
}

func (c *Connection) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	request := &connection.RPCRequest{
		Method: method,
		Params: params,
	}
	return c.Call(ctx, request)
}

// Call sends a custom RPC request to SurrealDB and expects a response.
// For HTTP connections, this method returns an error if req.Session or req.Txn is set,
// as sessions and interactive transactions require WebSocket connections.
func (c *Connection) Call(ctx context.Context, request *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	// HTTP connections do not support sessions or transactions
	if request.Session != nil {
		return nil, constants.ErrSessionsNotSupported
	}
	if request.Txn != nil {
		return nil, constants.ErrTransactionsNotSupported
	}

	if c.BaseURL == "" {
		return nil, constants.ErrNoBaseURL
	}

	// Set request ID if not already set
	if request.ID == nil || request.ID == "" {
		request.ID = rand.NewRequestID(constants.RequestIDLength)
	}

	// Methods which only change the state of the session are answered here,
	// as a stateless HTTP request could not keep what they change.
	if res, handled, err := c.callLocally(ctx, request); handled {
		return res, err
	}

	if request.Method == string(connection.Query) {
		request = c.withVariables(request)
	}

	marshaler, contentType, accept := c.requestFormat()
	reqBody, err := marshaler.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", contentType)

	c.setSessionHeaders(req, sendsOwnCredentials(request.Method))

	respData, respVersion, err := c.makeRequest(req)
	if err != nil {
		return nil, err
	}
	if respVersion == connection.CBORv2 {
		// Results are decoded later with GetUnmarshaler, so switch before returning.
		c.cborVersion.Store(int32(connection.CBORv2))
	}

	_, unmarshaler := c.codecs(respVersion)
	var res connection.RPCResponse[cbor.RawMessage]
	if err := unmarshaler.Unmarshal(respData, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, res.Error
	}

	c.trackAuth(request, res.Result)

	return &res, nil
}

// The methods which only change the state of the session, which the
// connection keeps itself.
const (
	methodUse        = "use"
	methodLet        = "let"
	methodSet        = "set"
	methodUnset      = "unset"
	methodReset      = "reset"
	methodInvalidate = "invalidate"
)

// sendsOwnCredentials reports whether a method carries the credentials it is
// checked with, so is sent without the token the connection keeps. The server
// checks the Authorization header of every request, and answers 401 to one
// whose token it no longer accepts, so sending the stored token would keep a
// connection whose token has expired from signing in again.
func sendsOwnCredentials(method string) bool {
	switch method {
	case "signin", "signup", "authenticate":
		return true
	}
	return false
}

// callLocally answers the methods which only change the state of the session.
// It refuses parameters a WebSocket server would refuse, rather than keep them,
// and changes nothing once ctx is done.
func (c *Connection) callLocally(
	ctx context.Context, request *connection.RPCRequest,
) (res *connection.RPCResponse[cbor.RawMessage], handled bool, err error) {
	// apply returns the result the server gives, where nil is the null a
	// WebSocket server answers let, unset, reset and invalidate with.
	var apply func(method string, params []any) (any, error)
	switch request.Method {
	case methodUse:
		apply = c.localUse
	case methodLet, methodSet:
		apply = c.localLet
	case methodUnset:
		apply = c.localUnset
	case methodReset:
		apply = c.localReset
	case methodInvalidate:
		apply = c.localInvalidate
	default:
		return nil, false, nil
	}

	if ctx.Err() != nil {
		return nil, true, ctx.Err()
	}
	result, err := apply(request.Method, request.Params)
	if err != nil {
		return nil, true, err
	}
	res = &connection.RPCResponse[cbor.RawMessage]{ID: request.ID}
	if result != nil {
		var encoded cbor.RawMessage
		if encoded, err = c.encodeParam(result); err != nil {
			return nil, true, err
		}
		res.Result = &encoded
	}
	return res, true, nil
}

// localUse returns the namespace and database it leaves selected, as
// SurrealDB 3.x servers do, with NONE for one that is not selected.
func (c *Connection) localUse(method string, params []any) (any, error) {
	ns, nsOK := scopeParam(params, 0)
	db, dbOK := scopeParam(params, 1)
	if !nsOK || !dbOK || len(params) != 2 {
		return nil, invalidParams(method, "a namespace and a database")
	}
	ns.name, db.name = c.setNamespaceAndDatabase(ns, db)
	return map[string]any{"namespace": noneIfEmpty(ns.name), "database": noneIfEmpty(db.name)}, nil
}

func noneIfEmpty(s string) any {
	if s == "" {
		return &models.CustomNil{}
	}
	return s
}

// protectedVariables are the names SurrealDB refuses to bind, with let and in
// the variables of a query alike.
var protectedVariables = map[string]bool{"access": true, "auth": true, "token": true, "session": true}

// localLet binds a variable, or, given only a name or a NONE value, removes
// it, as SurrealDB servers do.
func (c *Connection) localLet(method string, params []any) (any, error) {
	key, ok := paramString(params, 0)
	if !ok || len(params) > 2 {
		return nil, invalidParams(method, "a variable name and an optional value")
	}
	if protectedVariables[key] {
		// Kept, it would be sent with, and refused in, every later query.
		return nil, invalidParamsError(fmt.Sprintf("Cannot set protected variable: %s", key))
	}
	if len(params) == 1 || isNone(params[1]) {
		c.unsetVariable(key)
		return nil, nil
	}
	// The value is kept encoded, as a WebSocket server would receive it,
	// so later changes to it do not leak into queries, and a value which
	// cannot be encoded fails here rather than in every later query.
	value, err := c.encodeParam(params[1])
	if err != nil {
		return nil, fmt.Errorf("%s $%s: %w", method, key, err)
	}
	c.setVariable(key, value)
	return nil, nil
}

func isNone(v any) bool {
	switch v.(type) {
	case models.CustomNil, *models.CustomNil:
		return true
	}
	return false
}

func (c *Connection) localUnset(method string, params []any) (any, error) {
	key, ok := paramString(params, 0)
	if !ok || len(params) != 1 {
		return nil, invalidParams(method, "a variable name")
	}
	c.unsetVariable(key)
	return nil, nil
}

// localReset ignores any parameters, as SurrealDB servers do: 2.x and 3.x
// reset a WebSocket session given reset with parameters. It is never sent:
// SurrealDB 2.6.0 aborts given reset over HTTP.
func (c *Connection) localReset(_ string, _ []any) (any, error) {
	c.reset()
	return nil, nil
}

// localInvalidate forgets the token. Over HTTP the server keeps no session
// for invalidate to clear, and answers 401 to a request carrying a token it
// no longer accepts, so sending it could leave the token in place.
func (c *Connection) localInvalidate(_ string, _ []any) (any, error) {
	c.setToken("")
	return nil, nil
}

// invalidParams is the error a SurrealDB server gives for a method called
// with the wrong parameters.
func invalidParams(method, want string) error {
	return invalidParamsError(fmt.Sprintf("Invalid params: %s takes %s", method, want))
}

// invalidParamsError is an invalid params error with the given message. It
// is built as an error from the server is, so it is both an *RPCError and,
// through errors.As, a *ServerError.
func invalidParamsError(message string) error {
	data, err := cbor.Marshal(map[string]any{
		"code":    -32602,
		"kind":    connection.KindValidation,
		"message": message,
	})
	if err != nil {
		return err
	}
	rpcErr := &connection.RPCError{} //nolint:staticcheck // RPCError is still what server errors arrive as
	if err := rpcErr.UnmarshalCBOR(data); err != nil {
		return err
	}
	return rpcErr
}

// encodeParam encodes a parameter as it is encoded within a request: as an
// element of the request's parameters rather than on its own, as the
// deprecated models.CborMarshaler changes, and can fail on, the top-level
// value it is given. It encodes in the CBOR version requests are sent in now, as
// a WebSocket session sends let in the version it speaks at the time.
func (c *Connection) encodeParam(v any) (cbor.RawMessage, error) {
	marshaler, _, _ := c.requestFormat()
	data, err := marshaler.Marshal([]any{v})
	if err != nil {
		return nil, err
	}
	// The value follows the header of an array of one element.
	const arrayOfOne = 0x81
	if len(data) < 2 || data[0] != arrayOfOne {
		return nil, fmt.Errorf("encoding a parameter gave unexpected CBOR %x", data)
	}
	return cbor.RawMessage(data[1:]), nil
}

// scopeChange is a namespace or database given to use.
type scopeChange struct {
	name string
	// keep is set for NONE, which leaves the current one as it is.
	keep bool
}

// scopeParam returns the namespace or database parameter of use at i: a
// string, nil (none), or NONE (the current one, unchanged).
func scopeParam(params []any, i int) (scopeChange, bool) {
	if i >= len(params) {
		return scopeChange{}, false
	}
	switch p := params[i].(type) {
	case nil:
		return scopeChange{}, true
	case models.CustomNil, *models.CustomNil:
		return scopeChange{keep: true}, true
	default:
		if v := reflect.ValueOf(p); v.Kind() == reflect.Pointer && v.IsNil() {
			// Encoded as null, as nil is.
			return scopeChange{}, true
		}
		name, ok := stringValue(p)
		return scopeChange{name: name}, ok
	}
}

func paramString(params []any, i int) (string, bool) {
	if i >= len(params) {
		return "", false
	}
	return stringValue(params[i])
}

// stringValue returns v as the string it is encoded as: a string, a value of
// a defined string type such as type Token string, or a pointer to either.
// A type with a CBOR encoding of its own is not taken for a string.
func stringValue(v any) (string, bool) {
	if s, ok := v.(string); ok {
		return s, true
	}
	if _, custom := v.(cbor.Marshaler); custom {
		return "", false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv = rv.Elem()
		if rv.CanInterface() {
			if _, custom := rv.Interface().(cbor.Marshaler); custom {
				return "", false
			}
		}
	}
	if rv.Kind() != reflect.String {
		return "", false
	}
	return rv.String(), true
}

// stringKeyedMap returns the entries of v when it is a map with string keys,
// such as a map[string]string or a defined map type, which is encoded as an
// object as a map[string]any is.
func stringKeyedMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	if _, custom := v.(cbor.Marshaler); custom {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	m := make(map[string]any, rv.Len())
	for iter := rv.MapRange(); iter.Next(); {
		m[iter.Key().String()] = iter.Value().Interface()
	}
	return m, true
}

// withVariables returns a copy of a query request whose variables include
// those set with Let, which the variables given with the query take precedence over.
func (c *Connection) withVariables(request *connection.RPCRequest) *connection.RPCRequest {
	if len(request.Params) == 0 {
		return request
	}

	c.stateMu.RLock()
	merged := make(map[string]any, len(c.vars))
	for k, v := range c.vars {
		merged[k] = v
	}
	c.stateMu.RUnlock()

	if len(merged) == 0 {
		return request
	}

	if len(request.Params) > 1 && request.Params[1] != nil {
		given, ok := c.variablesObject(request.Params[1])
		if !ok {
			// Variables of another type are sent as they are.
			return request
		}
		for k, v := range given {
			merged[k] = v
		}
	}

	params := []any{request.Params[0], merged}
	if len(request.Params) > 2 {
		params = append(params, request.Params[2:]...)
	}

	copied := *request
	copied.Params = params
	return &copied
}

// variablesObject returns the entries of the variables given with a query
// when they are encoded as an object, such as a map with string keys, a
// struct, or a pointer to either; a nil pointer, encoded as null, has none.
func (c *Connection) variablesObject(v any) (map[string]any, bool) {
	if m, ok := stringKeyedMap(v); ok {
		return m, true
	}
	data, err := c.encodeParam(v)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	const (
		cborNull     = 0xf6
		cborMapMajor = 5
	)
	if data[0] == cborNull {
		return nil, true
	}
	if data[0]>>5 != cborMapMajor {
		return nil, false
	}
	var entries map[string]cbor.RawMessage
	if err := cbor.Unmarshal(data, &entries); err != nil {
		return nil, false
	}
	m := make(map[string]any, len(entries))
	for k, v := range entries {
		m[k] = v
	}
	return m, true
}

// trackAuth keeps the token of a successful signin, signup or authenticate
// request, however it was sent.
func (c *Connection) trackAuth(request *connection.RPCRequest, result *cbor.RawMessage) {
	switch request.Method {
	case "signin", "signup":
		if result == nil {
			return
		}
		// The result is a token, or a token pair when the access method
		// gives refresh tokens.
		var token string
		if err := c.GetUnmarshaler().Unmarshal(*result, &token); err == nil {
			c.setToken(token)
			return
		}
		var tokens connection.Tokens
		if err := c.GetUnmarshaler().Unmarshal(*result, &tokens); err == nil && tokens.Access != "" {
			c.setToken(tokens.Access)
		}
	case "authenticate":
		if token, ok := paramString(request.Params, 0); ok {
			c.setToken(token)
		}
	}
}

// setNamespaceAndDatabase changes the namespace and database, and returns
// those it leaves selected.
func (c *Connection) setNamespaceAndDatabase(namespace, database scopeChange) (ns, db string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if !namespace.keep {
		c.namespace = namespace.name
	}
	if !database.keep {
		c.database = database.name
	}
	return c.namespace, c.database
}

// reset forgets everything the session holds, as the reset RPC does on a
// WebSocket connection.
func (c *Connection) reset() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.namespace = ""
	c.database = ""
	c.token = ""
	c.vars = nil
}

func (c *Connection) setToken(token string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.token = token
}

func (c *Connection) setVariable(key string, value any) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.vars == nil {
		c.vars = make(map[string]any)
	}
	c.vars[key] = value
}

func (c *Connection) unsetVariable(key string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	delete(c.vars, key)
}

// setSessionHeaders sets the namespace, database and, unless withoutToken is
// set, authorization headers from the state of this connection.
func (c *Connection) setSessionHeaders(req *http.Request, withoutToken bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	// The namespace and database are optional: a root or namespace user signs in,
	// asks for the version, or runs a query naming its own namespace without them.
	// Methods that do need them are refused by the server with its own error.
	if c.namespace != "" {
		req.Header.Set("Surreal-NS", c.namespace)
	}

	if c.database != "" {
		req.Header.Set("Surreal-DB", c.database)
	}

	if c.token != "" && !withoutToken {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
}

func (c *Connection) MakeRequest(req *http.Request) ([]byte, error) {
	body, _, err := c.makeRequest(req)
	return body, err
}

// makeRequest sends req and returns the response body with the CBOR version
// to decode it in, which its Content-Type decides.
func (c *Connection) makeRequest(req *http.Request) ([]byte, connection.CBORVersion, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	respContentType := resp.Header.Get("Content-Type")
	version := c.responseCBORVersion(respContentType)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBytes, version, nil
	}

	return nil, 0, c.errorFromResponse(resp, respBytes, version)
}

// StatusError is returned when the server answers an HTTP request with a non-2xx
// status and a body that is not a SurrealDB RPC error, such as a plain-text
// rejection, or an HTML page from a proxy in front of the server.
type StatusError struct {
	// StatusCode is the HTTP status code, such as 502.
	StatusCode int
	// Status is the HTTP status line, such as "502 Bad Gateway".
	Status string
	// ContentType is the media type of the body, if the server sent one.
	ContentType string
	// Body is the response body, as text.
	Body string
}

func (e *StatusError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("unexpected HTTP status %s", e.Status)
	}
	return fmt.Sprintf("unexpected HTTP status %s: %s", e.Status, body)
}

// errorFromResponse turns a non-2xx response into an error. A body carrying a
// SurrealDB RPC error yields that error; anything else, including a body that is
// not CBOR at all, yields a *StatusError. The body is decoded in version.
func (c *Connection) errorFromResponse(resp *http.Response, body []byte, version connection.CBORVersion) error {
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])

	statusErr := &StatusError{
		StatusCode:  resp.StatusCode,
		Status:      resp.Status,
		ContentType: contentType,
		Body:        string(body),
	}
	if statusErr.Status == "" {
		statusErr.Status = fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if contentType == "" || len(body) == 0 {
		return statusErr
	}

	_, unmarshaler := c.codecs(version)
	var errorResponse connection.RPCResponse[any]
	if err := unmarshaler.Unmarshal(body, &errorResponse); err != nil || errorResponse.Error == nil {
		return statusErr
	}
	return errorResponse.Error
}

func (c *Connection) Use(ctx context.Context, namespace, database string) error {
	_, _, err := c.callLocally(ctx, &connection.RPCRequest{Method: methodUse, Params: []any{namespace, database}})
	return err
}

// Let defines a variable which every subsequent query on this connection can use.
//
// As HTTP is stateless, the variable is kept by the connection and sent along with
// each query, where the variables passed to the query take precedence over it.
// Only the query method takes variables, so unlike on a WebSocket connection the
// variable is not available to other methods such as select, create or run.
//
// A NONE value (models.None) removes the variable, as Unset does. The names
// access, auth, token and session are refused, as SurrealDB refuses them.
func (c *Connection) Let(ctx context.Context, key string, value any) error {
	_, _, err := c.callLocally(ctx, &connection.RPCRequest{Method: methodLet, Params: []any{key, value}})
	return err
}

// Unset removes a variable defined with Let.
func (c *Connection) Unset(ctx context.Context, key string) error {
	_, _, err := c.callLocally(ctx, &connection.RPCRequest{Method: methodUnset, Params: []any{key}})
	return err
}

func (c *Connection) Authenticate(ctx context.Context, token string) error {
	return rpc.Authenticate(c, ctx, token)
}

func (c *Connection) SignUp(ctx context.Context, authData any) (string, error) {
	return rpc.SignUp(c, ctx, authData)
}

func (c *Connection) SignIn(ctx context.Context, authData any) (string, error) {
	return rpc.SignIn(c, ctx, authData)
}

func (c *Connection) SignUpWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	return rpc.SignUpWithRefresh(c, ctx, authData)
}

func (c *Connection) SignInWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	return rpc.SignInWithRefresh(c, ctx, authData)
}

func (c *Connection) Invalidate(ctx context.Context) error {
	return rpc.Invalidate(c, ctx)
}

func (c *Connection) LiveNotifications(id string) (chan connection.Notification, error) {
	return nil, errors.New("live notifications are not supported in HTTP connections")
}

func (c *Connection) CloseLiveNotifications(id string) error {
	return errors.New("live notifications are not supported in HTTP connections")
}
