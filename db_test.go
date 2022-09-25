package surrealdb_test

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/test-go/testify/suite"
)

// a simple user struct for testing
type testUser struct {
	Username string
	Password string
	ID       string
}

// an example test for creating a new entry in surrealdb
func ExampleNew() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	// Output:
}

func ExampleDB_Delete() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     "root",
		Password: "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil {
		panic(err)
	}

	userData, err := db.Create(ctx, "users", testUser{
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
	_, err = db.Delete(ctx, "users")

	if err != nil {
		panic(err)
	}

	// Output:
}

func ExampleDB_Create() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	signin, err := db.Signin(ctx, surrealdb.UserInfo{
		User:     "root",
		Password: "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil || signin == nil {
		panic(err)
	}

	userMap, err := db.Create(ctx, "users", map[string]any{
		"username": "john",
		"password": "123",
	})

	if err != nil || userMap == nil {
		panic(err)
	}

	userData, err := db.Create(ctx, "users", testUser{
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")

	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     "root",
		Password: "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil {
		panic(err)
	}

	_, err = db.Create(ctx, "users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})

	userData, err := db.Select(ctx, "users")

	// unmarshal the data into a user slice
	var users []testUser
	log.Print(userData)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     "root",
		Password: "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil {
		panic(err)
	}

	userData, err := db.Create(ctx, "users", testUser{
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
	userData, err = db.Update(ctx, "users", &user)

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     "root",
		Password: "root",
	})

	if err != nil {
		panic(err)
	}

	_, err = db.Use(ctx, "test", "test")

	if err != nil {
		panic(err)
	}

	_, err = db.Delete(ctx, "users")
	if err != nil {
		panic(err)
	}

	username := "johnny"
	password := "123"

	// create test user with raw SurrealQL and unmarshal

	userData, err := db.Query(ctx, "create users:johnny set Username = $user, Password = $pass", map[string]any{
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

	// send query with empty result and unmarshal

	userData, err = db.Query(ctx, "select * from users where id = $id", map[string]any{
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := surrealdb.New(ctx, "ws://localhost:8000/rpc")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     "root",
		Password: "root",
	})
	_, err = db.Use(ctx, "test", "test")

	_, err = db.Create(ctx, "users:999", map[string]any{
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
	_, err = db.Modify(ctx, "users:999", patches)
	if err != nil {
		panic(err)
	}

	user2, err := db.Select(ctx, "users:999")
	if err != nil {
		panic(err)
	}

	// // TODO: this needs to simplified for the end user somehow
	fmt.Println((user2).(map[string]any)["age"])
	//
	// Output: 44
}

type TestDatabaseTestSuite struct {
	suite.Suite
	ctx context.Context
	db  *surrealdb.DB
}

func TestDatabaseSuite(t *testing.T) {
	suite.Run(t, new(TestDatabaseTestSuite))
}

func (suite *TestDatabaseTestSuite) SetupTest() {
	ctx := context.Background()

	rpcUrl := surrealdb.GetEnvOrDefault("SURREALDB_RPC_URL", "ws://localhost:8000/rpc")
	user := surrealdb.GetEnvOrDefault("SURREALDB_USER", "root")
	pass := surrealdb.GetEnvOrDefault("SURREALDB_PASS", "root")

	db, err := surrealdb.New(ctx, rpcUrl)
	suite.Require().NoError(err)

	_, err = db.Signin(ctx, surrealdb.UserInfo{
		User:     user,
		Password: pass,
	})
	suite.Require().NoError(err)

	_, err = db.Use(ctx, "test", "test")
	suite.Require().NoError(err)

	suite.db = db
	suite.ctx = ctx
}

func (suite *TestDatabaseTestSuite) TearDownSuite() {
	suite.db.Close()
}

func (suite *TestDatabaseTestSuite) Test_FailingUserSignin() {
	// NOTE: this query fails for some reason but works when I run it manually...
	// DEFINE SCOPE test_account_scope
	//     SIGNIN ( SELECT * FROM user WHERE username = $user AND crypto::argon2::compare(password, $pass) )
	//     SIGNUP ( CREATE user SET username = $user, password = crypto::argon2::generate($pass) )
	// ;
	// result, err := suite.db.Query(suite.ctx, scopeQuery, map[string]any{})
	// suite.Require().NoError(err)
	// suite.Require().NotNil(result)

	authResult, err := suite.db.SigninUser(suite.ctx, surrealdb.UserInfo{
		User:      "test_username",
		Password:  "test_password",
		Namespace: "test_account_scope",
		Database:  "test",
		Scope:     "test",
	})

	suite.Require().Error(err)
	suite.Require().NotNil(authResult)
	suite.Require().False(authResult.Success)

	authResult, err = suite.db.SignupUser(suite.ctx, surrealdb.UserInfo{
		User:      "test_username",
		Password:  "test_password",
		Namespace: "test",
		Database:  "test",
		Scope:     "test_account_scope",
	})
	suite.Require().NoError(err)
	suite.Require().NotNil(authResult)
	suite.Require().True(authResult.Success)
	suite.Require().NotZero(authResult.Token)
	suite.Require().NotZero(authResult.TokenData)
	suite.Require().Equal(authResult.TokenData.Scope, "test_account_scope")
}
