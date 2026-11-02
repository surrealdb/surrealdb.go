package rews

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

// endpointConnection is a WebSocket connection which knows its server's HTTP
// endpoint and health.
type endpointConnection struct {
	*mockWebSocketConnection
	baseURL string
	client  *http.Client
	health  error
}

func (c *endpointConnection) HTTPEndpoint() (string, *http.Client) { return c.baseURL, c.client }

func (c *endpointConnection) Health(context.Context) error { return c.health }

// The HTTP endpoint and the health check, which Import, Export and Health
// use, are those of the connection in use, which a reconnection replaces.
func TestConnection_forwardsHTTPEndpointAndHealth(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{}
	first := &endpointConnection{mockWebSocketConnection: &mockWebSocketConnection{}, baseURL: "ws://first/rpc", client: client}
	conn := &Connection[*endpointConnection]{WebSocketConnection: first}

	var _ connection.HTTPEndpoint = conn
	var _ connection.HealthChecker = conn

	baseURL, got := conn.HTTPEndpoint()
	assert.Equal(t, "ws://first/rpc", baseURL)
	assert.Same(t, client, got)
	require.NoError(t, conn.Health(ctx))

	unhealthy := errors.New("unhealthy")
	conn.connMu.Lock()
	conn.WebSocketConnection = &endpointConnection{mockWebSocketConnection: &mockWebSocketConnection{}, baseURL: "ws://second/rpc", health: unhealthy}
	conn.connMu.Unlock()

	baseURL, _ = conn.HTTPEndpoint()
	assert.Equal(t, "ws://second/rpc", baseURL)
	require.ErrorIs(t, conn.Health(ctx), unhealthy)
}

// A connection which knows neither gives no endpoint, and no health.
func TestConnection_withoutHTTPEndpointOrHealth(t *testing.T) {
	conn := &Connection[*mockWebSocketConnection]{WebSocketConnection: &mockWebSocketConnection{}}

	baseURL, client := conn.HTTPEndpoint()
	assert.Empty(t, baseURL)
	assert.Nil(t, client)
	require.ErrorIs(t, conn.Health(context.Background()), constants.ErrMethodNotAvailable)
}
