package connection

import (
	"bytes"
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
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
			Timeout: 10 * time.Second, // Set a default timeout to avoid hanging requests
		}
	}

	return &con
}

func (h *HTTPConnection) Connect() error {
	if h.baseURL == "" {
		return fmt.Errorf("base url not set")
	}

	if h.marshaler == nil {
		return fmt.Errorf("marshaler is not set")
	}

	if h.unmarshaler == nil {
		return fmt.Errorf("unmarshaler is not set")
	}

	httpReq, err := http.NewRequest(http.MethodGet, h.baseURL+"/health", http.NoBody)
	if err != nil {
		return err
	}
	_, err = h.MakeRequest(httpReq)
	if err != nil {
		return err
	}

	return nil
}

func (h *HTTPConnection) Close() error {
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

func (h *HTTPConnection) Send(method string, params []interface{}) (interface{}, error) {
	if h.baseURL == "" {
		return nil, fmt.Errorf("connection host not set")
	}

	rpcReq := &RPCRequest{
		ID:     rand.String(RequestIDLength),
		Method: method,
		Params: params,
	}

	reqBody, err := h.marshaler.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, h.baseURL+"/rpc", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	if namespace, ok := h.variables.Load("namespace"); ok {
		req.Header.Set("Surreal-NS", namespace.(string))
	} else {
		return nil, fmt.Errorf("namespace or database or both are not set")
	}

	if database, ok := h.variables.Load("database"); ok {
		req.Header.Set("Surreal-DB", database.(string))
	} else {
		return nil, fmt.Errorf("namespace or database or both are not set")
	}

	if token, ok := h.variables.Load("token"); ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := h.MakeRequest(req)
	if err != nil {
		return nil, err
	}

	var rpcResponse RPCResponse
	err = h.unmarshaler.Unmarshal(resp, &rpcResponse)
	if err != nil {
		return nil, err
	}

	// Manage auth tokens
	switch method {
	case "signin", "signup":
		h.variables.Store("token", rpcResponse.Result)
	case "authenticate":
		h.variables.Store("token", params[0])
	case "invalidate":
		h.variables.Delete("token")
	}

	return rpcResponse.Result, nil
}

func (h *HTTPConnection) MakeRequest(req *http.Request) ([]byte, error) {
	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (h *HTTPConnection) Use(namespace, database string) error {
	h.variables.Store("namespace", namespace)
	h.variables.Store("database", database)

	return nil
}

func (h *HTTPConnection) Let(key string, value interface{}) error {
	h.variables.Store(key, value)
	return nil
}

func (h *HTTPConnection) Unset(key string) error {
	h.variables.Delete(key)
	return nil
}
