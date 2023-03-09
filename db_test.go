package surrealdb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"
)

var testDB = "test"

func init() {
	testDB = fmt.Sprint("test-", time.Now().Unix())
	fmt.Println("TestDB(act like an ID for every test): ", testDB)
	// Later the test itself can open an auto-generated `GET` request with the default browser which is point to the last testDB
	// Like https://surrealdb-admin.vercel.app/app?server=http%3A%2F%2Flocalhost%3A8000&user=root&password=root&ns=test&db=test-XXXXXXXXXXXX
	// That's will increase the cycles of development. ^_^
}

// a simple user struct for testing
type testUser struct {
	Username string
	Password string
	ID       string
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
	if err != nil {
		t.Fatal(err)
	}

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	if err != nil {
		t.Fatal(err)
	}

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the users...
	_, err = db.Delete("users")

	if err != nil {
		t.Fatal(err)
	}

	// Output:
}

func TestCreate(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	if err != nil {
		t.Fatal(err)
	}

	userMap, err := db.Create("users", map[string]interface{}{
		"username": "john",
		"password": "123",
	})

	if err != nil || userMap == nil {
		t.Fatal(err)
	}

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	if err != nil || userMap == nil {
		t.Fatal(err)
	}

	var user testUser
	err = surrealdb.Unmarshal(userData, &user)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(user.Username)

	// Output: johnny
}

func TestSelect(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Create("users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	if err != nil {
		t.Fatal(err)
	}

	userData, err := db.Select("users")
	if err != nil {
		t.Fatal(err)
	}

	// unmarshal the data into a user slice
	var users []testUser
	err = surrealdb.Unmarshal(userData, &users)
	if err != nil {
		t.Fatal(err)
	}

	for _, user := range users {
		if user.Username == "johnnyjohn" {
			fmt.Println(user.Username)
			break
		}
	}
	// Output: johnnyjohn
}

func TestUpdate(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	if err != nil {
		t.Fatal(err)
	}

	userData, err := db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	if err != nil {
		t.Fatal(err)
	}

	// unmarshal the data into a user struct
	var user testUser
	err = surrealdb.Unmarshal(userData, &testUser{})
	if err != nil {
		t.Fatal(err)
	}

	user.Password = "456"

	// Update the user
	userData, err = db.Update("users", &user)

	if err != nil {
		t.Fatal(err)
	}

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)

	if err != nil {
		t.Fatal(err)
	}

	// TODO: check if this updates only the user with the same ID or all users
	fmt.Println(updatedUser[0].Password)

	// Output: 456
}

func TestUnmarshalRaw(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Delete("users")
	if err != nil {
		t.Fatal(err)
	}

	username := "johnny"
	password := "123"

	// create test user with raw SurrealQL and unmarshal
	userData, err := db.Query("create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
		"user": username,
		"pass": password,
	})
	if err != nil {
		t.Fatal(err)
	}

	var user testUser
	ok, err := surrealdb.UnmarshalRaw(userData, &user)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || user.Username != username || user.Password != password {
		panic("response does not match the request")
	}

	// send query with empty result and unmarshal
	userData, err = db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	if err != nil {
		t.Fatal(err)
	}

	ok, err = surrealdb.UnmarshalRaw(userData, &user)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		panic("select should return an empty result")
	}

	// Output:
}

func TestModify(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	if err != nil {
		t.Fatal(err)
	}

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: 44},
	}

	// Update the user
	_, err = db.Modify("users:999", patches)
	if err != nil {
		t.Fatal(err)
	}

	user2, err := db.Select("users:999")
	if err != nil {
		t.Fatal(err)
	}

	// // TODO: this needs to simplified for the end user somehow
	fmt.Println((user2).(map[string]interface{})["age"])
	// Output: 44
}

func TestAutoMigration(t *testing.T) {
	db := openConnection(t)
	defer db.Close()

	_ = signin(t, db)

	_, err := db.Use("test", testDB)
	if err != nil {
		t.Fatal(err)
	}

	errs := db.AutoMigrate(testUser{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			t.Fatalf(err.Error())
		}
	}

	errs = db.AutoMigrate(testTypefulUser{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			t.Fatalf(err.Error())
		}
	}

	errs = db.AutoMigrate(Book{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			t.Fatalf(err.Error())
		}
	}

	errs = db.AutoMigrate(Text{}, true)

	if len(errs) != 0 {
		for _, err := range errs {
			t.Fatalf(err.Error())
		}
	}
}
