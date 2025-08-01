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

type HTTPConnection struct {
	BaseURL     string
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler

	httpClient *http.Client
	variables  sync.Map
}

func New(p *connection.Config) *HTTPConnection {
	con := HTTPConnection{
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

func (h *HTTPConnection) Connect(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+"/health", http.NoBody)
	if err != nil {
		return err
	}
	_, err = h.MakeRequest(httpReq)
	if err != nil {
		return err
	}

	return nil
}

func (h *HTTPConnection) Close(ctx context.Context) error {
	return nil
}

func (h *HTTPConnection) SetTimeout(timeout time.Duration) *HTTPConnection {
	h.httpClient.Timeout = timeout
	return h
}

func (h *HTTPConnection) SetHTTPClient(client *http.Client) *HTTPConnection {
	h.httpClient = client
	return h
}

func (h *HTTPConnection) GetUnmarshaler() codec.Unmarshaler {
	return h.Unmarshaler
}

func (h *HTTPConnection) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	if h.BaseURL == "" {
		return nil, constants.ErrNoBaseURL
	}

	request := &connection.RPCRequest{
		ID:     rand.NewRequestID(constants.RequestIDLength),
		Method: method,
		Params: params,
	}
	reqBody, err := h.Marshaler.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	if namespace, ok := h.variables.Load("namespace"); ok {
		req.Header.Set("Surreal-NS", namespace.(string))
	} else {
		return nil, constants.ErrNoNamespaceOrDB
	}

	if database, ok := h.variables.Load("database"); ok {
		req.Header.Set("Surreal-DB", database.(string))
	} else {
		return nil, constants.ErrNoNamespaceOrDB
	}

	if token, ok := h.variables.Load(constants.AuthTokenKey); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.(string)))
	}

	respData, err := h.MakeRequest(req)
	if err != nil {
		return nil, err
	}

	var res connection.RPCResponse[cbor.RawMessage]
	if err := h.Unmarshaler.Unmarshal(respData, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, res.Error
	}

	return &res, nil
}

func (h *HTTPConnection) MakeRequest(req *http.Request) ([]byte, error) {
	resp, err := h.httpClient.Do(req)
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
	err = h.Unmarshaler.Unmarshal(respBytes, &errorResponse)
	if err != nil {
		panic(fmt.Sprintf("%s: %s", err, string(respBytes)))
	}
	return nil, errorResponse.Error
}

func (h *HTTPConnection) Use(ctx context.Context, namespace, database string) error {
	h.variables.Store("namespace", namespace)
	h.variables.Store("database", database)

	return nil
}

func (h *HTTPConnection) Let(ctx context.Context, key string, value any) error {
	h.variables.Store(key, value)
	return nil
}

func (h *HTTPConnection) Authenticate(ctx context.Context, token string) error {
	if err := rpc.Authenticate(h, ctx, token); err != nil {
		return err
	}

	if err := h.Let(ctx, constants.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

func (h *HTTPConnection) SignUp(ctx context.Context, authData any) (string, error) {
	token, err := rpc.SignUp(h, ctx, authData)
	if err != nil {
		return "", err
	}

	if err := h.Let(ctx, constants.AuthTokenKey, token); err != nil {
		return "", err
	}

	return token, nil
}

func (h *HTTPConnection) Invalidate(ctx context.Context) error {
	if err := rpc.Invalidate(h, ctx); err != nil {
		return err
	}

	if err := h.Unset(ctx, constants.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

func (h *HTTPConnection) Unset(ctx context.Context, key string) error {
	h.variables.Delete(key)
	return nil
}

func (h *HTTPConnection) LiveNotifications(id string) (chan connection.Notification, error) {
	return nil, errors.New("live notifications are not supported in HTTP connections")
}
