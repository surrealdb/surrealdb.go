package rews

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
)

// GetMarshaler returns the marshaler of the connection in use, and nil when
// there is no connection or it exposes no marshaler.
func TestGetMarshaler(t *testing.T) {
	conn := &Connection[connection.WebSocketConnection]{}
	assert.Nil(t, conn.GetMarshaler(), "no connection")

	conn.WebSocketConnection = &mockWebSocketConnection{}
	assert.Nil(t, conn.GetMarshaler(), "a connection which exposes no marshaler")

	u, err := url.ParseRequestURI("ws://localhost:8000/rpc")
	require.NoError(t, err)
	config := connection.NewConfig(u)
	conn.WebSocketConnection = gorillaws.New(config)
	assert.Same(t, config.Marshaler, conn.GetMarshaler())
}
