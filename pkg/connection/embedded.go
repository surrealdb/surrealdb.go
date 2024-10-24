package connection

/*
#cgo LDFLAGS: -L./../../libsrc -lsurrealdb_c
#include <stdlib.h>
#include "./../../libsrc/surrealdb.h"
*/
import "C"

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"sync"
	"unsafe"
)

type EmbeddedConnection struct {
	BaseConnection

	variables  sync.Map
	db         *C.sr_surreal_t
	surrealRPC *C.struct_sr_surreal_rpc_t
}

func (h *EmbeddedConnection) GetUnmarshaler() codec.Unmarshaler {
	//TODO implement me
	panic("implement me")
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
	//ctx := context.TODO()
	if err := h.preConnectionChecks(); err != nil {
		return err
	}

	var cErr C.sr_string_t
	defer C.sr_free_string(cErr)

	cEndpoint := C.CString(h.baseURL)
	defer C.free(unsafe.Pointer(cEndpoint))

	var surreal *C.sr_surreal_t
	if C.sr_connect(&cErr, &surreal, cEndpoint) < 0 {
		return fmt.Errorf("error connecting to SurrealDB: %s", C.GoString(cErr))
	}
	h.db = surreal

	var surrealOptions C.sr_option_t
	var surrealPtr *C.struct_sr_surreal_rpc_t
	if C.sr_surreal_rpc_new(&cErr, &surrealPtr, cEndpoint, surrealOptions) < 0 {
		return fmt.Errorf("error initiating RPC: %s", C.GoString(cErr))
	}
	h.surrealRPC = surrealPtr

	return nil
}

func (h *EmbeddedConnection) Close() error {
	C.sr_surreal_disconnect(h.db)

	h.db = nil
	return nil
}

func (h *EmbeddedConnection) Send(res interface{}, method string, params ...interface{}) error {

	return nil
}

func (h *EmbeddedConnection) Use(namespace, database string) error {
	var cErr C.sr_string_t
	defer C.sr_free_string(cErr)

	ns := C.CString(namespace)
	dbName := C.CString(database)
	defer C.free(unsafe.Pointer(ns))
	defer C.free(unsafe.Pointer(dbName))

	if C.sr_use_ns(h.db, &cErr, ns) < 0 {
		return fmt.Errorf("error while setting namespace: %s", C.GoString(cErr))
	}

	if C.sr_use_db(h.db, &cErr, dbName) < 0 {
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

func callSrSurrealRPCExecute(self *C.struct_sr_surreal_rpc_t, input []byte) ([]byte, error) {
	var errPtr C.sr_string_t
	var resPtr *C.uint8_t

	// Convert Go byte slice to C pointer
	inputPtr := (*C.uint8_t)(unsafe.Pointer(&input[0]))
	inputLen := C.int(len(input))

	// Call the C function
	ret := C.sr_surreal_rpc_execute(self, &errPtr, &resPtr, inputPtr, inputLen)

	// Check for error in return value
	if ret != 0 {
		errorMessage := C.GoString(errPtr)
		return nil, fmt.Errorf("Error: %s", errorMessage)
	}

	// If successful, process the result (resPtr).
	// Assuming the result is a null-terminated string, you could use:
	// GoString or manually manage memory if it’s a different format.

	// Let's assume the result is also a byte array. You can calculate the length
	// from some additional logic, here I'm assuming the result is another array.
	resultBytes := C.GoBytes(unsafe.Pointer(resPtr), C.int(len(input))) // or some other length calculation

	return resultBytes, nil
}
