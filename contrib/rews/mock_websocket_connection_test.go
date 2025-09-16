package rews

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const testToken = "test-token"

// mockWebSocketConnection extends the mock to return proper UUIDs
type mockWebSocketConnection struct {
	connection.WebSocketConnection
	notifications map[string]chan connection.Notification
	mu            sync.Mutex
	sendCalled    int
	lastMethod    string
	lastParams    []any
	isClosed      bool
}

func (m *mockWebSocketConnection) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	m.sendCalled++
	m.lastMethod = method
	m.lastParams = params

	var result cbor.RawMessage

	switch method {
	case methodLive:
		// Return a UUID for live queries
		liveUUID := uuid.Must(uuid.NewV4())
		modelUUID := models.UUID{UUID: liveUUID}
		data, _ := cbor.Marshal(modelUUID)
		result = data

		// Store the UUID for LiveNotifications
		m.mu.Lock()
		m.notifications[liveUUID.String()] = make(chan connection.Notification, 100)
		m.mu.Unlock()

	case methodQuery:
		// Check if it's a LIVE SELECT
		if len(params) > 0 {
			if query, ok := params[0].(string); ok {
				if strings.Contains(strings.ToUpper(query), "LIVE SELECT") {
					// For LIVE SELECT, return an array of QueryResult with UUID in the Result field
					type QueryResult struct {
						Status string          `json:"status"`
						Time   string          `json:"time"`
						Result cbor.RawMessage `json:"result"`
					}

					liveUUID := uuid.Must(uuid.NewV4())
					modelUUID := models.UUID{UUID: liveUUID}
					uuidData, _ := cbor.Marshal(modelUUID)

					queryResults := []QueryResult{
						{
							Status: "OK",
							Time:   "1ms",
							Result: uuidData,
						},
					}

					data, _ := cbor.Marshal(queryResults)
					result = data

					// Store the UUID for LiveNotifications
					m.mu.Lock()
					m.notifications[liveUUID.String()] = make(chan connection.Notification, 100)
					m.mu.Unlock()
				} else {
					// Regular query result
					result = cbor.RawMessage{}
				}
			}
		}

	default:
		result = cbor.RawMessage{}
	}

	return &connection.RPCResponse[cbor.RawMessage]{
		ID:     uuid.Must(uuid.NewV4()).String(),
		Result: &result,
	}, nil
}

func (m *mockWebSocketConnection) LiveNotifications(id string) (chan connection.Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, exists := m.notifications[id]
	if !exists {
		return nil, fmt.Errorf("live query %s not found", id)
	}
	return ch, nil
}

func (m *mockWebSocketConnection) CloseLiveNotifications(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, exists := m.notifications[id]; exists {
		close(ch)
		delete(m.notifications, id)
	}
	return nil
}

func (m *mockWebSocketConnection) SendNotification(id string, notification connection.Notification) {
	m.mu.Lock()
	ch, exists := m.notifications[id]
	m.mu.Unlock()

	if exists {
		ch <- notification
	}
}

func (m *mockWebSocketConnection) IsClosed() bool {
	return m.isClosed
}

func (m *mockWebSocketConnection) Connect(ctx context.Context) error {
	m.isClosed = false
	return nil
}

func (m *mockWebSocketConnection) Close(ctx context.Context) error {
	m.isClosed = true
	return nil
}

func (m *mockWebSocketConnection) Use(ctx context.Context, ns, db string) error {
	if m.isClosed {
		return fmt.Errorf("connection is closed")
	}
	return nil
}

func (m *mockWebSocketConnection) Authenticate(ctx context.Context, token string) error {
	if m.isClosed {
		return fmt.Errorf("connection is closed")
	}
	return nil
}

func (m *mockWebSocketConnection) Let(ctx context.Context, key string, val any) error {
	if m.isClosed {
		return fmt.Errorf("connection is closed")
	}
	return nil
}

func (m *mockWebSocketConnection) Unset(ctx context.Context, key string) error {
	if m.isClosed {
		return fmt.Errorf("connection is closed")
	}
	return nil
}

func (m *mockWebSocketConnection) SignIn(ctx context.Context, auth any) (string, error) {
	if m.isClosed {
		return "", fmt.Errorf("connection is closed")
	}
	return testToken, nil
}

func (m *mockWebSocketConnection) SignUp(ctx context.Context, auth any) (string, error) {
	if m.isClosed {
		return "", fmt.Errorf("connection is closed")
	}
	return testToken, nil
}
