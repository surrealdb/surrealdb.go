package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"

	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

type Connection struct {
	BaseURL     string
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler

	httpClient *http.Client

	// HTTP is stateless, so the state a WebSocket session keeps on the server
	// is kept here, and sent with every request.
	stateMu   sync.RWMutex
	namespace string
	database  string
	token     string
	vars      map[string]any
}

func New(p *connection.Config) *Connection {
	con := Connection{
		Marshaler:   p.Marshaler,
		Unmarshaler: p.Unmarshaler,
		BaseURL:     p.BaseURL,
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

func (c *Connection) GetUnmarshaler() codec.Unmarshaler {
	return c.Unmarshaler
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

	// Methods which only change the state of the session are answered here,
	// as a stateless HTTP request could not keep what they change.
	if res, handled, err := c.callLocally(ctx, request); handled {
		return res, err
	}

	// Set request ID if not already set
	if request.ID == nil || request.ID == "" {
		request.ID = rand.NewRequestID(constants.RequestIDLength)
	}

	if request.Method == string(connection.Query) {
		request = c.withVariables(request)
	}

	reqBody, err := c.Marshaler.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	c.setSessionHeaders(req)

	respData, err := c.MakeRequest(req)
	if err != nil {
		return nil, err
	}

	var res connection.RPCResponse[cbor.RawMessage]
	if err := c.Unmarshaler.Unmarshal(respData, &res); err != nil {
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
	methodUse   = "use"
	methodLet   = "let"
	methodSet   = "set"
	methodUnset = "unset"
)

// callLocally answers the methods which only change the state of the session.
// It refuses parameters a WebSocket server would refuse, rather than keep them,
// and changes nothing once ctx is done.
func (c *Connection) callLocally(
	ctx context.Context, request *connection.RPCRequest,
) (res *connection.RPCResponse[cbor.RawMessage], handled bool, err error) {
	switch request.Method {
	case methodUse, methodLet, methodSet, methodUnset:
		if err := ctx.Err(); err != nil {
			return nil, true, err
		}
	default:
		return nil, false, nil
	}

	switch request.Method {
	case methodUse:
		ns, nsOK := optionalString(request.Params, 0)
		db, dbOK := optionalString(request.Params, 1)
		if !nsOK || !dbOK || len(request.Params) != 2 {
			return nil, true, invalidParams(request.Method, "a namespace and a database")
		}
		c.setNamespaceAndDatabase(ns, db)
	case methodLet, methodSet:
		key, ok := paramString(request.Params, 0)
		if !ok || len(request.Params) != 2 {
			return nil, true, invalidParams(request.Method, "a variable name and a value")
		}
		// The value is kept encoded, as a WebSocket server would receive it,
		// so later changes to it do not leak into queries, and a value which
		// cannot be encoded fails here rather than in every later query.
		value, err := c.Marshaler.Marshal(request.Params[1])
		if err != nil {
			return nil, true, fmt.Errorf("%s $%s: %w", request.Method, key, err)
		}
		c.setVariable(key, cbor.RawMessage(value))
	case methodUnset:
		key, ok := paramString(request.Params, 0)
		if !ok || len(request.Params) != 1 {
			return nil, true, invalidParams(request.Method, "a variable name")
		}
		c.unsetVariable(key)
	default:
		return nil, false, nil
	}

	return &connection.RPCResponse[cbor.RawMessage]{ID: request.ID}, true, nil
}

// invalidParams is the error a SurrealDB server gives for a method called
// with the wrong parameters. It is built as an error from the server is, so
// it is both an *RPCError and, through errors.As, a *ServerError.
func invalidParams(method, want string) error {
	data, err := cbor.Marshal(map[string]any{
		"code":    -32602,
		"kind":    connection.KindValidation,
		"message": fmt.Sprintf("Invalid params: %s takes %s", method, want),
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

// optionalString returns the string parameter at i, or "" for nil.
func optionalString(params []any, i int) (string, bool) {
	if i < len(params) && params[i] == nil {
		return "", true
	}
	return paramString(params, i)
}

func paramString(params []any, i int) (string, bool) {
	if i >= len(params) {
		return "", false
	}
	s, ok := params[i].(string)
	return s, ok
}

// withVariables returns a copy of a query request whose variables include
// those set with Let, which the variables given with the query take precedence over.
func (c *Connection) withVariables(request *connection.RPCRequest) *connection.RPCRequest {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	if len(c.vars) == 0 || len(request.Params) == 0 {
		return request
	}

	merged := make(map[string]any, len(c.vars))
	for k, v := range c.vars {
		merged[k] = v
	}

	if len(request.Params) > 1 && request.Params[1] != nil {
		given, ok := request.Params[1].(map[string]any)
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

// trackAuth keeps the token of a successful signin, signup or authenticate
// request, however it was sent, and forgets it on invalidate.
func (c *Connection) trackAuth(request *connection.RPCRequest, result *cbor.RawMessage) {
	switch request.Method {
	case "signin", "signup":
		if result == nil {
			return
		}
		// The result is a token, or a token pair when the access method
		// gives refresh tokens.
		var token string
		if err := c.Unmarshaler.Unmarshal(*result, &token); err == nil {
			c.setToken(token)
			return
		}
		var tokens connection.Tokens
		if err := c.Unmarshaler.Unmarshal(*result, &tokens); err == nil && tokens.Access != "" {
			c.setToken(tokens.Access)
		}
	case "authenticate":
		if token, ok := paramString(request.Params, 0); ok {
			c.setToken(token)
		}
	case "invalidate":
		c.setToken("")
	}
}

func (c *Connection) setNamespaceAndDatabase(namespace, database string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.namespace = namespace
	c.database = database
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

// setSessionHeaders sets the namespace, database and authorization headers
// from the state of this connection.
func (c *Connection) setSessionHeaders(req *http.Request) {
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

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
}

func (c *Connection) MakeRequest(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return respBytes, nil
	}

	contentType := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	if strings.TrimSpace(contentType) == "" {
		return nil, fmt.Errorf("%s", string(respBytes))
	}
	var errorResponse connection.RPCResponse[any]
	err = c.Unmarshaler.Unmarshal(respBytes, &errorResponse)
	if err != nil {
		panic(fmt.Sprintf("%s: %s", err, string(respBytes)))
	}
	return nil, errorResponse.Error
}

func (c *Connection) Use(ctx context.Context, namespace, database string) error {
	_, _, err := c.callLocally(ctx, &connection.RPCRequest{Method: methodUse, Params: []any{namespace, database}})
	return err
}

// Let defines a variable which every subsequent query on this connection can use.
//
// As HTTP is stateless, the variable is kept by the connection and sent along with
// each query, where the variables passed to the query take precedence over it.
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
