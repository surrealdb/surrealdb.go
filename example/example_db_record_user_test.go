package main

import (
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

//nolint:funlen
func ExampleDB_record_user_auth_struct() {
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

	// Refer to the next example, `ExampleDB_record_user_custom_struct`,
	// when you need to use fields other than `user` and `pass` in the query specified for SIGNUP.
	_, err := db.SignUp(&surrealdb.Auth{
		Namespace: "examples",
		Database:  "record_auth_demo",
		Access:    "user",
		Username:  "yusuke",
		Password:  "VerySecurePassword123!",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("User signed up successfully")

	// Refer to the next example, `ExampleDB_record_user_custom_struct`,
	// when you need to use fields other than `user` and `pass` in the query specified for SIGNIN.
	//
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

func ExampleDB_record_user_custom_struct() {
	db := newSurrealDBWSConnection("record_user_custom", "user")

	setupQuery := `
		-- Define the user table with schema
		DEFINE TABLE user SCHEMAFULL
			PERMISSIONS
				FOR select, update, delete WHERE id = $auth.id;

		-- Define fields
		DEFINE FIELD name ON user TYPE string;
		DEFINE FIELD email ON user TYPE string;
		DEFINE FIELD password ON user TYPE string;

		-- Define unique index on email
		REMOVE INDEX IF EXISTS email ON user;
		DEFINE INDEX email ON user FIELDS email UNIQUE;

		-- Define access method for record authentication
		REMOVE ACCESS IF EXISTS user ON DATABASE;
		DEFINE ACCESS user ON DATABASE TYPE RECORD
			SIGNIN (
				SELECT * FROM user WHERE email = $email AND crypto::argon2::compare(password, $password)
			)
			SIGNUP (
				CREATE user CONTENT {
					name: $name,
					email: $email,
					password: crypto::argon2::generate($password)
				}
			);
	`

	if _, err := surrealdb.Query[any](db, setupQuery, nil); err != nil {
		panic(err)
	}

	fmt.Println("Database schema setup complete")

	type User struct {
		Namespace string `json:"NS"`
		Database  string `json:"DB"`
		Access    string `json:"AC"`
		Name      string `json:"name"`
		Password  string `json:"password"`
		Email     string `json:"email"`
	}

	type LoginRequest struct {
		Namespace string `json:"NS"`
		Database  string `json:"DB"`
		Access    string `json:"AC"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}

	_, err := db.SignUp(&User{
		// Corresponds to the SurrealDB namespace
		Namespace: "examples",
		// Corresponds to the SurrealDB database
		Database: "record_user_custom",
		// Corresponds to `user` in `DEFINE ACCESS USER ON ...`
		Access: "user",
		// Corresponds to the $name in the SIGNUP query and `name` in `DEFINE FIELD name ON user`
		Name: "yusuke",
		// Corresponds to the $password in the SIGNUP query and `password` in `DEFINE FIELD password ON user`
		Password: "VerySecurePassword123!",
		// Corresponds to the $email in the SIGNUP query and `email` in `DEFINE FIELD email ON user`
		Email: "yusuke@example.com",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("User signed up successfully")

	_, err = db.SignIn(&LoginRequest{
		Namespace: "examples",
		Database:  "record_user_custom",
		Access:    "user",
		// Corresponds to the $email in the SIGNIN query and `email` in `DEFINE FIELD email ON user`
		Email: "yusuke@example.com",
		// Corresponds to the $password in the SIGNIN query and `password` in `DEFINE FIELD password ON user`
		Password: "VerySecurePassword123!",
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
