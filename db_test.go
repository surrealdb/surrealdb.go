package surrealdb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go"
)

// TestDBSuite is a test s for the DB struct
type SurrealDBTestSuite struct {
	suite.Suite
	db      *surrealdb.DB
	options []surrealdb.Option
}

// a simple user struct for testing
type testUser struct {
	surrealdb.Basemodel `table:"test"`
	Username            string `json:"username,omitempty"`
	Password            string `json:"password,omitempty"`
	ID                  string `json:"id,omitempty"`
}

// getOptions returns a list of options to be used when creating a new websocket connection
func getOptions() (options []surrealdb.Option) {
	options = append(options, surrealdb.UseWriteCompression(true), surrealdb.WithTimeout(20*time.Second))
	return
}

func TestSurrealDBSuite(t *testing.T) {
	// Without options
	SurrealDBSuite := new(SurrealDBTestSuite)
	suite.Run(t, SurrealDBSuite)

	// With options
	SurrealDBSuite.options = getOptions()
	suite.Run(t, SurrealDBSuite)
}

// SetupTest is called before each test
func (s *SurrealDBTestSuite) TearDownTest() {
	_, err := s.db.Delete("users")
	s.Require().NoError(err)
}

// TearDownSuite is called after the s has finished running
func (s *SurrealDBTestSuite) TearDownSuite() {
	s.db.Close()
}

func (t testUser) String() string {
	// TODO I found out we can use go generate stringer to generate these, but it was a bit confusing and too much
	// overhead atm, so doing this as a shortcut
	return fmt.Sprintf("testUser{Username: %+v, Password: %+v, ID: %+v}", t.Username, t.Password, t.ID)
}

// openConnection opens a new connection to the database
func (s *SurrealDBTestSuite) openConnection() *surrealdb.DB {
	url := os.Getenv("SURREALDB_URL")
	if url == "" {
		url = "ws://localhost:8000/rpc"
	}
	db, err := surrealdb.New(url, s.options...)
	s.Require().NoError(err)

	return db
}

// SetupSuite is called before the s starts running
func (s *SurrealDBTestSuite) SetupSuite() {
	db := s.openConnection()
	s.Require().NotNil(db)
	s.db = db
	_ = signin(s)
	_, err := db.Use("test", "test")
	s.Require().NoError(err)
}

