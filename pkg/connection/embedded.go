package connection

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"net/http"
	"sync"
	"time"
)

/*
#include <stdlib.h>
*/
import "C"

type EmbeddedConnection struct {
	BaseConnection

	variables sync.Map
}

func NewEmbeddedConnection(p NewConnectionParams) *EmbeddedConnection {
	con := EmbeddedConnection{
		BaseConnection: BaseConnection{
			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,
			baseURL:     p.BaseURL,
		},
	}

	return &con
}

func (h *EmbeddedConnection) Connect() error {
	if h.baseURL == "" {
		return fmt.Errorf("base url not set")
	}

	if h.marshaler == nil {
		return fmt.Errorf("marshaler is not set")
	}

	if h.unmarshaler == nil {
		return fmt.Errorf("unmarshaler is not set")
	}

	return nil
}

func (h *EmbeddedConnection) Close() error {
	return nil
}

func (h *EmbeddedConnection) SetTimeout(timeout time.Duration) *EmbeddedConnection {
	return h
}

func (h *EmbeddedConnection) Send(method string, params []interface{}) (interface{}, error) {
	if h.baseURL == "" {
		return nil, fmt.Errorf("connection host not set")
	}

	rpcReq := &RPCRequest{
		ID:     rand.String(RequestIDLength),
		Method: method,
		Params: params,
	}

	_, err := h.marshaler.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (h *EmbeddedConnection) MakeRequest(req *http.Request) ([]byte, error) {
	return nil, nil
}

func (h *EmbeddedConnection) Use(namespace, database string) error {
	h.variables.Store("namespace", namespace)
	h.variables.Store("database", database)

	return nil
}

func (h *EmbeddedConnection) Let(key string, value interface{}) error {
	h.variables.Store(key, value)
	return nil
}

func (h *EmbeddedConnection) Unset(key string) error {
	h.variables.Delete(key)
	return nil
}
