package connection

import (
	"context"
	"fmt"
)

func Send[Result any](c Connection, ctx context.Context, res *RPCResponse[Result], method string, params ...interface{}) error {
	rawRes, err := c.Send(ctx, method, params...)
	if err != nil {
		return err
	}

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
