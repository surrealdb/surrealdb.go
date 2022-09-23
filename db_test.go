package surrealdb_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
)

// a simple user struct for testing
type testUser struct {
	Username string
	Password string
	ID       string
}

// an example test for creating a new entry in surrealdb
func Test_ExampleNew(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)
	defer db.Close()

	// Output:
}

func Test_ExampleDB_Delete(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	require.Nil(t, err)

	_, err = db.Use("test", "test")
	require.Nil(t, err)

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	require.Nil(t, err)

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	require.Nil(t, err)

	// Delete the users...
	_, err = db.Delete("users")
	require.Nil(t, err)

	// Output:
}

func Test_ExampleDB_Create(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)

	defer db.Close()

	signin, err := db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	require.Nil(t, err)

	_, err = db.Use("test", "test")
	require.Nil(t, err)
	assert.NotNil(t, signin)

	userMap, err := db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	require.Nil(t, err)
	assert.NotNil(t, userMap)

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	require.Nil(t, err)

	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	require.Nil(t, err)

	fmt.Println(user.Username)

	// Output: johnny
}

func Test_ExampleDB_Select(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	require.Nil(t, err)

	_, err = db.Use("test", "test")
	require.Nil(t, err)

	_, err = db.Create("users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	require.Nil(t, err)

	userData, err := db.Select("users")
	require.Nil(t, err)

	// unmarshal the data into a user slice
	var users []testUser
	err = surrealdb.Unmarshal(userData, &users)
	require.Nil(t, err)

	for _, user := range users {
		if user.Username == "johnnyjohn" {
			fmt.Println(user.Username)
			break
		}
	}
	// Output: johnnyjohn
}

func Test_ExampleDB_Update(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	require.Nil(t, err)

	_, err = db.Use("test", "test")
	require.Nil(t, err)

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	require.Nil(t, err)

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &testUser{})
	require.Nil(t, err)

	user.Password = "456"

	// Update the user
	userData, err = db.Update("users", &user)
	require.Nil(t, err)

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)
	require.Nil(t, err)

	// TODO: check if this updates only the user with the same ID or all users
	fmt.Println(updatedUser[0].Password)

	// Output: 456
}

func Test_UnmarshalRaw(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	require.Nil(t, err)

	_, err = db.Use("test", "test")
	require.Nil(t, err)

	_, err = db.Delete("users")
	require.Nil(t, err)

	username := "johnny"
	password := "123"

	//create test user with raw SurrealQL and unmarshal

	userData, err := db.Query("create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
		"user": username,
		"pass": password,
	})
	require.Nil(t, err)

	var user testUser
	ok, err := surrealdb.UnmarshalRaw(userData, &user)
	require.Nil(t, err)
	assert.True(t, ok)
	assert.Equal(t, username, user.Username)
	assert.Equal(t, password, user.Password)

	//send query with empty result and unmarshal

	userData, err = db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	require.Nil(t, err)

	ok, err = surrealdb.UnmarshalRaw(userData, &user)
	require.Nil(t, err)
	assert.False(t, ok)

	// Output:
}

func Test_ExampleDB_Modify(t *testing.T) {
	db, err := surrealdb.New("ws://localhost:8000/rpc")
	require.Nil(t, err)
	defer db.Close()

	_, err = db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	require.Nil(t, err)
	_, err = db.Use("test", "test")
	require.Nil(t, err)

	_, err = db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	require.Nil(t, err)

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: 44},
	}

	// Update the user
	_, err = db.Modify("users:999", patches)
	require.Nil(t, err)

	user2, err := db.Select("users:999")
	require.Nil(t, err)

	// // TODO: this needs to simplified for the end user somehow
	fmt.Println((user2).(map[string]interface{})["age"])
	//
	// Output: 44
}
