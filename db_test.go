package surrealdb_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go"
)

// a simple user struct for testing
type testUser struct {
	Username string
	Password string
	ID       string
}

func (t testUser) String() string {
	// TODO I found out we can use go generate stringer to generate these, but it was a bit confusing and too much
	// overhead atm, so doing this as a shortcut
	return fmt.Sprintf("testUser{Username: %+v, Password: %+v, ID: %+v}", t.Username, t.Password, t.ID)
}

func setupDB(t *testing.T) *surrealdb.DB {
	db := openConnection(t)
	_ = signin(t, db)
	_, err := db.Use(context.Background(), "test", "test")
	assert.NoError(t, err)
	return db
}

func openConnection(t *testing.T) *surrealdb.DB {
	url := os.Getenv("SURREALDB_URL")
	if url == "" {
		url = "ws://localhost:8000/rpc"
	}

	db, err := surrealdb.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func signin(t *testing.T, db *surrealdb.DB) interface{} {
	signin, err := db.Signin(context.Background(), map[string]interface{}{
		"user": "root",
		"pass": "root",
	})

	if err != nil {
		t.Fatal(err)
	}

	return signin
}

func TestDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use(ctx, "test", "test")
	assert.NoError(t, err)

	userData, err := db.Create(ctx, "users", testUser{
		Username: "johnny",
		Password: "123",
	})
	assert.NoError(t, err)

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	assert.NoError(t, err)

	// Delete the users...
	_, err = db.Delete(ctx, "users")
	assert.NoError(t, err)
}

func TestCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use(ctx, "test", "test")
	assert.NoError(t, err)

	userMap, err := db.Create(ctx, "users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, userMap)

	if err != nil || userMap == nil {
		t.Fatal(err)
	}

	userData, err := db.Create(ctx, "users", testUser{
		Username: "johnny",
		Password: "123",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, userMap)

	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	assert.NoError(t, err)
	assert.Equal(t, "johnny", user.Username)
}

func TestSelect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use(ctx, "test", "test")
	assert.NoError(t, err)

	_, err = db.Create(ctx, "users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	assert.NoError(t, err)

	userData, err := db.Select(ctx, "users")
	assert.NoError(t, err)

	// unmarshal the data into a user slice
	var users []testUser
	err = surrealdb.Unmarshal(userData, &users)
	assert.NoError(t, err)
	matching := assertContains(t, users, func(item testUser) bool {
		return item.Username == "johnnyjohn"
	})
	assert.GreaterOrEqual(t, len(matching), 1)
}

func TestUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use(ctx, "test", "test")
	assert.NoError(t, err)

	userData, err := db.Create(ctx, "users", testUser{
		Username: "johnny",
		Password: "123",
	})
	assert.NoError(t, err)

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &testUser{})
	assert.NoError(t, err)

	user.Password = "456"

	// Update the user
	userData, err = db.Update(ctx, "users", &user)
	assert.NoError(t, err)

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)
	assert.NoError(t, err)

	// TODO: check if this updates only the user with the same ID or all users
	assert.Equal(t, "456", updatedUser[0].Password)
}

func TestUnmarshalRaw(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use(ctx, "test", "test")
	assert.NoError(t, err)

	_, err = db.Delete(ctx, "users")
	assert.NoError(t, err)

	username := "johnny"
	password := "123"

	// create test user with raw SurrealQL and unmarshal
	userData, err := db.Query(ctx, "create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
		"user": username,
		"pass": password,
	})
	assert.NoError(t, err)

	var user testUser
	ok, err := surrealdb.UnmarshalRaw(userData, &user)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, username, user.Username)
	assert.Equal(t, password, user.Password)

	// send query with empty result and unmarshal
	userData, err = db.Query(ctx, "select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	assert.NoError(t, err)

	ok, err = surrealdb.UnmarshalRaw(userData, &user)
	assert.NoError(t, err)
	assert.False(t, ok, "select should return an empty result")
}

func TestModify(t *testing.T) {
	t.Skip("There is a permission issue with this test that may need to be solved in a different change")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use(ctx, "test", "test")
	assert.NoError(t, err)

	_, err = db.Delete(ctx, "users:999") // Cleanup for reproducibility
	assert.NoError(t, err)

	_, err = db.Create(ctx, "users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	assert.NoError(t, err) // TODO: permission error, "Unable to access record:users:999"

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: 44},
	}

	// Update the user
	_, err = db.Modify(ctx, "users:999", patches)
	assert.NoError(t, err)

	user2, err := db.Select(ctx, "users:999")
	assert.NoError(t, err)

	// // TODO: this needs to simplified for the end user somehow
	assert.Equal(t, "44", (user2).(map[string]interface{})["age"])
}

func TestSmartQuerySelect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := setupDB(t)
	defer db.Close()

	user := []testUser{{
		Username: "electwix",
		Password: "1234",
	}}

	// Clean up from other tests
	_, err := db.Delete(ctx, "users")
	assert.NoError(t, err)

	t.Run("raw create query", func(t *testing.T) {
		QueryStr := "Create users set Username = $user, Password = $pass"
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](db.Query(ctx, QueryStr, map[string]interface{}{
			"user": user[0].Username,
			"pass": user[0].Password,
		}))

		assert.NoError(t, err)
		assert.Equal(t, "electwix", dataArr[0].Username)
		user = dataArr
	})

	t.Run("raw select query", func(t *testing.T) {
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](db.Query(ctx, "Select * from $record", map[string]interface{}{
			"record": user[0].ID,
		}))

		assert.Equal(t, "electwix", dataArr[0].Username)
		assert.NoError(t, err)
	})

	t.Run("select query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](db.Select(ctx, user[0].ID))

		assert.Equal(t, "electwix", data.Username)
		assert.NoError(t, err)
	})

	t.Run("select array query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[[]testUser](db.Select(ctx, "users"))

		assert.Equal(t, "electwix", data[0].Username)
		assert.NoError(t, err)
	})

	t.Run("delete record query", func(t *testing.T) {
		nulldata, err := surrealdb.SmartUnmarshal[*testUser](db.Delete(ctx, user[0].ID))

		assert.NoError(t, err)
		assert.Nil(t, nulldata)
	})
}

// assertContains performs an assertion on a list, asserting that at least one element matches a provided condition.
// All the matching elements are returned from this function, which can be used as a filter.
func assertContains[K fmt.Stringer](t *testing.T, input []K, matcher func(K) bool) []K {
	matching := make([]K, 0)
	for i := range input {
		if matcher(input[i]) {
			matching = append(matching, input[i])
		}
	}
	assert.NotEmptyf(t, matching, "Input %+v did not contain matching element", fmt.Sprintf("%+v", input))
	return matching
}
