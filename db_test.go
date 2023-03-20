package surrealdb_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go"
)

// a simple user struct for testing
type testUser struct {
	surrealdb.Basemodel `table:"test"`
	Username            string
	Password            string
	ID                  string
}

func (t testUser) String() string {
	// TODO I found out we can use go generate stringer to generate these, but it was a bit confusing and too much
	// overhead atm, so doing this as a shortcut
	return fmt.Sprintf("testUser{Username: %+v, Password: %+v, ID: %+v}", t.Username, t.Password, t.ID)
}

func setupDB(t *testing.T) *surrealdb.DB {
	db := openConnection(t)
	_ = signin(t, db)
	_, err := db.Use("test", "test")
	assert.NoError(t, err)
	return db
}

func openConnection(t *testing.T) *surrealdb.DB {
	url := os.Getenv("SURREALDB_URL")
	if url == "" {
		url = "ws://localhost:8000/rpc"
	}

	db, err := surrealdb.New(url)
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func signin(t *testing.T, db *surrealdb.DB) interface{} {
	signin, err := db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})

	if err != nil {
		t.Fatal(err)
	}

	return signin
}

func TestDelete(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	assert.NoError(t, err)

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	assert.NoError(t, err)

	// Delete the users...
	_, err = db.Delete("users")
	assert.NoError(t, err)
}

func TestCreate(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	userMap, err := db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, userMap)

	userData, err := db.Create("users", testUser{
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
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	_, err = db.Create("users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	assert.NoError(t, err)

	userData, err := db.Select("users")
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
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	userData, err := db.Create("users", testUser{
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
	userData, err = db.Update("users", &user)
	assert.NoError(t, err)

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)
	assert.NoError(t, err)

	// TODO: check if this updates only the user with the same ID or all users
	assert.Equal(t, "456", updatedUser[0].Password)
}

func TestUnmarshalRaw(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	_, err = db.Delete("users")
	assert.NoError(t, err)

	username := "johnny"
	password := "123"

	// create test user with raw SurrealQL and unmarshal
	userData, err := db.Query("create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
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
	userData, err = db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	assert.NoError(t, err)

	ok, err = surrealdb.UnmarshalRaw(userData, &user)
	assert.NoError(t, err)
	assert.False(t, ok, "select should return an empty result")
}

func TestModify(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	_, err = db.Delete("users:999") // Cleanup for reproducibility
	assert.NoError(t, err)

	_, err = db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	assert.NoError(t, err)

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: int(44)},
	}

	// Update the user
	_, err = db.Modify("users:999", patches)
	assert.NoError(t, err)

	user2, err := db.Select("users:999")
	assert.NoError(t, err)

	data := (user2).(map[string]interface{})["age"].(float64)

	assert.Equal(t, patches[1].Value, int(data))
}

func TestNonRowSelect(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	assert.NoError(t, err)

	user := testUser{
		Username: "ElecTwix",
		Password: "1234",
		ID:       "users:notexists",
	}

	_, err = db.Select("users:notexists")
	assert.Equal(t, err, surrealdb.ErrNoRow)

	_, err = surrealdb.SmartUnmarshal[testUser](db.Select("users:notexists"))
	assert.Equal(t, err, surrealdb.ErrNoRow)

	_, err = surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Select, user))
	assert.Equal(t, err, surrealdb.ErrNoRow)
}

func TestSmartUnMarshalQuery(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	user := []testUser{{
		Username: "electwix",
		Password: "1234",
	}}

	// Clean up from other tests
	_, err := db.Delete("users")
	assert.NoError(t, err)

	t.Run("raw create query", func(t *testing.T) {
		QueryStr := "Create users set Username = $user, Password = $pass"
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](db.Query(QueryStr, map[string]interface{}{
			"user": user[0].Username,
			"pass": user[0].Password,
		}))

		assert.NoError(t, err)
		assert.Equal(t, "electwix", dataArr[0].Username)
		user = dataArr
	})

	t.Run("raw select query", func(t *testing.T) {
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](db.Query("Select * from $record", map[string]interface{}{
			"record": user[0].ID,
		}))

		assert.Equal(t, "electwix", dataArr[0].Username)
		assert.NoError(t, err)
	})

	t.Run("select query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](db.Select(user[0].ID))

		assert.Equal(t, "electwix", data.Username)
		assert.NoError(t, err)
	})

	t.Run("select array query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[[]testUser](db.Select("users"))

		assert.Equal(t, "electwix", data[0].Username)
		assert.NoError(t, err)
	})

	t.Run("delete record query", func(t *testing.T) {
		nulldata, err := surrealdb.SmartUnmarshal[*testUser](db.Delete(user[0].ID))

		assert.NoError(t, err)
		assert.Nil(t, nulldata)
	})
}

func TestSmartMarshalQuery(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	user := []testUser{{
		Username: "electwix",
		Password: "1234",
		ID:       "sometable:someid",
	}}

	// Clean up from other tests
	_, err := db.Delete("users")
	assert.NoError(t, err)

	t.Run("create with SmartMarshal query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Create, user[0]))
		assert.NoError(t, err)
		assert.Equal(t, user[0], data)
	})

	t.Run("select with SmartMarshal query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Select, user[0]))
		assert.NoError(t, err)
		assert.Equal(t, user[0], data)
	})

	t.Run("select with nil pointer SmartMarshal query", func(t *testing.T) {
		var nilptr *testUser
		data, err := surrealdb.SmartUnmarshal[*testUser](surrealdb.SmartMarshal(db.Select, &nilptr))
		assert.Equal(t, err, surrealdb.ErrNotStruct)
		assert.Equal(t, nilptr, data)
	})

	t.Run("select with pointer SmartMarshal query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[*testUser](surrealdb.SmartMarshal(db.Select, &user[0]))
		assert.NoError(t, err)
		assert.Equal(t, &user[0], data)
	})

	t.Run("update with SmartMarshal query", func(t *testing.T) {
		user[0].Password = "test123"
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Update, user[0]))
		assert.NoError(t, err)
		assert.Equal(t, user[0].Password, data.Password)
	})

	t.Run("delete with SmartMarshal query", func(t *testing.T) {
		nulldata, err := surrealdb.SmartMarshal(db.Delete, &user[0])
		assert.NoError(t, err)
		assert.Nil(t, nulldata)
	})

	t.Run("check if data deleted SmartMarshal query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Select, user[0]))
		assert.Equal(t, err, surrealdb.ErrNoRow)
		assert.Equal(t, data, testUser{})
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
