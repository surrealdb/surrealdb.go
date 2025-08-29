package rews_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

func ExampleNew() {
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL: %v", err))
	}

	conf := connection.NewConfig(u)
	// Create a logger that discards output for the example
	silentLogger := logger.New(slog.NewTextHandler(io.Discard, nil))
	conf.Logger = silentLogger

	// Create a reconnecting WebSocket connection using rews
	// The first argument is a constructor function that creates a new gws connection
	conn := rews.New(
		func(ctx context.Context) (*gws.Connection, error) {
			return gws.New(conf), nil
		},
		5*time.Second,    // Check interval for reconnection attempts
		conf.Unmarshaler, // CBOR unmarshaler
		silentLogger,     // Logger (discards output in this example)
	)

	// Connect to the database
	err = conn.Connect(context.Background())
	fmt.Println("Connect error:", err)

	db, err := surrealdb.FromConnection(context.Background(), conn)
	fmt.Println("FromConnection error:", err)

	// Attempt to sign in without setting namespace or database
	// This should fail with an error, whose message will depend on the connection type.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "invalid",
	})
	fmt.Println("SignIn error:", err)

	err = db.Use(context.Background(), "testNS", "testDB")
	fmt.Println("Use error:", err)

	// Even though the ns/db is set, the SignIn should still fail
	// when the credentials are invalid.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "invalid",
	})
	fmt.Println("SignIn error:", err)

	// Now let's try with the correct credentials
	// This should succeed if the database is set up correctly.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	fmt.Println("SignIn error:", err)

	// The rews connection automatically handles reconnection,
	// so even if the connection drops, it will attempt to reconnect
	// and restore any active live queries.

	err = db.Close(context.Background())
	fmt.Println("Close error:", err)

	// Output:
	// Connect error: <nil>
	// FromConnection error: <nil>
	// SignIn error: rews.Connection failed to sign in: There was a problem with the database: There was a problem with authentication
	// Use error: <nil>
	// SignIn error: rews.Connection failed to sign in: There was a problem with the database: There was a problem with authentication
	// SignIn error: <nil>
	// Close error: <nil>
}
