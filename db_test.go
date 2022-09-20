package surrealdb

import (
	"fmt"
	"testing"
)

// an example test for creating a new entry in surrealdb
func Test_New(t *testing.T) {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	defer db.Close()

	// Output:
}

func TestDB_Delete(t *testing.T) {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	_, err = db.Use("test", "test")

	_, err = db.Delete("users")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}

	// Output:
}

func TestDB_Create(t *testing.T) {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
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
		t.Errorf("Unexpected error: %s", err.Error())
	}

	// Output:
}

func TestDB_Select(t *testing.T) {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
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
		t.Errorf("Unexpected error: %s", err.Error())
	}
	user, err := db.Select("users") // TODO: should let users specify a selector other than '*'
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}

	// TODO: this needs to simplified for the end user somehow
	fmt.Println((user).([]interface{})[0].(map[string]interface{})["username"])

	// Output: john
}

func TestDB_Update(t *testing.T) {
	db, err := New("ws://localhost:8000/rpc")
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
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
		t.Errorf("Unexpected error: %s", err.Error())
	}
	user, err := db.Select("users") // // TODO: should let users specify a selector other than '*'
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}

	// Update the user
	user, err = db.Update("users", map[string]interface{}{
		"username": "john",
		"password": "1234",
	})

	// TODO: this needs to simplified for the end user somehow
	fmt.Println((user).([]interface{})[0].(map[string]interface{})["password"])

	// Output: 1234
}
