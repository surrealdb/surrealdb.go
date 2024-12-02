//go:build exclude

package connection

/*
#cgo LDFLAGS: -L./../../libsrc -lsurrealdb_c
#include <stdlib.h>
#include "./../../libsrc/surrealdb.h"
*/
import "C"

import (
	"fmt"
	"net/url"
	"sync"
	"unsafe"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

type EmbeddedConnection struct {
	BaseConnection

	variables sync.Map

	surrealRPC    *C.sr_surreal_rpc_t
	surrealStream *C.sr_RpcStream

	closeChan chan int
	closeErr  error
}

func (h *EmbeddedConnection) GetUnmarshaler() codec.Unmarshaler {
	return h.unmarshaler
}

func NewEmbeddedConnection(p NewConnectionParams) *EmbeddedConnection {
	con := EmbeddedConnection{
		BaseConnection: BaseConnection{
			baseURL: p.BaseURL,

			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,

			responseChannels:     make(map[string]chan []byte),
			notificationChannels: make(map[string]chan Notification),
		},

		closeChan: make(chan int),
	}

	return &con
}

func (h *EmbeddedConnection) Connect() error {
	err := h.preConnectionChecks()
	if err != nil {
		return err
	}

	var cErr C.sr_string_t
	defer C.sr_free_string(cErr)

	cEndpoint := C.CString(h.baseURL)
	u, err := url.ParseRequestURI(h.baseURL)
	if err != nil {
		return err
	}
	if u.Scheme == "mem" || u.Scheme == "memory" {
		cEndpoint = C.CString("memory")
	}
	defer C.free(unsafe.Pointer(cEndpoint))

	var surrealOptions C.sr_option_t
	var surrealRPC *C.sr_surreal_rpc_t
	if ret := C.sr_surreal_rpc_new(&cErr, &surrealRPC, cEndpoint, surrealOptions); ret < 0 {
		return fmt.Errorf("error initiating rpc. %v. return %v", C.GoString(cErr), ret)
	}
	h.surrealRPC = surrealRPC

	var cStream *C.sr_RpcStream
	if ret := C.sr_surreal_rpc_notifications(h.surrealRPC, &cErr, &cStream); ret < 0 {
		return fmt.Errorf("error initiating rpc. %v. return %v", C.GoString(cErr), ret)
	}
	h.surrealStream = cStream

	return nil
}

func (h *EmbeddedConnection) Close() error {
	C.sr_surreal_rpc_free(h.surrealRPC)

	h.surrealRPC = nil
	return nil
}

func (h *EmbeddedConnection) Send(res interface{}, method string, params ...interface{}) error {
	request := &RPCRequest{
		ID:     rand.String(constants.RequestIDLength),
		Method: method,
		Params: params,
	}
	reqBody, err := h.marshaler.Marshal(request)
	if err != nil {
		return err
	}

	var cErr C.sr_string_t
	defer C.sr_free_string(cErr)

	inputPtr := (*C.uint8_t)(unsafe.Pointer(&reqBody[0]))
	inputLen := C.int(len(reqBody))

	var cRes *C.uint8_t
	defer C.free(unsafe.Pointer(cRes))

	resSize := C.sr_surreal_rpc_execute(h.surrealRPC, &cErr, &cRes, inputPtr, inputLen)
	if resSize < 0 {
		return fmt.Errorf("%v", C.GoString(cErr))
	}

	if res == nil {
		return nil
	}

	resultBytes := cbor.RawMessage(C.GoBytes(unsafe.Pointer(cRes), resSize))

	rpcRes, _ := h.marshaler.Marshal(RPCResponse[cbor.RawMessage]{ID: request.ID, Result: &resultBytes})
	return h.unmarshaler.Unmarshal(rpcRes, res)
}

func (h *EmbeddedConnection) Use(namespace, database string) error {
	return h.Send(nil, "use", namespace, database)
}

func (h *EmbeddedConnection) Let(key string, value interface{}) error {
	return h.Send(nil, "let", key, value)
}

func (h *EmbeddedConnection) Unset(key string) error {
	return h.Send(nil, "unset", key)
}
