package connection

import (
	"context"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// Send sends a request to SurrealDB using the connection's Send method.
// It unmarshals the response into the provided res parameter.
func Send[Result any](c Connection, ctx context.Context, res *RPCResponse[Result], method string, params ...interface{}) error {
	rawRes, err := c.Send(ctx, method, params...)
	if err != nil {
		return err
	}

	return unmarshalResponse(c, rawRes, res)
}

// Call sends a custom RPC request to SurrealDB using the connection's Call method.
// Unlike Send, Call accepts an RPCRequest directly, allowing you to set
// Session and Txn fields for session-scoped or transaction-scoped operations (SurrealDB v3+).
func Call[Result any](c Connection, ctx context.Context, res *RPCResponse[Result], req *RPCRequest) error {
	rawRes, err := c.Call(ctx, req)
	if err != nil {
		return err
	}

	return unmarshalResponse(c, rawRes, res)
}

// unmarshalResponse is a shared helper to unmarshal the RPC response.
func unmarshalResponse[Result any](c Connection, rawRes *RPCResponse[cbor.RawMessage], res *RPCResponse[Result]) error {
	if res == nil {
		return nil
	}

	// Unmarshal the ID and Error fields of the response.
	if rawRes.ID != nil {
		res.ID = rawRes.ID
	}
	res.Error = rawRes.Error

	// Unmarshal the Result field of the response.
	if rawRes.Result == nil {
		res.Result = nil
		return nil
	}

	var r Result

	data, err := rawRes.Result.MarshalCBOR()
	if err != nil {
		return fmt.Errorf("Send: error marshaling result: %w", err)
	}

	if err := c.GetUnmarshaler().Unmarshal(data, &r); err != nil {
		return fmt.Errorf("Send: error unmarshaling result: %w", err)
	}

	res.Result = &r

	return nil
}
