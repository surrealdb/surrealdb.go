package connection

/*
#cgo CFLAGS: -I./../../libsrc/
#cgo LDFLAGS: -L./../../libsrc/target/release -lsurrealdb_c
#include "../../libsrc/surrealdb.h"
*/
import "C"

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"sync"
	"unsafe"
)

type EmbeddedConnection struct {
	BaseConnection

	variables sync.Map
	db        *C.sr_surreal_t
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

	var cErr C.sr_string_t
	var surreal *C.sr_surreal_t
	defer C.sr_free_string(cErr)

	endpoint := C.CString(h.baseURL)
	defer C.free(unsafe.Pointer(endpoint))

	if C.sr_connect(&cErr, &surreal, endpoint) < 0 {
		return fmt.Errorf("error connecting to SurrealDB: %s", C.GoString(cErr))
	}
	h.db = surreal

	return nil
}

func (h *EmbeddedConnection) Close() error {
	C.sr_surreal_disconnect(h.db)

	h.db = nil
	return nil
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

func (h *EmbeddedConnection) Use(namespace, database string) error {
	var cErr C.sr_string_t
	defer C.sr_free_string(cErr)

	ns := C.CString(namespace)
	dbName := C.CString(database)
	defer C.free(unsafe.Pointer(ns))
	defer C.free(unsafe.Pointer(dbName))

	if C.sr_use_ns(h.db, &cErr, ns) > 0 {
		return fmt.Errorf("error while setting namespace: %s", C.GoString(cErr))
	}

	if C.sr_use_db(h.db, &cErr, dbName) > 0 {
		return fmt.Errorf("error while setting database: %s", C.GoString(cErr))
	}

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
