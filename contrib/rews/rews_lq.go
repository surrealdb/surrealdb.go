package rews

import (
	"context"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// Send overrides the WebSocketConnection's Send method to intercept live queries
func (arws *Connection[C]) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	// Get the current connection while holding the read lock
	arws.connMu.RLock()
	conn := arws.WebSocketConnection
	arws.connMu.RUnlock()

	// Let handleSend intercept and potentially handle the send
	handled, resp, err := arws.reliableLQ.handleSend(ctx, method, params, conn, arws.logger)
	if handled {
		// handleSend processed this request
		return resp, err
	}

	// Not a live query, send normally through the underlying connection
	return conn.Send(ctx, method, params...)
}

// LiveNotifications overrides to provide UUID mapping for live queries
func (arws *Connection[C]) LiveNotifications(id string) (chan connection.Notification, error) {
	arws.connMu.RLock()
	conn := arws.WebSocketConnection
	arws.connMu.RUnlock()
	return arws.reliableLQ.liveNotifications(id, conn)
}

// CloseLiveNotifications overrides to handle UUID mapping
func (arws *Connection[C]) CloseLiveNotifications(id string) error {
	arws.connMu.RLock()
	conn := arws.WebSocketConnection
	arws.connMu.RUnlock()
	_, err := arws.reliableLQ.closeLiveNotifications(conn, id)
	return err
}
