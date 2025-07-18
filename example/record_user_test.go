package main

import (
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

//nolint:funlen
func ExampleDB_recordAuthentication() {
	db := newSurrealDBWSConnection("record_auth_demo", "user")

	setupQuery := `
		-- Define the user table with schema
		DEFINE TABLE user SCHEMAFULL
			PERMISSIONS
				FOR select, update, delete WHERE id = $auth.id;

		-- Define fields
		DEFINE FIELD name ON user TYPE string;
		DEFINE FIELD password ON user TYPE string;

		-- Define unique index on email
		REMOVE INDEX IF EXISTS name ON user;
		DEFINE INDEX name ON user FIELDS name UNIQUE;

		-- Define access method for record authentication
		REMOVE ACCESS IF EXISTS user ON DATABASE;
		DEFINE ACCESS user ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM user WHERE name = $user AND crypto::argon2::compare(password, $pass)
			)
			SIGNUP (
				CREATE user CONTENT {
					name: $user,
					password: crypto::argon2::generate($pass)
				}
			);
	`

	if _, err := surrealdb.Query[any](db, setupQuery, nil); err != nil {
		panic(err)
	}

	fmt.Println("Database schema setup complete")

	// TODO: We might need to add support for auth data other than Auth struct,
	// when you need to use fields other than `user` and `pass` in the query specified for SIGNUP.
	_, err := db.SignUp(&map[string]any{
		"Namespace": "examples",
		"Database":  "record_auth_demo",
		"Access":    "user",
		"Username":  "yusuke",
		"Password":  "VerySecurePassword123!",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("User signed up successfully")

	// TODO: We might need to add support for auth data other than Auth struct,
	// when you need to use fields other than `user` and `pass` in the query specified for SIGNIN.
	// For example, you might want to use `email` and `password` instead of `user` and `pass`.
	// In that case, you need to something that encodes to a cbor map containing those keys.
	_, err = db.SignIn(&surrealdb.Auth{
		Namespace: "examples",
		Database:  "record_auth_demo",
		Access:    "user",
		Username:  "yusuke",
		Password:  "VerySecurePassword123!",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("User signed in successfully")

	info, err := db.Info()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Authenticated user name: %v\n", info["name"])

	// Output:
	// Database schema setup complete
	// User signed up successfully
	// User signed in successfully
	// Authenticated user name: yusuke
}
