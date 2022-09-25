package surrealdb

import (
	"log"

	"github.com/buger/jsonparser"
)

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int64  `json:"code" msgpack:"code"`
	Message string `json:"message,omitempty" msgpack:"message,omitempty"`
}

func (r *RPCError) Error() string {
	return r.Message
}

// RPCRequest represents an incoming JSON-RPC request
type RPCRequest struct {
	ID     any    `json:"id" msgpack:"id"`
	Async  bool   `json:"async,omitempty" msgpack:"async,omitempty"`
	Method string `json:"method,omitempty" msgpack:"method,omitempty"`
	Params []any  `json:"params,omitempty" msgpack:"params,omitempty"`
}

// RPCResponse represents an outgoing JSON-RPC response
type RPCResponse struct {
	ID     any       `json:"id" msgpack:"id"`
	Error  *RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result any       `json:"result,omitempty" msgpack:"result,omitempty"`
}

// RPCNotification represents an outgoing JSON-RPC notification
type RPCNotification struct {
	ID     any    `json:"id" msgpack:"id"`
	Method string `json:"method,omitempty" msgpack:"method,omitempty"`
	Params []any  `json:"params,omitempty" msgpack:"params,omitempty"`
}

type RPCRawResponse struct {
	id      string
	Data    []byte
	Decoded *RPCResponse

	decodedError bool
	hasError     bool
	error        *RPCError
}

func (res *RPCRawResponse) Error() error {
	if res.HasError() {
		return res.error
	}

	return nil
}

func (res *RPCRawResponse) HasError() bool {
	if res.decodedError {
		return res.hasError
	}

	if res.error != nil {
		return true
	}
	errorValue, dataType, _, err := jsonparser.Get(res.Data, "error")
	if err != nil && dataType != jsonparser.NotExist {
		log.Println("Error parsing error", err)
	}

	if dataType == jsonparser.NotExist {
		res.hasError = false
		res.decodedError = true
		return false
	}

	if dataType == jsonparser.Object {
		res.error = &RPCError{}
		var err error
		res.error.Message, err = jsonparser.GetString(errorValue, "message")
		if err != nil {
			log.Println("Error parsing error message", err)
		}
		res.error.Code, err = jsonparser.GetInt(errorValue, "code")
		if err != nil {
			log.Println("Error parsing error code", err)
		}

		res.hasError = true
		res.decodedError = true
		return true
	}

	res.decodedError = true

	return false
}

func (res *RPCRawResponse) ResolveId() (string, error) {
	if res.id != "" {
		return res.id, nil
	}

	id, err := jsonparser.GetString(res.Data, "id")
	if err != nil {
		return "", err
	}

	res.id = id

	return id, nil
}
func (res *RPCRawResponse) Id() string {
	return res.id
}
