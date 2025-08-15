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
	variables  sync.Map
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
	if c.BaseURL == "" {
		return nil, constants.ErrNoBaseURL
	}

	request := &connection.RPCRequest{
		ID:     rand.NewRequestID(constants.RequestIDLength),
		Method: method,
		Params: params,
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

	return &res, nil
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
