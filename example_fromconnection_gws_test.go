package surrealdb_test

import (
	"context"
	"fmt"
	"net/url"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
)

// FromConnection can take any connection.Connection implementation, including
// gws.Connection which is based on https://github.com/lxzan/gws.
func ExampleFromConnection_alternativeWebSocketLibrary_gws() {
	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse URL: %v", err))
	}

	conf := connection.NewConfig(u)
	conf.Logger = nil // Disable logging for this example

	conn := gws.New(conf)

	db, err := surrealdb.FromConnection(context.Background(), conn)
	fmt.Println("FromConnection error:", err)

	// normalizeAuthError normalizes authentication error messages for version compatibility
	// SurrealDB 2.x: "There was a problem with the database: There was a problem with authentication"
	// SurrealDB 3.x: "There was a problem with authentication"
	normalizeAuthError := func(err error) string {
		if err == nil {
			return "<nil>"
		}
		errMsg := err.Error()
		//nolint:goconst // Keeping error messages inline for readability in examples
		switch errMsg {
		case "There was a problem with the database: There was a problem with authentication":
			return "authentication failed"
		case "There was a problem with authentication":
			return "authentication failed"
		}
		return errMsg
	}

	// Attempt to sign in without setting namespace or database
	// This should fail with an error, whose message will depend on the connection type.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "invalid",
	})
	fmt.Println("SignIn error:", normalizeAuthError(err))

	err = db.Use(context.Background(), "testNS", "testDB")
	fmt.Println("Use error:", err)

	// Even though the ns/db is set, the SignIn should still fail
	// when the credentials are invalid.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "invalid",
	})
	fmt.Println("SignIn error:", normalizeAuthError(err))

	// Now let's try with the correct credentials
	// This should succeed if the database is set up correctly.
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	fmt.Println("SignIn error:", normalizeAuthError(err))

	err = db.Close(context.Background())
	fmt.Println("Close error:", err)

	// Output:
	// FromConnection error: <nil>
	// SignIn error: authentication failed
	// Use error: <nil>
	// SignIn error: authentication failed
	// SignIn error: <nil>
	// Close error: <nil>
}
