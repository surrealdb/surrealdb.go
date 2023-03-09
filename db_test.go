package surrealdb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go"
)

var testDB = "test"

func init() {
	testDB = fmt.Sprint("test-", time.Now().Unix())
	fmt.Println("TestDB(act like an ID for every test): ", testDB)
	// TODO: Later the test itself can open an auto-generated `GET` request with the default browser which is point to the last testDB
	// Like https://surrealdb-admin.vercel.app/app?server=http%3A%2F%2Flocalhost%3A8000&user=root&password=root&ns=test&db=test-XXXXXXXXXXXX
	// That's will increase the cycles of development. ^_^
}

// a simple user struct for testing
type testUser struct {
	Username string
	Password string
	ID       string
}

func (t testUser) String() string {
	// TODO: I found out we can use go generate stringer to generate these, but it was a bit confusing and too much
	// overhead atm, so doing this as a shortcut
	return fmt.Sprintf("testUser{Username: %+v, Password: %+v, ID: %+v}", t.Username, t.Password, t.ID)
}

type testTypefulUser struct {
	Username  string      `type:"string" unique:"Username"`
	Email     string      `type:"string" unique:"Email,Username" assert:"$value != NONE AND is::email($value)"`
	Password  string      `type:"string"`
	Tag       string      `type:"string" index:"Tag"`
	ID        int64       `type:"int"`
	Cost      float64     `type:"float" value:"$value OR 0.0" assert:"$value > 0"`
	Status    bool        `type:"bool" index:"Cost,Status"`
	MetaData  struct{}    `type:"object"`
	CreatedAt time.Time   `type:"datetime"`
	Checkouts []time.Time `type:"array"`

	Name Text `type:"object"`

	UsernameOptional  *string      `type:"string"` // all pointers are optional unless assert is mentioned
	PasswordOptional  *string      `type:"string"`
	IDOptional        *int64       `type:"int"`
	CostOptional      *float64     `type:"float"`
	StatusOptional    *bool        `type:"bool"`
	MetaDataOptional  *struct{}    `type:"object"`
	CreatedAtOptional *time.Time   `type:"datetime"`
	CheckoutsOptional *[]time.Time `type:"array"`
}

type Text struct {
	Ar string `type:"string" schema:"schemafull"` // by default schemaless
	En string `type:"string"`
}
type Book struct {
	Title Text `type:"object" unique:"Title"`
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

	_, err := db.Use("test", testDB)
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

	_, err := db.Use("test", testDB)
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

	_, err := db.Use("test", testDB)
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

	_, err := db.Use("test", testDB)
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

	_, err := db.Use("test", testDB)
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
	t.Skip("There is a permission issue with this test that may need to be solved in a different change")
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	assert.NoError(t, err)

	_, err = db.Delete("users:999") // Cleanup for reproducibility
	assert.NoError(t, err)

	_, err = db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	assert.NoError(t, err) // TODO: permission error, "Unable to access record:users:999"

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: 44},
	}

	// Update the user
	_, err = db.Modify("users:999", patches)
	assert.NoError(t, err)

	user2, err := db.Select("users:999")
	assert.NoError(t, err)

	// // TODO: this needs to simplified for the end user somehow
	assert.Equal(t, "44", (user2).(map[string]interface{})["age"])
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

func TestAutoMigration(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	assert.NoError(t, err)

	errs := db.AutoMigrate(testUser{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			assert.NoError(t, err)
		}
	}

	errs = db.AutoMigrate(testTypefulUser{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			assert.NoError(t, err)
		}
	}

	errs = db.AutoMigrate(Book{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			assert.NoError(t, err)
		}
	}

	errs = db.AutoMigrate(Text{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			assert.NoError(t, err)
		}
	}
}
