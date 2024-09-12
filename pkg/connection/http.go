package connection

import (
	"bytes"
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/model"
	"io"
	"log"
	"net/http"
	"time"
)

type Http struct {
	BaseConnection

	httpClient *http.Client

	namespace string
	database  string
	token     string
}

func NewHttp(p NewConnectionParams) *Http {
	con := Http{
		BaseConnection: BaseConnection{
			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,
		},
	}

	if con.httpClient == nil {
		con.httpClient = &http.Client{
			Timeout: 10 * time.Second, // Set a default timeout to avoid hanging requests
		}
	}

	return &con
}

func (h *Http) Connect() error {
	if h.baseURL == "" {
		return fmt.Errorf("base url not set")
	}

	if h.marshaler == nil {
		return fmt.Errorf("marshaler is not set")
	}

	if h.unmarshaler == nil {
		return fmt.Errorf("unmarshaler is not set")
	}

	httpReq, err := http.NewRequest(http.MethodGet, h.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	_, err = h.MakeRequest(httpReq)
	if err != nil {
		return err
	}

	return nil
}

func (h *Http) Close() error {
	return nil
}

func (h *Http) SetTimeout(timeout time.Duration) *Http {
	h.httpClient.Timeout = timeout
	return h
}

func (h *Http) SetHttpClient(client *http.Client) *Http {
	h.httpClient = client
	return h
}

func (h *Http) Send(method string, params []interface{}) (interface{}, error) {
	if h.baseURL == "" {
		return nil, fmt.Errorf("connection host not set")
	}

	if h.namespace == "" || h.database == "" {
		return nil, fmt.Errorf("namespace or database or both are not set")
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
	req.Header.Set("Accept", "application/cbor")
	req.Header.Set("Content-Type", "application/cbor")

	if h.namespace != "" {
		req.Header.Set("Surreal-NS", h.namespace)
	}

	if h.database != "" {
		req.Header.Set("Surreal-DB", h.database)
	}

	if h.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.token))
	}

	resp, err := h.MakeRequest(req)
	if err != nil {
		return nil, err
	}

	var rpcResponse RPCResponse
	err = h.unmarshaler.Unmarshal(resp, &rpcResponse)

	return rpcResponse.Result, nil
}

func (h *Http) MakeRequest(req *http.Request) ([]byte, error) {
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

func (h *Http) Use(namespace string, database string) error {
	h.namespace = namespace
	h.database = database

	return nil
}

func (h *Http) SignIn(auth model.Auth) (string, error) {
	resp, err := h.Send("signin", []interface{}{auth})
	if err != nil {
		return "", err
	}

	h.token = resp.(string)

	return resp.(string), nil
}

func (h *Http) signup() {

}

func (h *Http) let() {

}

func (h *Http) unset() {

}

func (h *Http) authenticate() {

}

func (h *Http) invalidate() {

}
