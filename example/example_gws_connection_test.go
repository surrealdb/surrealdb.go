package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
)

func ExampleConnection_gws() {
	conf, err := surrealdb.Configure(
		getSurrealDBWSURL(),
	)
	conf.Logger = nil // Disable logging for this example
	if err != nil {
		panic(err)
	}

	conn := gws.New(conf)
	if connErr := conn.Connect(context.Background()); connErr != nil {
		panic(fmt.Sprintf("Failed to connect: %v", connErr))
	}

	db := surrealdb.New(conn)

	// Attempt to sign in without setting namespace or database
	// This should fail with an error, whose message will depend on the connection type.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "invalid",
	})
	switch err.Error() {
	case "namespace or database or both are not set":
		// In case the connection is over HTTP, this error is expected
	case "There was a problem with the database: There was a problem with authentication":
		// In case the connection is over WebSocket, this error is expected
	default:
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}

	err = db.Use(context.Background(), "testNS", "testDB")
	if err != nil {
		fmt.Println("Use error:", err)
	}

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
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	if err := db.Close(context.Background()); err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	// Output:
	// SignIn error: There was a problem with the database: There was a problem with authentication
}
