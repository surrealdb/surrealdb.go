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

	switch method {
	case "refresh", "revoke":
		return arws.sendTokenChange(ctx, method, params)
	}

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
	return arws.reliableLQ.closeLiveNotifications(conn, id)
}

// sendTokenChange sends a refresh or revoke, which change the token pair a
// reconnection signs in with, and keeps that pair in step, as SignInWithRefresh
// does. Without this, a pair renewed by the client, which sends refresh through
// Send, would leave the connection holding a refresh token the server has
// already revoked.
func (arws *Connection[C]) sendTokenChange(
	ctx context.Context, method string, params []any,
) (*connection.RPCResponse[cbor.RawMessage], error) {
	ctx, conn, release, err := arws.lockSessionSocket(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	res, err := conn.Send(ctx, method, params...)
	if err != nil {
		return nil, socketErr(ctx, err)
	}

	switch method {
	case "refresh":
		// The server signs the session in with the new pair.
		var tokens connection.Tokens
		if res.Result != nil && arws.unmarshaler.Unmarshal(*res.Result, &tokens) == nil && tokens.Access != "" {
			arws.setToken(tokens.Access, tokens.Refresh, arws.sessionAccess)
		}
	case "revoke":
		// The session stays signed in, but the refresh token no longer works.
		if refresh := revokedRefresh(params); refresh != "" && refresh == arws.sessionRefresh {
			arws.sessionRefresh = ""
		}
	}
	return res, nil
}

// revokedRefresh returns the refresh token of the pair given to revoke.
func revokedRefresh(params []any) string {
	if len(params) == 0 {
		return ""
	}
	switch t := params[0].(type) {
	case connection.Tokens:
		return t.Refresh
	case *connection.Tokens:
		if t != nil {
			return t.Refresh
		}
	}
	return ""
}
