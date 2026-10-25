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
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"

	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
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
	variables  sync.Map

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

	if namespace, ok := c.variables.Load("namespace"); ok {
		req.Header.Set("Surreal-NS", namespace.(string))
	} else {
		return nil, constants.ErrNoNamespaceOrDB
	}

	if database, ok := c.variables.Load("database"); ok {
		req.Header.Set("Surreal-DB", database.(string))
	} else {
		return nil, constants.ErrNoNamespaceOrDB
	}

	if token, ok := c.variables.Load(constants.AuthTokenKey); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.(string)))
	}

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

	return &res, nil
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

	contentType := strings.Split(respContentType, ";")[0]
	if strings.TrimSpace(contentType) == "" {
		return nil, 0, fmt.Errorf("%s", string(respBytes))
	}
	_, unmarshaler := c.codecs(version)
	var errorResponse connection.RPCResponse[any]
	err = unmarshaler.Unmarshal(respBytes, &errorResponse)
	if err != nil {
		panic(fmt.Sprintf("%s: %s", err, string(respBytes)))
	}
	return nil, 0, errorResponse.Error
}

func (c *Connection) Use(ctx context.Context, namespace, database string) error {
	c.variables.Store("namespace", namespace)
	c.variables.Store("database", database)

	return nil
}

func (c *Connection) Let(ctx context.Context, key string, value any) error {
	c.variables.Store(key, value)
	return nil
}

func (c *Connection) Authenticate(ctx context.Context, token string) error {
	if err := rpc.Authenticate(c, ctx, token); err != nil {
		return err
	}

	if err := c.Let(ctx, constants.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

func (c *Connection) SignUp(ctx context.Context, authData any) (string, error) {
	token, err := rpc.SignUp(c, ctx, authData)
	if err != nil {
		return "", err
	}

	if err := c.Let(ctx, constants.AuthTokenKey, token); err != nil {
		return "", err
	}

	return token, nil
}

func (c *Connection) SignIn(ctx context.Context, authData any) (string, error) {
	token, err := rpc.SignIn(c, ctx, authData)
	if err != nil {
		return "", err
	}

	if err := c.Let(ctx, constants.AuthTokenKey, token); err != nil {
		return "", err
	}

	return token, nil
}

func (c *Connection) SignUpWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	tokens, err := rpc.SignUpWithRefresh(c, ctx, authData)
	if err != nil {
		return nil, err
	}

	// Store access token for Authorization header on subsequent requests
	if err := c.Let(ctx, constants.AuthTokenKey, tokens.Access); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (c *Connection) SignInWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	tokens, err := rpc.SignInWithRefresh(c, ctx, authData)
	if err != nil {
		return nil, err
	}

	// Store access token for Authorization header on subsequent requests
	if err := c.Let(ctx, constants.AuthTokenKey, tokens.Access); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (c *Connection) Invalidate(ctx context.Context) error {
	if err := rpc.Invalidate(c, ctx); err != nil {
		return err
	}

	if err := c.Unset(ctx, constants.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

func (c *Connection) Unset(ctx context.Context, key string) error {
	c.variables.Delete(key)
	return nil
}

func (c *Connection) LiveNotifications(id string) (chan connection.Notification, error) {
	return nil, errors.New("live notifications are not supported in HTTP connections")
}

func (c *Connection) CloseLiveNotifications(id string) error {
	return errors.New("live notifications are not supported in HTTP connections")
}
