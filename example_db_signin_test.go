package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

func ExampleDB_SignIn_namespaceLevelUser() {
	db, err := surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "exampledb_signin_namespacelevel", "testdb", "testtable")
	if err != nil {
		panic(err)
	}

	// Login at the root level to set up the namespace-level user
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	err = db.Use(context.Background(), "exampledb_signin_namespacelevel", "")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Clean up any existing namespace-level user
	_, err = surrealdb.Query[any](context.Background(), db, `REMOVE USER IF EXISTS myuser ON NAMESPACE`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing namespace-level user: %v", err))
	}

	// Create a namespace-level user
	_, err = surrealdb.Query[any](context.Background(), db, `DEFINE USER myuser ON NAMESPACE PASSWORD 'mypassword' ROLES OWNER`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create namespace-level user: %v", err))
	}

	err = db.Close(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	// Reconnect to ensure a fresh session
	db, err = surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	// Now sign in as the namespace-level user
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Namespace: "exampledb_signin_namespacelevel",
		Username:  "myuser",
		Password:  "mypassword",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	err = db.Use(context.Background(), "exampledb_signin_namespacelevel", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	_, err = surrealdb.Query[any](context.Background(), db, `SELECT * FROM testtable`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query failed: %v", err))
	}

	if err := db.Close(context.Background()); err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	// Reconnect to ensure a fresh session
	db, err = surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Namespace: "exampledb_signin_namespacelevel",
		// Note the extra Database field here.
		// This is invalid for namespace-level users, because
		// the existence of a database signals SurrealDB to authenticate you as a database-level user,
		// which we didn't create for this test.
		Database: "testdb",
		Username: "myuser",
		Password: "mypassword",
	})
	if err == nil {
		panic("Expected SignIn to fail, but it succeeded")
	}

	if err := db.Close(context.Background()); err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	fmt.Println("Namespace-level user SignIn tests completed successfully")

	// Output:
	// Namespace-level user SignIn tests completed successfully
}

func ExampleDB_SignIn_databaseLevelUser() {
	db, err := surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "exampledb_signin_databaselevel", "testdb", "testtable")
	if err != nil {
		panic(err)
	}

	// Login at the root level to set up the namespace-level user
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	err = db.Use(context.Background(), "exampledb_signin_databaselevel", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Clean up any existing database-level user
	_, err = surrealdb.Query[any](context.Background(), db, `REMOVE USER IF EXISTS myuser ON DATABASE`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to remove existing database-level user: %v", err))
	}

	// Create a database-level user
	_, err = surrealdb.Query[any](context.Background(), db, `DEFINE USER myuser ON DATABASE PASSWORD 'mypassword' ROLES OWNER`, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create database-level user: %v", err))
	}

	err = db.Close(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	// Reconnect to ensure a fresh session
	db, err = surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	// Now sign in as the database-level user
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Namespace: "exampledb_signin_databaselevel",
		Database:  "testdb",
		Username:  "myuser",
		Password:  "mypassword",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	err = db.Use(context.Background(), "exampledb_signin_databaselevel", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	_, err = surrealdb.Query[any](context.Background(), db, `SELECT * FROM testtable`, nil)
	if err != nil {
		panic(fmt.Sprintf("Query failed: %v", err))
	}

	if err := db.Close(context.Background()); err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	// Reconnect to ensure a fresh session
	db, err = surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		// Note the omission of the Database field here.
		// This is invalid for database-level users, because
		// the database is present in a namespace.
		// Namespace: "",
		Database: "testdb",
		Username: "myuser",
		Password: "mypassword",
	})
	if err == nil {
		panic("Expected SignIn to fail, but it succeeded")
	}

	if err := db.Close(context.Background()); err != nil {
		panic(fmt.Sprintf("Failed to close the database connection: %v", err))
	}

	fmt.Println("Database-level user SignIn tests completed successfully")

	// Output:
	// Database-level user SignIn tests completed successfully
}

func ExampleDB_signin_failure() {
	db, err := surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

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
