package surrealdb

import "fmt"

// an example test for creating a new entry in surrealdb
func ExampleNew() {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

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
		panic(err)
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

	_, err = db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	if err != nil {
		panic(err)
	}
	user, err := db.Select("users") // // TODO: should let users specify a selector other than '*'
	if err != nil {
		panic(err)
	}

	// Update the user
	_, err = db.Update("users", map[string]interface{}{
		"username": "john",
		"password": "1234",
	})

	// TODO: this needs to simplified for the end user somehow
	fmt.Println((user).([]interface{})[0].(map[string]interface{})["password"])

	// Output: 1234
}