// Sign with the root user
// Can be used with any user
func signin(s *SurrealDBTestSuite) interface{} {
	signin, err := s.db.Signin(map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	s.Require().NoError(err)
	return signin
}

func (s *SurrealDBTestSuite) TestDelete() {
	userData, err := s.db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	s.Require().NoError(err)

	// unmarshal the data into a user struct
	var user []testUser
	err = surrealdb.Unmarshal(userData, &user)
	s.Require().NoError(err)

	// Delete the users...
	_, err = s.db.Delete("users")
	s.Require().NoError(err)
}

func (s *SurrealDBTestSuite) TestDeleteWithOptions() {
	userData, err := s.db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	s.NoError(err)

	// unmarshal the data into a user struct
	var user []testUser
	err = surrealdb.Unmarshal(userData, &user)
	s.NoError(err)

	// Delete the users...
	_, err = s.db.Delete("users")
	s.NoError(err)
}

func (s *SurrealDBTestSuite) TestCreate() {
	s.Run("raw map works", func() {
		userData, err := s.db.Create("users", map[string]interface{}{
			"username": "johnny",
			"password": "123",
		})
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var userSlice []testUser
		err = surrealdb.Unmarshal(userData, &userSlice)
		s.Require().NoError(err)
		s.Len(userSlice, 1)

		s.Equal("johnny", userSlice[0].Username)
		s.Equal("123", userSlice[0].Password)
	})

	s.Run("Single create works", func() {
		userData, err := s.db.Create("users", testUser{
			Username: "johnny",
			Password: "123",
		})
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var userSlice []testUser
		err = surrealdb.Unmarshal(userData, &userSlice)
		s.Require().NoError(err)
		s.Len(userSlice, 1)

		s.Equal("johnny", userSlice[0].Username)
		s.Equal("123", userSlice[0].Password)
	})

	s.Run("Multiple creates works", func() {
		s.T().Skip("Creating multiple records is not supported yet")
		data := make([]testUser, 0)
		data = append(data,
			testUser{
				Username: "johnny",
				Password: "123"},
			testUser{
				Username: "joe",
				Password: "123",
			})
		userData, err := s.db.Create("users", data)
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var users []testUser
		err = surrealdb.Unmarshal(userData, &users)
		s.Require().NoError(err)
		assertContains(s, users, func(user testUser) bool {
			return user == data[0] ||
				user == data[1]
		})
	})
}

func (s *SurrealDBTestSuite) TestSelect() {
	createdUsers, err := s.db.Create("users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	s.Require().NoError(err)
	s.NotEmpty(createdUsers)
	var createdUsersUnmarshalled []testUser
	s.Require().NoError(surrealdb.Unmarshal(createdUsers, &createdUsersUnmarshalled))
	s.NotEmpty(createdUsersUnmarshalled)
	s.NotEmpty(createdUsersUnmarshalled[0].ID, "The ID should have been set by the database")

	s.Run("Select many with table", func() {
		userData, err := s.db.Select("users")
		s.Require().NoError(err)

		// unmarshal the data into a user slice
		var users []testUser
		err = surrealdb.Unmarshal(userData, &users)
		s.NoError(err)
		matching := assertContains(s, users, func(item testUser) bool {
			return item.Username == "johnnyjohn"
		})
		s.GreaterOrEqual(len(matching), 1)
	})

	s.Run("Select single record", func() {
		userData, err := s.db.Select(createdUsersUnmarshalled[0].ID)
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var user testUser
		err = surrealdb.Unmarshal(userData, &user)
		s.Require().NoError(err)

		s.Equal("johnnyjohn", user.Username)
		s.Equal("123", user.Password)
	})
}

func (s *SurrealDBTestSuite) TestUpdate() {
	userData, err := s.db.Create("users", testUser{
		Username: "johnny",
		Password: "123",
	})
	s.Require().NoError(err)

	// unmarshal the data into a user struct
	var createdUser []testUser
	err = surrealdb.Unmarshal(userData, &createdUser)
	s.Require().NoError(err)
	s.Len(createdUser, 1)

	createdUser[0].Password = "456"

	// Update the user
	userData, err = s.db.Update("users", &createdUser[0])
	s.Require().NoError(err)

	// unmarshal the data into a user struct
	var updatedUser []testUser
	err = surrealdb.Unmarshal(userData, &updatedUser)
	s.Require().NoError(err)

	// TODO: check if this updates only the user with the same ID or all users
	s.Equal("456", updatedUser[0].Password)
}

func (s *SurrealDBTestSuite) TestUnmarshalRaw() {
	username := "johnny"
	password := "123"

	// create test user with raw SurrealQL and unmarshal
	userData, err := s.db.Query("create users:johnny set Username = $user, Password = $pass", map[string]interface{}{
		"user": username,
		"pass": password,
	})
	s.Require().NoError(err)

	var userSlice []testUser
	ok, err := surrealdb.UnmarshalRaw(userData, &userSlice)
	s.Require().NoError(err)
	s.True(ok)
	s.Len(userSlice, 1)
	s.Equal(username, userSlice[0].Username)
	s.Equal(password, userSlice[0].Password)

	// send query with empty result and unmarshal
	userData, err = s.db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	s.Require().NoError(err)

	ok, err = surrealdb.UnmarshalRaw(userData, &userSlice)
	s.NoError(err)
	s.False(ok, "select should return an empty result")
}

func (s *SurrealDBTestSuite) TestModify() {
	_, err := s.db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	s.NoError(err)

	patches := []surrealdb.Patch{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: int(44)},
	}

	// Update the user
	_, err = s.db.Modify("users:999", patches)
	s.Require().NoError(err)

	user2, err := s.db.Select("users:999")
	s.Require().NoError(err)

	data := (user2).(map[string]interface{})["age"].(float64)

	s.Equal(patches[1].Value, int(data))
}

func (s *SurrealDBTestSuite) TestNonRowSelect() {
	user := testUser{
		Username: "ElecTwix",
		Password: "1234",
		ID:       "users:notexists",
	}

	_, err := s.db.Select("users:notexists")
	s.Equal(err, surrealdb.ErrNoRow)

	_, err = surrealdb.SmartUnmarshal[testUser](s.db.Select("users:notexists"))
	s.Equal(err, surrealdb.ErrNoRow)

	_, err = surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Select, user))
	s.Equal(err, surrealdb.ErrNoRow)
}

