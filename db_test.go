package surrealdb_test

import (
	"encoding/json"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
)

// a simple user struct for testing
type testUser struct {
	Username string
	Password string
	ID       string
}

// an example test for creating a new entry in surrealdb
func ExampleNew() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Output:
}

func ExampleDB_Create() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	signin, err := db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})

	if err != nil || signin == nil {
		panic(err)
	}

	_, err = db.Use("test", "test")
	if err != nil {
		panic(err)
	}

	userMap, err := db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})

	if err != nil || userMap == nil {
		panic(err)
	}

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})

	// marshal the data into a JSON string
	jsonString, err := json.Marshal(userData)

	// unmarshal the data into a user struct
	var user testUser
	err = json.Unmarshal(jsonString, &user)

	if err != nil {
		panic(err)
	}

	fmt.Println(user.Username)

	// Output: johnny
}

func ExampleDB_Select() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use("test", "test")

	if err != nil {
		panic(err)
	}

	_, err = db.Create("users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})

	userData, err := db.Select("users") // TODO: should let users specify a selector other than '*'

	// marshal createdUser into a JSON string
	jsonString, err := json.Marshal(userData)

	// unmarshal the data into a user struct
	var selectedTestUsers []testUser
	err = json.Unmarshal(jsonString, &selectedTestUsers)

	if err != nil {
		panic(err)
	}

	if err != nil {
		panic(err)
	}
	for _, user := range selectedTestUsers {
		if user.Username == "johnnyjohn" {
			fmt.Println(user.Username)
			break
		}
	}
	// Output: johnnyjohn
}

func ExampleDB_Update() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use("test", "test")

	if err != nil {
		panic(err)
	}

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})

	jsonString, err := json.Marshal(userData)

	// unmarshal the data into a user struct
	var user testUser
	err = json.Unmarshal(jsonString, &user)

	if err != nil {
		panic(err)
	}
	user.Password = "456"
	// Update the user
	userData, err = db.Update("users", user)

	if err != nil {
		panic(err)
	}

	// marshal the data into a JSON string
	jsonString, err = json.Marshal(userData)

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = json.Unmarshal(jsonString, &updatedUser)

	// marshal the data into a JSON string

	if err != nil {
		panic(err)
	}

	// TODO: this needs to simplified for the end user somehow
	fmt.Println(updatedUser[0].Password)

	// Output: 456
}

func ExampleDB_Delete() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use("test", "test")

	if err != nil {
		panic(err)
	}

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})

	jsonString, err := json.Marshal(userData)

	// unmarshal the data into a user struct
	var user testUser
	err = json.Unmarshal(jsonString, &user)

	if err != nil {
		panic(err)
	}

	// Delete the user
	_, err = db.Delete("users")

	if err != nil {
		panic(err)
	}

	// Output:
}