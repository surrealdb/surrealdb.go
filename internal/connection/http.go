package connection

import (
	"bytes"
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Http struct {
	BaseConnection

	httpClient *http.Client

	namespace string
	database  string
}

func NewHttp(p NewConnectionParams) Connection {
	con := Http{
		BaseConnection: BaseConnection{
			encode: p.Encoder,
			decode: p.Decoder,
		},
	}

	if con.httpClient == nil {
		con.httpClient = &http.Client{
			Timeout: 10 * time.Second, // Set a default timeout to avoid hanging requests
		}
	}

	return &con
}

func (h *Http) Connect(url string) (Connection, error) {
	// TODO: EXTRACT BASE url and set
	h.baseURL = url

	_, err := h.MakeRequest(http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}

	return h, nil
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

	rpcReq := &RPCRequest{
		ID:     rand.String(RequestIDLength),
		Method: method,
		Params: params,
	}

	reqBody, err := h.encode(rpcReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(method, h.baseURL+"rpc", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Accept", "application/cbor")
	httpReq.Header.Set("Content-Type", "application/cbor")

	resp, err := h.MakeRequest(http.MethodPost, "/rpc", reqBody)
	if err != nil {
		return nil, err
	}

	var rpcResponse RPCResponse
	err = h.decode(resp, &rpcResponse)

	return &rpcResponse, nil
}

func (h *Http) MakeRequest(method string, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}
