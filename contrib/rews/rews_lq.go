package rews

import (
	"context"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// Send overrides the WebSocketConnection's Send method to intercept live queries
func (arws *Connection[C]) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	// Let handleSend intercept and potentially handle the send
	handled, resp, err := arws.reliableLQ.handleSend(ctx, method, params, arws.WebSocketConnection, arws.logger)
	if handled {
		// handleSend processed this request
		return resp, err
	}

	// Not a live query, send normally through the underlying connection
	return arws.WebSocketConnection.Send(ctx, method, params...)
}

// LiveNotifications overrides to provide UUID mapping for live queries
func (arws *Connection[C]) LiveNotifications(id string) (chan connection.Notification, error) {
	return arws.reliableLQ.liveNotifications(id, arws.WebSocketConnection)
}

// CloseLiveNotifications overrides to handle UUID mapping
func (arws *Connection[C]) CloseLiveNotifications(id string) error {
	_, err := arws.reliableLQ.closeLiveNotifications(arws.WebSocketConnection, id)
	return err
}
