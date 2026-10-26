package gorillaws

import (
	"errors"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

type WsTestSuite struct {
	suite.Suite
	name string
}

func TestWsTestSuite(t *testing.T) {
	ts := new(WsTestSuite)
	ts.name = "WS Test Suite"

	suite.Run(t, ts)
}

// SetupSuite is called before the s starts running
func (s *WsTestSuite) SetupSuite() {
}

func (s *WsTestSuite) TearDownSuite() {
}

// mockLogger captures log messages for testing
type mockLogger struct {
	errorLogs []string
	debugLogs []string
	infoLogs  []string
	warnLogs  []string
}

func (m *mockLogger) Error(msg string, args ...any) {
	m.errorLogs = append(m.errorLogs, msg)
}

func (m *mockLogger) Debug(msg string, args ...any) {
	m.debugLogs = append(m.debugLogs, msg)
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.infoLogs = append(m.infoLogs, msg)
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.warnLogs = append(m.warnLogs, msg)
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
			ResponseChannels:     make(map[string]chan connection.RPCResponse[cbor.RawMessage]),
			NotificationChannels: make(map[string]chan connection.Notification),
			Unmarshaler:          mockUnmarshal,
		},
		logger: mockLog,
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
		logger: &mockLogger{},
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

// TestHandleResponse_ErrorWithoutID verifies handling of errors without ID field
func TestHandleResponse_ErrorWithoutID(t *testing.T) {
	mockLog := &mockLogger{}

	// Create a mock unmarshaler that returns an error response without ID
	mockUnmarshal := &mockUnmarshaler{
		unmarshalFunc: func(data []byte, v any) error {
			if res, ok := v.(*connection.RPCResponse[cbor.RawMessage]); ok {
				//nolint:staticcheck // v2 backward compat with RPCError
				res.Error = &connection.RPCError{
					Code:    -32600,
					Message: "Invalid Request",
				}
				// No ID is set
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
		logger: mockLog,
	}

	// Handle the response
	conn.handleResponse([]byte("any data"))

	// Verify that error was logged
	if len(mockLog.errorLogs) != 1 {
		t.Errorf("Expected 1 error log, got %d", len(mockLog.errorLogs))
	}
	expectedMsg := "error in response: Invalid Request"
	if mockLog.errorLogs[0] != expectedMsg {
		t.Errorf("Expected '%s', got: %s", expectedMsg, mockLog.errorLogs[0])
	}
}

// TestHandleResponse_callerStoppedListening checks that a response for a caller
// which has stopped listening (its context expired after its channel was found)
// does not block the goroutine reading from the socket, which would hang every
// other request on the connection.
func TestHandleResponse_callerStoppedListening(t *testing.T) {
	testID := "late-1"

	mockUnmarshal := &mockUnmarshaler{
		unmarshalFunc: func(data []byte, v any) error {
			if res, ok := v.(*connection.RPCResponse[cbor.RawMessage]); ok {
				res.ID = testID
			}
			return nil
		},
	}

	conn := &Connection{
		Toolkit: connection.Toolkit{
			Logger:               &mockLogger{},
			ResponseChannels:     make(map[string]chan connection.RPCResponse[cbor.RawMessage]),
			NotificationChannels: make(map[string]chan connection.Notification),
			Unmarshaler:          mockUnmarshal,
		},
		logger: &mockLogger{},
	}

	// The caller's channel exists, but nobody will ever receive from it.
	if _, err := conn.CreateResponseChannel(testID); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		// The second, duplicate response must not block either.
		conn.handleResponse([]byte("any data"))
		conn.handleResponse([]byte("any data"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleResponse blocked on a response channel nobody reads")
	}
}
