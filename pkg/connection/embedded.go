package connection

/*
#cgo LDFLAGS: -L./../../libsrc -lsurrealdb_c
#include <stdlib.h>
#include "./../../libsrc/surrealdb.h"
*/
import "C"

import (
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"sync"
	"unsafe"
)

type EmbeddedConnection struct {
	BaseConnection

	variables  sync.Map
	surrealRPC *C.sr_surreal_rpc_t
}

func (h *EmbeddedConnection) GetUnmarshaler() codec.Unmarshaler {
	return h.unmarshaler
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
	if err := h.preConnectionChecks(); err != nil {
		return err
	}

	var cErr C.sr_string_t
	defer C.sr_free_string(cErr)

	cEndpoint := C.CString(h.baseURL)
	defer C.free(unsafe.Pointer(cEndpoint))

	var surrealOptions C.sr_option_t
	var surrealPtr *C.sr_surreal_rpc_t
	if ret := C.sr_surreal_rpc_new(&cErr, &surrealPtr, cEndpoint, surrealOptions); ret < 0 {
		return fmt.Errorf("error initiating rpc. %v. return %v", C.GoString(cErr), ret)
	}
	h.surrealRPC = surrealPtr

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

	resultBytes := cbor.RawMessage(C.GoBytes(unsafe.Pointer(cRes), C.int(resSize)))

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
