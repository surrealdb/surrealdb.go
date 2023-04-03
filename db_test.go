package surrealdb_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go"
)

// a simple user struct for testing
type testUser struct {
	surrealdb.Basemodel `table:"test"`
	Username            string `json:"username,omitempty"`
	Password            string `json:"password,omitempty"`
	ID                  string `json:"id,omitempty"`
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
	require.NoError(t, err)
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
	require.NoError(t, err)

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	require.NoError(t, err)

	// unmarshal the data into a user struct
	var user []testUser
	err = surrealdb.Unmarshal(userData, &user)
	require.NoError(t, err)

	// Delete the users...
	_, err = db.Delete("users")
	require.NoError(t, err)
}

func TestCreate(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	require.NoError(t, err)

	t.Run("raw map works", func(t *testing.T) {
		userData, err := db.Create("users", map[string]interface{}{
			"username": "johnny",
			"password": "123",
		})
		require.NoError(t, err)

		// unmarshal the data into a user struct
		var userSlice []testUser
		err = surrealdb.Unmarshal(userData, &userSlice)
		require.NoError(t, err)
		assert.Len(t, userSlice, 1)

		assert.Equal(t, "johnny", userSlice[0].Username)
		assert.Equal(t, "123", userSlice[0].Password)
	})

	t.Run("Single create works", func(t *testing.T) {
		userData, err := db.Create("users", testUser{
			Username: "johnny",
			Password: "123",
		})
		require.NoError(t, err)

		// unmarshal the data into a user struct
		var userSlice []testUser
		err = surrealdb.Unmarshal(userData, &userSlice)
		require.NoError(t, err)
		assert.Len(t, userSlice, 1)

		assert.Equal(t, "johnny", userSlice[0].Username)
		assert.Equal(t, "123", userSlice[0].Password)
	})

	t.Run("Multiple creates works", func(t *testing.T) {
		t.Skip("Creating multiple records is not supported yet")
		data := make([]testUser, 0)
		data = append(data, testUser{
			Username: "johnny",
			Password: "123",
		})
		data = append(data, testUser{
			Username: "joe",
			Password: "123",
		})
		userData, err := db.Create("users", data)
		require.NoError(t, err)

		// unmarshal the data into a user struct
		var users []testUser
		err = surrealdb.Unmarshal(userData, &users)
		require.NoError(t, err)
		assertContains(t, users, func(user testUser) bool {
			return user == data[0] ||
				user == data[1]
		})
	})
}

func TestSelect(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	require.NoError(t, err)

	createdUsers, err := db.Create("users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, createdUsers)
	var createdUsersUnmarshalled []testUser
	require.NoError(t, surrealdb.Unmarshal(createdUsers, &createdUsersUnmarshalled))
	assert.NotEmpty(t, createdUsersUnmarshalled)
	assert.NotEmpty(t, createdUsersUnmarshalled[0].ID, "The ID should have been set by the database")

	t.Run("Select many with table", func(t *testing.T) {
		userData, err := db.Select("users")
		require.NoError(t, err)

		// unmarshal the data into a user slice
		var users []testUser
		err = surrealdb.Unmarshal(userData, &users)
		require.NoError(t, err)
		matching := assertContains(t, users, func(item testUser) bool {
			return item.Username == "johnnyjohn"
		})
		assert.GreaterOrEqual(t, len(matching), 1)
	})

	t.Run("Select single record", func(t *testing.T) {
		userData, err := db.Select(createdUsersUnmarshalled[0].ID)
		require.NoError(t, err)

		// unmarshal the data into a user struct
		var user testUser
		err = surrealdb.Unmarshal(userData, &user)
		require.NoError(t, err)

		assert.Equal(t, "johnnyjohn", user.Username)
		assert.Equal(t, "123", user.Password)
	})
}

func TestUpdate(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	require.NoError(t, err)

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	require.NoError(t, err)

	// unmarshal the data into a user struct
	var createdUser []testUser
	err = surrealdb.Unmarshal(userData, &createdUser)
	require.NoError(t, err)
	assert.Len(t, createdUser, 1)

	createdUser[0].Password = "456"

	// Update the user
	userData, err = db.Update("users", &createdUser[0])
	require.NoError(t, err)

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)
	require.NoError(t, err)

	// TODO: check if this updates only the user with the same ID or all users
	assert.Equal(t, "456", updatedUser[0].Password)
}