func (s *SurrealDBTestSuite) TestSmartUnMarshalQuery() {
	user := []testUser{{
		Username: "electwix",
		Password: "1234",
	}}

	s.Run("raw create query", func() {
		QueryStr := "Create users set Username = $user, Password = $pass"
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](s.db.Query(QueryStr, map[string]interface{}{
			"user": user[0].Username,
			"pass": user[0].Password,
		}))

		s.Require().NoError(err)
		s.Equal("electwix", dataArr[0].Username)
		user = dataArr
	})

	s.Run("raw select query", func() {
		dataArr, err := surrealdb.SmartUnmarshal[[]testUser](s.db.Query("Select * from $record", map[string]interface{}{
			"record": user[0].ID,
		}))

		s.Require().NoError(err)
		s.Equal("electwix", dataArr[0].Username)
	})

	s.Run("select query", func() {
		data, err := surrealdb.SmartUnmarshal[testUser](s.db.Select(user[0].ID))

		s.Require().NoError(err)
		s.Equal("electwix", data.Username)
	})

	s.Run("select array query", func() {
		data, err := surrealdb.SmartUnmarshal[[]testUser](s.db.Select("users"))

		s.Require().NoError(err)
		s.Equal("electwix", data[0].Username)
	})

	s.Run("delete record query", func() {
		nulldata, err := surrealdb.SmartUnmarshal[*testUser](s.db.Delete(user[0].ID))

		s.Require().NoError(err)
		s.Nil(nulldata)
	})
}

func (s *SurrealDBTestSuite) TestSmartMarshalQuery() {
	user := []testUser{{
		Username: "electwix",
		Password: "1234",
		ID:       "sometable:someid",
	}}

	s.Run("create with SmartMarshal query", func() {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Create, user[0]))
		s.Require().NoError(err)
		s.Equal(user[0], data)
	})

	s.Run("select with SmartMarshal query", func() {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Select, user[0]))
		s.Require().NoError(err)
		s.Equal(user[0], data)
	})

	s.Run("select with nil pointer SmartMarshal query", func() {
		var nilptr *testUser
		data, err := surrealdb.SmartUnmarshal[*testUser](surrealdb.SmartMarshal(s.db.Select, &nilptr))
		s.Require().Equal(err, surrealdb.ErrNotStruct)
		s.Equal(nilptr, data)
	})

	s.Run("select with pointer SmartMarshal query", func() {
		data, err := surrealdb.SmartUnmarshal[*testUser](surrealdb.SmartMarshal(s.db.Select, &user[0]))
		s.NoError(err)
		s.Equal(&user[0], data)
	})

	s.Run("update with SmartMarshal query", func() {
		user[0].Password = "test123"
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Update, user[0]))
		s.Require().NoError(err)
		s.Equal(user[0].Password, data.Password)
	})

	s.Run("delete with SmartMarshal query", func() {
		nulldata, err := surrealdb.SmartMarshal(s.db.Delete, &user[0])
		s.Require().NoError(err)
		s.Nil(nulldata)
	})

	s.Run("check if data deleted SmartMarshal query", func() {
		data, err := surrealdb.SmartUnmarshal[testUser](surrealdb.SmartMarshal(s.db.Select, user[0]))
		s.Require().Equal(err, surrealdb.ErrNoRow)
		s.Equal(data, testUser{})
	})
}

// assertContains performs an assertion on a list, asserting that at least one element matches a provided condition.
// All the matching elements are returned from this function, which can be used as a filter.
func assertContains[K fmt.Stringer](s *SurrealDBTestSuite, input []K, matcher func(K) bool) []K {
	matching := make([]K, 0)
	for i := range input {
		if matcher(input[i]) {
			matching = append(matching, input[i])
		}
	}
	s.NotEmptyf(matching, "Input %+v did not contain matching element", fmt.Sprintf("%+v", input))
	return matching
}
