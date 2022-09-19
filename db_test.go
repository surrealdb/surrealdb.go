package surrealdb

import (
	"fmt"
)

// an example test for creating a new entry in surrealdb
func ExampleNew() {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Output:
}

func ExampleDB_Delete() {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	_, err = db.Use("test", "test")

	_, err = db.Delete("users")
	if err != nil {
		panic(err)
	}

	// Output:
}

func ExampleDB_Create() {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	_, err = db.Use("test", "test")

	_, err = db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	if err != nil {
		fmt.Println(err)
	}

	// Output:
}

func ExampleDB_Select() {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	_, err = db.Use("test", "test")

	_, err = db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	if err != nil {
		panic(err)
	}
	user, err := db.Select("users") // TODO: should let users specify a selector other than '*'
	if err != nil {
		panic(err)
	}

	// TODO: this needs to simplified for the end user somehow
	fmt.Println((user).([]interface{})[0].(map[string]interface{})["username"])

	// Output: john
}

func ExampleDB_Update() {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	_, err = db.Use("test", "test")

	_, err = db.Create("users:123", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	if err != nil {
		panic(err)
	}

	// Update the user
	user, err := db.Update("users:123", map[string]interface{}{
		"username": "john",
		"password": "1234",
	})

	if err != nil {
		panic(err)
	}

	// TODO: this needs to simplified for the end user somehow
	fmt.Println((user).(map[string]interface{})["password"])

	// Output: 1234
}

func ExampleDB_Modify() {
	db, err := New("ws://localhost:8000/rpc")
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

	patches := []Patch{
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
