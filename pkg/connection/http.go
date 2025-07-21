package connection

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/internal/codec"

	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

type HTTPConnection struct {
	BaseConnection

	httpClient *http.Client
	variables  sync.Map
}

func NewHTTPConnection(p NewConnectionParams) *HTTPConnection {
	con := HTTPConnection{
		BaseConnection: BaseConnection{
			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,
			baseURL:     p.BaseURL,
		},
	}

	if con.httpClient == nil {
		con.httpClient = &http.Client{
			Timeout: constants.DefaultHTTPTimeout, // Set a default timeout to avoid hanging requests
		}
	}

	return &con
}

func (h *HTTPConnection) Connect(ctx context.Context) error {
	if err := h.preConnectionChecks(); err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"/health", http.NoBody)
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
	return h.unmarshaler
}

func (h *HTTPConnection) Send(ctx context.Context, dest any, method string, params ...interface{}) error {
	if h.baseURL == "" {
		return constants.ErrNoBaseURL
	}

	request := &RPCRequest{
		ID:     rand.String(constants.RequestIDLength),
		Method: method,
		Params: params,
	}
	reqBody, err := h.marshaler.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	if namespace, ok := h.variables.Load("namespace"); ok {
		req.Header.Set("Surreal-NS", namespace.(string))
	} else {
		return constants.ErrNoNamespaceOrDB
	}

	if database, ok := h.variables.Load("database"); ok {
		req.Header.Set("Surreal-DB", database.(string))
	} else {
		return constants.ErrNoNamespaceOrDB
	}

	if token, ok := h.variables.Load(constants.AuthTokenKey); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.(string)))
	}

	respData, err := h.MakeRequest(req)
	if err != nil {
		return err
	}

	var rpcRes RPCResponse[interface{}]
	if err := h.unmarshaler.Unmarshal(respData, &rpcRes); err != nil {
		return err
	}
	if rpcRes.Error != nil {
		return rpcRes.Error
	}

	if dest != nil {
		return h.unmarshaler.Unmarshal(respData, dest)
	}

	return nil
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
	var errorResponse RPCResponse[any]
	err = h.unmarshaler.Unmarshal(respBytes, &errorResponse)
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

func (h *HTTPConnection) Let(ctx context.Context, key string, value interface{}) error {
	h.variables.Store(key, value)
	return nil
}

func (h *HTTPConnection) Unset(ctx context.Context, key string) error {
	h.variables.Delete(key)
	return nil
}
