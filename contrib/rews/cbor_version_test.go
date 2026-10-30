package rews

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// cbor2Conn is a connection that negotiated cbor.2.
type cbor2Conn struct {
	mockWebSocketConnection
}

func (c *cbor2Conn) CBORVersion() connection.CBORVersion { return connection.CBORv2 }

func (c *cbor2Conn) GetUnmarshaler() codec.Unmarshaler { return surrealcbor.NewV2() }

func TestConnection_forwardsCBORVersion(t *testing.T) {
	v1 := surrealcbor.New()
	arws := New(func(context.Context) (*cbor2Conn, error) { return &cbor2Conn{}, nil }, 0, v1, nil)

	// Before connecting: plain CBOR and the unmarshaler given to New.
	assert.Equal(t, connection.CBORv1, arws.CBORVersion())
	assert.Same(t, v1, arws.GetUnmarshaler())

	// Once connected: whatever the connection in use negotiated.
	arws.WebSocketConnection = &cbor2Conn{}
	assert.Equal(t, connection.CBORv2, arws.CBORVersion())
	var got any
	assert.NoError(t, arws.GetUnmarshaler().Unmarshal([]byte{0xd8, 0xb4, 0x01}, &got))
	assert.Equal(t, int8(1), got)

	// A connection that does not report a version is plain CBOR.
	arws.WebSocketConnection = &mockWebSocketConnection{}
	assert.Equal(t, connection.CBORv1, arws.CBORVersion())
}
