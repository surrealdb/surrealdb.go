package surrealdb

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code" msgpack:"code"`
	Message string `json:"message,omitempty" msgpack:"message,omitempty"`
}

func (r *RPCError) Error() string {
	return r.Message
}

// RPCRequest represents an incoming JSON-RPC request
type RPCRequest struct {
	ID     interface{}   `json:"id" msgpack:"id"`
	Async  bool          `json:"async,omitempty" msgpack:"async,omitempty"`
	Method string        `json:"method,omitempty" msgpack:"method,omitempty"`
	Params []interface{} `json:"params,omitempty" msgpack:"params,omitempty"`
}

// RPCResponse represents an outgoing JSON-RPC response
type RPCResponse struct {
	ID     interface{} `json:"id" msgpack:"id"`
	Error  *RPCError   `json:"error,omitempty" msgpack:"error,omitempty"`
	Result interface{} `json:"result,omitempty" msgpack:"result,omitempty"`
}

// RPCNotification represents an outgoing JSON-RPC notification
type RPCNotification struct {
	ID     interface{}   `json:"id" msgpack:"id"`
	Method string        `json:"method,omitempty" msgpack:"method,omitempty"`
	Params []interface{} `json:"params,omitempty" msgpack:"params,omitempty"`
}
