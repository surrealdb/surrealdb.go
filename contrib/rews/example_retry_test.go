package rews_test

import (
	"context"
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

func ExampleConnection_withExponentialBackoff() {
	// Create a function that establishes the WebSocket connection
	newConn := func(ctx context.Context) (connection.WebSocketConnection, error) {
		// Create a gorilla/websocket-based connection
		// ws := gorillaws.New(&connection.Config{
		//     BaseURL:     "ws://localhost:8000",
		//     Marshaler:   models.CborMarshaler{},
		//     Unmarshaler: models.CborUnmarshaler{},
		// })
		// return ws, nil
		return nil, fmt.Errorf("example connection creation")
	}

	// Create the auto-reconnecting connection
	conn := rews.New(
		newConn,
		5*time.Second, // Check interval for reconnection
		nil,           // unmarshaler
		nil,           // logger
	)

	// Configure exponential backoff for both initial connections and reconnections
	retryer := rews.NewExponentialBackoffRetryer()
	retryer.InitialDelay = 1 * time.Second
	retryer.MaxDelay = 30 * time.Second
	retryer.Multiplier = 2.0
	retryer.MaxRetries = 10 // Give up after 10 attempts for initial connection
	// Note: For different behavior between initial and reconnect, you could
	// implement a custom Retryer that tracks connection state

	// Apply the retryer
	conn.Retryer = retryer

	// Connect with automatic retry on failure
	ctx := context.Background()
	err := conn.Connect(ctx)
	if err != nil {
		// Initial connection failed after all retry attempts
		fmt.Printf("Failed to establish connection: %v\n", err)
		return
	}

	// Connection established successfully
	// The connection will now automatically reconnect with exponential backoff
	// if the connection is lost

	// Use the connection...
	// defer conn.Close(ctx)
}

func ExampleConnection_withCustomRetryer() {
	// Custom retryers can implement complex logic like:
	// - Different delays based on error types
	// - Circuit breaker patterns
	// - Adaptive retry based on success/failure history
	// - Integration with external monitoring systems
	//
	// Example structure:
	// type CustomRetryer struct {
	//     attempts int
	//     maxAttempts int
	// }
	// Implement NextDelay and Reset methods to satisfy the Retryer interface

	fmt.Println("Custom retryer can be implemented by implementing the Retryer interface")
}

func ExampleConnection_withFixedDelay() {
	// Create a function that establishes the WebSocket connection
	newConn := func(ctx context.Context) (connection.WebSocketConnection, error) {
		return nil, fmt.Errorf("example connection creation")
	}

	// Create the auto-reconnecting connection
	conn := rews.New(
		newConn,
		5*time.Second,
		nil, // unmarshaler
		nil, // logger
	)

	// Use fixed delay retryer - simple and predictable
	fixedRetryer := rews.NewFixedDelayRetryer(
		3*time.Second, // Wait 3 seconds between each retry
		5,             // Try maximum 5 times
	)

	// Apply for both initial connection and reconnection
	conn.Retryer = fixedRetryer

	// Connect with fixed delay retry
	ctx := context.Background()
	_ = conn.Connect(ctx)
}

func ExampleConnection_withNoRetry() {
	// Create a function that establishes the WebSocket connection
	newConn := func(ctx context.Context) (connection.WebSocketConnection, error) {
		return nil, fmt.Errorf("example connection creation")
	}

	// Create the auto-reconnecting connection
	conn := rews.New(
		newConn,
		5*time.Second,
		nil, // unmarshaler
		nil, // logger
	)

	// For different retry behavior between initial and reconnect,
	// you could implement a custom retryer that tracks state
	// Here we use exponential backoff for all connection attempts
	exponentialRetryer := rews.NewExponentialBackoffRetryer()

	conn.Retryer = exponentialRetryer

	// Connection attempts will use exponential backoff
	// To disable retries entirely, simply leave Retryer as nil
	ctx := context.Background()
	_ = conn.Connect(ctx)
}
