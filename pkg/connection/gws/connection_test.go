package gws

import (
	"errors"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

// mockLogger captures log messages for testing
type mockLogger struct {
	errorLogs []string
	debugLogs []string
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorLogs = append(m.errorLogs, msg)
}

func (m *mockLogger) Debug(msg string, args ...any) {
	m.debugLogs = append(m.debugLogs, msg)
}

func (m *mockLogger) Info(msg string, args ...any) {
	// Not used in test
}

func (m *mockLogger) Warn(msg string, args ...any) {
	// Not used in test
}

var _ logger.Logger = (*mockLogger)(nil)

// mockUnmarshaler allows us to control unmarshaling behavior in tests
type mockUnmarshaler struct {
	unmarshalFunc func(data []byte, v any) error
}

func (m *mockUnmarshaler) Unmarshal(data []byte, v any) error {
	if m.unmarshalFunc != nil {
		return m.unmarshalFunc(data, v)
	}
	return nil
}

func TestHandleResponse_InvalidResponse(t *testing.T) {
	// Create a mock unmarshaler that always returns an error
	mockLog := &mockLogger{}
	mockUnmarshal := &mockUnmarshaler{
		unmarshalFunc: func(data []byte, v any) error {
			return errors.New("unmarshal error")
		},
	}

	conn := &Connection{
		Toolkit: connection.Toolkit{
			Logger:               mockLog,
			ResponseChannels:     make(map[string]chan connection.RPCResponse[cbor.RawMessage]),
			NotificationChannels: make(map[string]chan connection.Notification),
			Unmarshaler:          mockUnmarshal,
		},
	}

	// Any data will trigger the unmarshal error
	testData := []byte("test data")
	conn.handleResponse(testData)

	// Verify that error was logged and no panic occurred
	if len(mockLog.errorLogs) != 1 {
		t.Errorf("Expected 1 error log, got %d", len(mockLog.errorLogs))
	}
	if mockLog.errorLogs[0] != "Failed to unmarshal response" {
		t.Errorf("Unexpected error message: %s", mockLog.errorLogs[0])
	}
}

func TestHandleResponse_ValidResponse(t *testing.T) {
	testID := "test-123"

	// Create a mock unmarshaler that populates a valid response
	mockUnmarshal := &mockUnmarshaler{
		unmarshalFunc: func(data []byte, v any) error {
			if res, ok := v.(*connection.RPCResponse[cbor.RawMessage]); ok {
				res.ID = testID
				// We don't need to set Result for this test
			}
			return nil
		},
	}

	conn := &Connection{
		Toolkit: connection.Toolkit{
			ResponseChannels:     make(map[string]chan connection.RPCResponse[cbor.RawMessage]),
			NotificationChannels: make(map[string]chan connection.Notification),
			Unmarshaler:          mockUnmarshal,
		},
	}

	// Create response channel
	responseChan := make(chan connection.RPCResponse[cbor.RawMessage], 1)
	conn.ResponseChannels[testID] = responseChan

	// Handle the response - data doesn't matter since mock will populate the response
	conn.handleResponse([]byte("any data"))

	// Verify response was routed correctly
	select {
	case res := <-responseChan:
		if res.ID != testID {
			t.Errorf("Expected ID %s, got %v", testID, res.ID)
		}
	default:
		t.Error("Expected response to be sent to channel")
	}
}
