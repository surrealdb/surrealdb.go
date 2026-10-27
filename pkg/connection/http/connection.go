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

	c.setSessionHeaders(req)

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

// setSessionHeaders sets the namespace, database and authorization headers
// from the state of this connection.
func (c *Connection) setSessionHeaders(req *http.Request) {
	// The namespace and database are optional: a root or namespace user signs in,
	// asks for the version, or runs a query naming its own namespace without them.
	// Methods that do need them are refused by the server with its own error.
	if namespace, ok := c.variables.Load("namespace"); ok && namespace.(string) != "" {
		req.Header.Set("Surreal-NS", namespace.(string))
	}

	if database, ok := c.variables.Load("database"); ok && database.(string) != "" {
		req.Header.Set("Surreal-DB", database.(string))
	}

	if token, ok := c.variables.Load(constants.AuthTokenKey); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.(string)))
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
