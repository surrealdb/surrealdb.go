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

	"github.com/fxamacker/cbor/v2"
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
			Marshaler:   p.Marshaler,
			Unmarshaler: p.Unmarshaler,
			BaseURL:     p.BaseURL,
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
	if err := h.PreConnectionChecks(); err != nil {
		return err
	}

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

func (h *HTTPConnection) Send(ctx context.Context, dest any, method string, params ...interface{}) error {
	if h.BaseURL == "" {
		return constants.ErrNoBaseURL
	}

	request := &RPCRequest{
		ID:     rand.String(constants.RequestIDLength),
		Method: method,
		Params: params,
	}
	reqBody, err := h.Marshaler.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+"/rpc", bytes.NewBuffer(reqBody))
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

	var res RPCResponse[cbor.RawMessage]
	if err := h.Unmarshaler.Unmarshal(respData, &res); err != nil {
		return err
	}
	if res.Error != nil {
		return res.Error
	}

	// In case the caller designated to throw away the result by specifying `nil` as `dest`,
	// OR the response Result says its nowherey by being nil,
	// we cannot proceed with unmarshaling the Result field,
	// because it would always fail.
	// The only thing we can do is to return the error if any.
	if nilOrTypedNil(dest) || res.Result == nil || res.Error != nil {
		return eliminateTypedNilError(res.Error)
	}

	if err := unmarshalRes(h.Unmarshaler, res, dest); err != nil {
		return fmt.Errorf("error unmarshaling response: %w", err)
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

func (h *HTTPConnection) Let(ctx context.Context, key string, value interface{}) error {
	h.variables.Store(key, value)
	return nil
}

func (h *HTTPConnection) Unset(ctx context.Context, key string) error {
	h.variables.Delete(key)
	return nil
}
