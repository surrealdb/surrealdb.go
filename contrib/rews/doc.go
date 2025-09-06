// Package rews provides a reliable, auto-reconnecting WebSocket connection for SurrealDB
// with support for session restoration, live query persistence, and customizable retry strategies.
//
// The main component is Connection, which wraps a standard WebSocket connection and adds:
//   - Automatic reconnection when the connection is lost
//   - Session state restoration (namespace, database, authentication, variables)
//   - Live query persistence across reconnections
//   - Customizable retry strategies with exponential backoff
//
// Basic usage:
//
//	// Create a connection with exponential backoff retry
//	conn := rews.New(
//	    func(ctx context.Context) (connection.WebSocketConnection, error) {
//	        ws := gorillaws.New(&connection.Config{
//	            BaseURL:     "ws://localhost:8000",
//	            Marshaler:   marshaler,
//	            Unmarshaler: unmarshaler,
//	        })
//	        return ws, nil
//	    },
//	    5*time.Second,  // reconnection check interval
//	    unmarshaler,
//	    logger,
//	)
//
//	// Configure retry behavior
//	retryer := rews.NewExponentialBackoffRetryer()
//	retryer.MaxRetries = 10
//	conn.Retryer = retryer
//
//	// Connect with automatic retry on failure
//	if err := conn.Connect(ctx); err != nil {
//	    // Handle connection failure after all retries
//	}
//
//	// Use the connection - it will automatically reconnect if disconnected
//	conn.Use(ctx, "namespace", "database")
//	conn.SignIn(ctx, credentials)
//
// The package includes several built-in retry strategies:
//   - ExponentialBackoffRetryer: Exponential backoff with jitter
//   - FixedDelayRetryer: Fixed delay between retries
//   - nil: No retries (fail immediately on first error)
//
// Custom retry strategies can be implemented by satisfying the [Retryer] interface.
package rews
