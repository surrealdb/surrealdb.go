package surrealdb

import (
	"encoding/json"

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

// Some example RPC Responses for reference

// RPC Create Method:
// {"id":"08b8b533d5a62fc8","result":[{"id":"tests:rsmek8y8q26q1y20472p","username":"test"}]}

// RPC Update Method:
// {"id":"70f0b5034830fc8f","result":[{"id":"tests:rsmek8y8q26q1y20472p","username":"test"},{"id":"tests:t5ysffcdge93j7lsejus","username":"test"}]}

// Rpc Modify Method:
// {"id":"6","result":[[{"op":"add","path":"/age","value":44},{"op":"add","path":"/nickname","value":"Bobs nickname"}]]}

// Rpc Response with error:
// {"id":"9451b5314acbd4be","error":{"code":-32000,"message":"There was a problem with the database: Parse error on line 1 at character 0 when parsing 'fjhfdjhfhdj;'"}}

type RPCRawResponse struct {
	// Holds the raw socket response data
	rawData []byte

	// The method used for the RPCRequest, query, create, update etc
	rpcMethod string

	// Set to true when we've pulled the "result" data out of the response
	hasDecodedRpcResult bool
	// Holds the data from the "result" field in the response
	rpcResult []byte
	// Holds the type of the "result" for future reference in QueryResolver
	rpcResultDataType jsonparser.ValueType

	// Set to true when we've decoded the rpc message id
	hasDecodedRpcId bool
	// The id string, once we've decoded it from the rpc response
	id string

	// This is set to true when we do 100% have an error, we can't just check for error != nil
	// because we need to check for the error field in the response, so by default, error will be nil
	hasRpcError bool
	// This is set to true, when we've done our look-up for the error field
	hasDecodedRpcError bool
	// When we've done our checks and decoded the error, we set it here, if it exists
	rpcError *RPCError

	// This is set if we come across an error during the decoding, just prevents "silent failure"
	internalProcessingError error
}

func CreateRPCRawResponse(data []byte) *RPCRawResponse {
	response := &RPCRawResponse{
		rawData: data,
	}

	response.resolveId()
	response.resolveError()

	// If we didn't hit an error, then we'll pull the "result" data out of the
	// response and store it for the end user to decode how they wish!
	if !response.HasError() {
		response.resolveResult()
	}

	return response
}

// resolveError will try to do a cheap decoding of the response to check if we have an rpc error
// this will just prevent unmarshalling and re-marshalling multiple times.
func (res *RPCRawResponse) resolveError() {
	if res.hasDecodedRpcError {
		return
	}

	// After some benchmarks:
	// BenchmarkDecodeRpcError-10   8340408	   142.3 ns/op
	// Time wise, it's cheap to look up, but then takes the same time if we just json.Unmarshal once we know it's there.
	errorValue, dataType, _, err := jsonparser.Get(res.rawData, "error")
	if err != nil && dataType != jsonparser.NotExist {
		res.internalProcessingError = err
		return
	}
	// There's no error field in the response
	if dataType == jsonparser.NotExist {
		res.hasRpcError = false
		res.hasDecodedRpcError = true
		return
	}

	err = json.Unmarshal(errorValue, &res.rpcError)
	if err != nil {
		res.internalProcessingError = err
		return
	}

	res.hasRpcError = true
	res.hasDecodedRpcError = true

	return
}

// resolveId this will try to do a cheap decoding of the response to check if we have an rpc id
func (res *RPCRawResponse) resolveId() {
	if res.hasDecodedRpcId {
		return
	}

	id, err := jsonparser.GetString(res.rawData, "id")
	if err != nil {
		res.internalProcessingError = err
		return
	}

	res.id = id
	res.hasDecodedRpcId = true
}

func (res *RPCRawResponse) resolveResult() {
	if res.hasDecodedRpcResult {
		return
	}

	resultData, dataType, _, err := jsonparser.Get(res.rawData, "result")
	if err != nil {
		res.internalProcessingError = err
		return
	}

	res.rpcResultDataType = dataType
	res.rpcResult = resultData
	res.hasDecodedRpcResult = true
}

// HasError Check if we have an error set
func (res *RPCRawResponse) HasError() bool {
	return res.Error() != nil
}

// HasInternalError Check if we have an internal error set
func (res *RPCRawResponse) HasInternalError() bool {
	return res.internalProcessingError != nil
}

// Error returns the error we discovered in the rpc response, or the error we encountered while decoding
func (res *RPCRawResponse) Error() error {
	if res.hasRpcError && res.rpcError != nil {
		return res.rpcError
	}

	if res.HasInternalError() {
		return res.internalProcessingError
	}

	return nil
}

func (res *RPCRawResponse) Id() string {
	return res.id
}

func (res *RPCRawResponse) RawData() []byte {
	return res.rawData
}

type RpcResultData struct {
	Result []byte
	Type   jsonparser.ValueType
}

func (res *RPCRawResponse) Result() *RpcResultData {
	return &RpcResultData{
		Result: res.rpcResult,
		Type:   res.rpcResultDataType,
	}
}