func TestUnmarshalRaw(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	require.NoError(t, err)

	_, err = db.Delete("users")
	require.NoError(t, err)

	username := "johnny"
	password := "123"

	// create test user with raw SurrealQL and unmarshal
	userData, err := db.Query("create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
		"user": username,
		"pass": password,
	})
	require.NoError(t, err)

	var userSlice []testUser
	ok, err := surrealdb.UnmarshalRaw(userData, &userSlice)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Len(t, userSlice, 1)
	assert.Equal(t, username, userSlice[0].Username)
	assert.Equal(t, password, userSlice[0].Password)

	// send query with empty result and unmarshal
	userData, err = db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	require.NoError(t, err)

	ok, err = surrealdb.UnmarshalRaw(userData, &userSlice)
	require.NoError(t, err)
	assert.False(t, ok, "select should return an empty result")
}

func TestModify(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	require.NoError(t, err)

	_, err = db.Delete("users:999") // Cleanup for reproducibility
	require.NoError(t, err)

	_, err = db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	require.NoError(t, err)

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: int(44)},
	}

	// Update the user
	_, err = db.Modify("users:999", patches)
	require.NoError(t, err)

	user2, err := db.Select("users:999")
	require.NoError(t, err)

	data := (user2).(map[string]interface{})["age"].(float64)

	assert.Equal(t, patches[1].Value, int(data))
}

func TestNonRowSelect(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", "test")
	require.NoError(t, err)

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
	require.NoError(t, err)

	t.Run("raw create query", func(t *testing.T) {
		QueryStr := "Create users set Username = $user, Password = $pass"
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](db.Query(QueryStr, map[string]interface{}{
			"user": user[0].Username,
			"pass": user[0].Password,
		}))

		require.NoError(t, err)
		assert.Equal(t, "electwix", dataArr[0].Username)
		user = dataArr
	})

	t.Run("raw select query", func(t *testing.T) {
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](db.Query("Select * from $record", map[string]interface{}{
			"record": user[0].ID,
		}))

		assert.Equal(t, "electwix", dataArr[0].Username)
		require.NoError(t, err)
	})

	t.Run("select query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](db.Select(user[0].ID))

		assert.Equal(t, "electwix", data.Username)
		require.NoError(t, err)
	})

	t.Run("select array query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[[]testUser](db.Select("users"))

		assert.Equal(t, "electwix", data[0].Username)
		require.NoError(t, err)
	})

	t.Run("delete record query", func(t *testing.T) {
		nulldata, err := surrealdb.SmartUnmarshal[*testUser](db.Delete(user[0].ID))

		require.NoError(t, err)
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
	require.NoError(t, err)

	t.Run("create with SmartMarshal query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Create, user[0]))
		require.NoError(t, err)
		assert.Equal(t, user[0], data)
	})

	t.Run("select with SmartMarshal query", func(t *testing.T) {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Select, user[0]))
		require.NoError(t, err)
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
		require.NoError(t, err)
		assert.Equal(t, &user[0], data)
	})

	t.Run("update with SmartMarshal query", func(t *testing.T) {
		user[0].Password = "test123"
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(db.Update, user[0]))
		require.NoError(t, err)
		assert.Equal(t, user[0].Password, data.Password)
	})

	t.Run("delete with SmartMarshal query", func(t *testing.T) {
		nulldata, err := surrealdb.SmartMarshal(db.Delete, &user[0])
		require.NoError(t, err)
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
