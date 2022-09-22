package surrealdb_test

import (
	"fmt"
	"github.com/surrealdb/surrealdb.go"
	"testing"
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

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	if err != nil {
		panic(err)
	}

	// Delete the users...
	_, err = db.Delete("users")

	if err != nil {
		panic(err)
	}

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

	if err != nil {
		panic(err)
	}

	_, err = db.Use("test", "test")

	if err != nil || signin == nil {
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

	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
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

	userData, err := db.Select("users")

	// unmarshal the data into a user slice
	var users []testUser
	err = surrealdb.Unmarshal(userData, &users)
	if err != nil {
		panic(err)
	}

	for _, user := range users {
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

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &testUser{})
	if err != nil {
		panic(err)
	}

	user.Password = "456"

	// Update the user
	userData, err = db.Update("users", &user)

	if err != nil {
		panic(err)
	}

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)

	if err != nil {
		panic(err)
	}

	// TODO: check if this updates only the user with the same ID or all users
	fmt.Println(updatedUser[0].Password)

	// Output: 456
}

func TestUnmarshalRaw(t *testing.T) {
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

	_, err = db.Delete("users")
	if err != nil {
		panic(err)
	}

	username := "johnny"
	password := "123"

	//create test user with raw SurrealQL and unmarshal

	userData, err := db.Query("create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
		"user": username,
		"pass": password,
	})
	if err != nil {
		panic(err)
	}

	var user testUser
	ok, err := surrealdb.UnmarshalRaw(userData, &user)
	if err != nil {
		panic(err)
	}
	if !ok || user.Username != username || user.Password != password {
		panic("response does not match the request")
	}

	//send query with empty result and unmarshal

	userData, err = db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	if err != nil {
		panic(err)
	}

	ok, err = surrealdb.UnmarshalRaw(userData, &user)
	if err != nil {
		panic(err)
	}
	if ok {
		panic("select should return an empty result")
	}

	// Output:
}

func ExampleDB_Modify() {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	_, err = db.Use("test", "test")

	_, err = db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	if err != nil {
		panic(err)
	}

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: 44},
	}

	// Update the user
	_, err = db.Modify("users:999", patches)
	if err != nil {
		panic(err)
	}

	user2, err := db.Select("users:999")
	if err != nil {
		panic(err)
	}

	// // TODO: this needs to simplified for the end user somehow
	fmt.Println((user2).(map[string]interface{})["age"])
	//
	// Output: 44
}
