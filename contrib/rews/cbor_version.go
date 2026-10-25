package rews

import (
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

var _ connection.CBORVersioner = (*Connection[connection.WebSocketConnection])(nil)

// currentConn returns the connection in use, which a reconnect replaces.
func (arws *Connection[C]) currentConn() connection.WebSocketConnection {
	arws.connMu.RLock()
	defer arws.connMu.RUnlock()
	return arws.WebSocketConnection
}

// GetUnmarshaler returns the unmarshaler of the connection in use, which
// decodes the CBOR version that connection negotiated. Before the first
// connection it returns the unmarshaler given to New.
func (arws *Connection[C]) GetUnmarshaler() codec.Unmarshaler {
	if conn := arws.currentConn(); conn != nil {
		return conn.GetUnmarshaler()
	}
	return arws.unmarshaler
}

// CBORVersion returns the CBOR protocol version the connection in use
// negotiated. Each reconnect negotiates again, so it can change when the
// server is replaced by one of another version.
func (arws *Connection[C]) CBORVersion() connection.CBORVersion {
	if v, ok := arws.currentConn().(connection.CBORVersioner); ok {
		return v.CBORVersion()
	}
	return connection.CBORv1
}

// connUnmarshaler decodes with the unmarshaler of the connection in use, so
// that live-query bookkeeping reads responses in the CBOR version that
// connection negotiated.
type connUnmarshaler[C connection.WebSocketConnection] struct {
	arws *Connection[C]
}

func (u connUnmarshaler[C]) Unmarshal(data []byte, dst any) error {
	return u.arws.GetUnmarshaler().Unmarshal(data, dst)
}
