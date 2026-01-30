package surrealdb_test

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// nolint:gocyclo // Example covers end-to-end signup flow with version detection

func ExampleDB_SignUp_databaseLevelRecordUser() {
	// SignUp's sole purpose is to create a new record user in a database
	// that has been configured to use RECORD access method type at the database level.
	//
	// # SignIn with and without ACCESS field
	//
	// The only difference between signing in as a database user and signing in as a record user
	// is that you need to specify the Access field to indicate which access method to use for authentication.
	//
	// Like logging in as a database user defined using DEFINE USER ON DATABASE,
	// signing in as a record user also requires specifying the target namespace and database.

	db, err := surrealdb.FromEndpointURLString(
		context.Background(),
		testenv.GetSurrealDBWSURL(),
	)
	if err != nil {
		panic(err)
	}

	db, err = testenv.Init(db, "exampledb_signup_rootlevel", "testdb", "user")
	if err != nil {
		panic(err)
	}

	// Sign in as the root user
	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}

	err = db.Use(context.Background(), "exampledb_signup_rootlevel", "testdb")
	if err != nil {
		panic(fmt.Sprintf("Use failed: %v", err))
	}

	// Detect SurrealDB version to use the correct function name
	// SurrealDB 2.x uses type::thing(), SurrealDB 3.x uses type::record()
	v, err := testenv.GetVersion(context.Background(), db)
	if err != nil {
		panic(fmt.Sprintf("GetVersion failed: %v", err))
	}
	recordFn := v.ThingOrRecordFn()

	setupQuery := fmt.Sprintf(`
		-- Define the user table with schema
		DEFINE TABLE user SCHEMAFULL
			PERMISSIONS
				FOR select, update, delete WHERE id = $auth.id;

		-- Define fields
		DEFINE FIELD password ON user TYPE string;

		-- Define access method for record authentication
		REMOVE ACCESS IF EXISTS user ON DATABASE;
		DEFINE ACCESS user ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM %s("user", $user) WHERE crypto::argon2::compare(password, $pass)
			)
			SIGNUP (
				CREATE %s("user", $user) CONTENT {
					password: crypto::argon2::generate($pass)
				}
			);
	`, recordFn, recordFn)

	_, err = surrealdb.Query[any](context.Background(), db, setupQuery, nil)
	if err != nil {
		panic(fmt.Sprintf("Query failed: %v", err))
	}

	_, err = db.SignUp(context.Background(), surrealdb.Auth{
		Access:    "user",
		Namespace: "exampledb_signup_rootlevel",
		Database:  "testdb",
		Username:  "myuser",
		Password:  "mypassword",
	})
	if err != nil {
		panic(fmt.Sprintf("SignUp failed: %v", err))
	}

	_, err = db.SignIn(context.Background(), surrealdb.Auth{
		Access:    "user",
		Namespace: "exampledb_signup_rootlevel",
		Database:  "testdb",
		Username:  "myuser",
		Password:  "mypassword",
	})
	if err != nil {
		panic(fmt.Sprintf("SignIn failed: %v", err))
	}
	fmt.Println("User signed up and signed in successfully")

	// Output:
	// User signed up and signed in successfully
}
