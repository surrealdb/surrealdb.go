package fakesdb_test

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/internal/fakesdb"
)

func ExampleServer() {
	// Create a new fake SurrealDB server
	server := fakesdb.NewServer("127.0.0.1:18080")

	// Add a stub with custom matcher for query method
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("query", func(params []interface{}) bool {
			// Match only queries for users table
			if len(params) > 0 {
				if query, ok := params[0].(string); ok {
					return contains(query, "users")
				}
			}
			return false
		}),
		Response: map[string]interface{}{
			"result": []interface{}{
				map[string]interface{}{"id": "user:1", "name": "Alice"},
				map[string]interface{}{"id": "user:2", "name": "Bob"},
			},
		},
	})

	// Add failure injection for testing resilience
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher:  fakesdb.MatchMethod("create"),
		Response: map[string]interface{}{"id": "record:1"},
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureResponseDelay,
				Probability: 0.3, // 30% chance of delay
				MinDelay:    100 * time.Millisecond,
				MaxDelay:    500 * time.Millisecond,
			},
		},
	})

	// Set global failures that apply to all requests
	server.SetGlobalFailures([]fakesdb.FailureConfig{
		{
			Type:        fakesdb.FailureRequestDelay,
			Probability: 0.1, // 10% chance of request delay
			MinDelay:    50 * time.Millisecond,
			MaxDelay:    150 * time.Millisecond,
		},
	})

	// Start the server
	if err := server.Start(); err != nil {
		panic(err)
	}
	defer server.Stop()

	// Get the server address for client connection
	addr := server.Address()
	wsURL := fmt.Sprintf("ws://%s/rpc", addr)
	fmt.Printf("Fake SurrealDB server running at: %s\n", wsURL)

	// Create your WebSocket client with auto-reconnection enabled
	// and test that it properly handles the connection drops
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Your test code here that uses the WebSocket connection
	// The fake server will randomly drop connections to test reconnection logic
	_ = ctx

	// Output:
	// Fake SurrealDB server running at: ws://127.0.0.1:18080/rpc
}

func ExampleServer_connectionFailures() {
	server := fakesdb.NewServer("127.0.0.1:18081")

	// Simulate WebSocket close on query requests
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher:  fakesdb.MatchMethod("query"),
		Response: map[string]interface{}{"result": []interface{}{}},
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureWebSocketClose,
				Probability: 1.0, // Always close connection
				CloseCode:   1001,
				CloseReason: "Server going away",
			},
		},
	})

	// Simulate TCP reset
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher:  fakesdb.MatchMethod("delete"),
		Response: map[string]interface{}{"ok": true},
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureTCPReset,
				Probability: 0.5, // 50% chance of TCP reset
			},
		},
	})

	// Simulate corrupted messages
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher:  fakesdb.MatchMethod("update"),
		Response: map[string]interface{}{"updated": true},
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureCorruptedMessage,
				Probability: 0.2, // 20% chance of corruption
			},
		},
	})

	if err := server.Start(); err != nil {
		panic(err)
	}
	defer server.Stop()

	// Get the server address for client connection
	addr := server.Address()
	wsURL := fmt.Sprintf("ws://%s/rpc", addr)
	fmt.Printf("Fake SurrealDB server running at: %s\n", wsURL)

	// Create your WebSocket client with auto-reconnection enabled
	// and test that it properly handles the connection drops
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Your test code here that uses the WebSocket connection
	// The fake server will randomly drop connections to test reconnection logic
	_ = ctx

	// Output:
	// Fake SurrealDB server running at: ws://127.0.0.1:18081/rpc
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
