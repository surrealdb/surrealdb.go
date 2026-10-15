package rews

import (
	"context"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// CurrentConnection returns the socket in use, for tests outside the package.
func (arws *Connection[C]) CurrentConnection() connection.WebSocketConnection {
	arws.connMu.RLock()
	defer arws.connMu.RUnlock()
	return arws.WebSocketConnection
}

// IsConnected reports whether the connection is in the connected state.
func (arws *Connection[C]) IsConnected() bool {
	arws.stateMu.Lock()
	defer arws.stateMu.Unlock()
	return arws.state == StateConnected
}

// SessionToken returns the access token kept for reconnection.
func (arws *Connection[C]) SessionToken() string {
	arws.sessionMu.Lock()
	defer arws.sessionMu.Unlock()
	return arws.sessionToken
}

// DropSocket closes the socket in use, as a network failure would.
func (arws *Connection[C]) DropSocket(ctx context.Context) (connection.WebSocketConnection, error) {
	old := arws.CurrentConnection()
	return old, old.Close(ctx)
}
