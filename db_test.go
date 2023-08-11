package surrealdb_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	gorilla "github.com/surrealdb/surrealdb.go/pkg/gorilla"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
	"github.com/surrealdb/surrealdb.go/pkg/websocket"
)

// TestDBSuite is a test s for the DB struct
type SurrealDBTestSuite struct {
	suite.Suite
	db                *surrealdb.DB
	name              string
	wsImplementations map[string]websocket.WebSocket
}

// a simple user struct for testing
type testUser struct {
	marshal.Basemodel `table:"test"`
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	ID                string `json:"id,omitempty"`
}

func TestSurrealDBSuite(t *testing.T) {
	SurrealDBSuite := new(SurrealDBTestSuite)
	SurrealDBSuite.wsImplementations = make(map[string]websocket.WebSocket)

	// Without options
	logData, err := createLogData(t)
	require.NoError(t, err)
	SurrealDBSuite.wsImplementations["gorilla"] = gorilla.Create().Logger(logData)

	// With options
	logData, err = createLogData(t)
	require.NoError(t, err)
	SurrealDBSuite.wsImplementations["gorilla_opt"] = gorilla.Create().SetTimeOut(time.Minute).SetCompression(true).Logger(logData)

	RunWsMap(t, SurrealDBSuite)
}

func createLogData(t *testing.T) (*logger.LogData, error) {
	t.Helper()
	buff := bytes.NewBuffer([]byte{})
	return logger.New().FromBuffer(buff).Make()
}

func RunWsMap(t *testing.T, s *SurrealDBTestSuite) {
	for wsName := range s.wsImplementations {
		// Run the test suite
		t.Run(wsName, func(t *testing.T) {
			s.name = wsName
			suite.Run(t, s)
		})
	}
}

// SetupTest is called after each test
func (s *SurrealDBTestSuite) TearDownTest() {
	_, err := s.db.Delete("users")
	s.Require().NoError(err)
}

// TearDownSuite is called after the s has finished running
func (s *SurrealDBTestSuite) TearDownSuite() {
	s.db.Close()
}

func (t testUser) String() (str string, err error) {
	byteData, err := json.Marshal(t)
	if err != nil {
		return
	}
	str = string(byteData)
	return
}

// openConnection opens a new connection to the database
func (s *SurrealDBTestSuite) openConnection() *surrealdb.DB {
	url := os.Getenv("SURREALDB_URL")
	if url == "" {
		url = "ws://localhost:8000/rpc"
	}
	impl := s.wsImplementations[s.name]
	require.NotNil(s.T(), impl)
	ws, err := impl.Connect(url)
	s.Require().NoError(err)
	db, err := surrealdb.New(url, ws)
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
	err = marshal.Unmarshal(userData, &user)
	s.Require().NoError(err)

	// Delete the users...
	_, err = s.db.Delete("users")
	s.Require().NoError(err)
}

func (s *SurrealDBTestSuite) TestInsert() {
	s.Run("raw map works", func() {
		userData, err := s.db.Insert("user", map[string]interface{}{
			"username": "johnny",
			"password": "123",
		})
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var user []testUser
		err = marshal.Unmarshal(userData, &user)
		s.Require().NoError(err)

		s.Equal("johnny", user[0].Username)
		s.Equal("123", user[0].Password)
	})

	s.Run("Single insert works", func() {
		userData, err := s.db.Insert("user", testUser{
			Username: "johnny",
			Password: "123",
		})
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var user []testUser
		err = marshal.Unmarshal(userData, &user)
		s.Require().NoError(err)

		s.Equal("johnny", user[0].Username)
		s.Equal("123", user[0].Password)
	})

	s.Run("Multiple insert works", func() {
		userInsert := make([]testUser, 0)
		userInsert = append(userInsert, testUser{
			Username: "johnny1",
			Password: "123",
		}, testUser{
			Username: "johnny2",
			Password: "123",
		})
		userData, err := s.db.Insert("user", userInsert)
		s.Require().NoError(err)

		// unmarshal the data into a user struct
		var users []testUser
		err = marshal.Unmarshal(userData, &users)
		s.Require().NoError(err)
		s.Len(users, 2)

		assertContains(s, users, func(user testUser) bool {
			return s.Contains(users, user)
		})
	})
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
		err = marshal.Unmarshal(userData, &userSlice)
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
		err = marshal.Unmarshal(userData, &userSlice)
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
		err = marshal.Unmarshal(userData, &users)
		s.Require().NoError(err)

		assertContains(s, users, func(user testUser) bool {
			return s.Contains(users, user)
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
	s.Require().NoError(marshal.Unmarshal(createdUsers, &createdUsersUnmarshalled))
	s.NotEmpty(createdUsersUnmarshalled)
	s.NotEmpty(createdUsersUnmarshalled[0].ID, "The ID should have been set by the database")

	s.Run("Select many with table", func() {
		userData, err := s.db.Select("users")
		s.Require().NoError(err)

		// unmarshal the data into a user slice
		var users []testUser
		err = marshal.Unmarshal(userData, &users)
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
		err = marshal.Unmarshal(userData, &user)
		s.Require().NoError(err)

		s.Equal("johnnyjohn", user.Username)
		s.Equal("123", user.Password)
	})
}

func (s *SurrealDBTestSuite) TestUpdate() {
	newPassword := "456"
	users := []testUser{
		{Username: "Johnny", Password: "123"},
		{Username: "Mat", Password: "555"},
	}

	// create users
	var createdUsers []testUser
	for _, v := range users {
		createdUser, err := s.db.Create("users", v)
		s.Require().NoError(err)
		var tempUserArr []testUser
		err = marshal.Unmarshal(createdUser, &tempUserArr)
		s.Require().NoError(err)
		createdUsers = append(createdUsers, tempUserArr...)
	}

	createdUsers[0].Password = newPassword

	// Update the user
	UpdatedUserRaw, err := s.db.Update(createdUsers[0].ID, createdUsers[0])
	s.Require().NoError(err)

	// unmarshal the data into a user struct
	var updatedUser testUser
	err = marshal.Unmarshal(UpdatedUserRaw, &updatedUser)
	s.Require().NoError(err)

	// Check if password changes
	s.Equal(newPassword, updatedUser.Password)

	// select controlUser
	controlUserRaw, err := s.db.Select(createdUsers[1].ID)
	s.Require().NoError(err)

	// unmarshal the data into a user struct
	var controlUser testUser
	err = marshal.Unmarshal(controlUserRaw, &controlUser)
	s.Require().NoError(err)

	// check control user is changed or not
	s.Equal(createdUsers[1], controlUser)
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

	var userSlice []marshal.RawQuery[testUser]
	err = marshal.UnmarshalRaw(userData, &userSlice)
	s.Require().NoError(err)
	s.Len(userSlice, 1)
	s.Equal(userSlice[0].Status, marshal.StatusOK)
	s.Equal(username, userSlice[0].Result[0].Username)
	s.Equal(password, userSlice[0].Result[0].Password)

	// send query with empty result and unmarshal
	userData, err = s.db.Query("select * from users where id = $id", map[string]interface{}{
		"id": "users:jim",
	})
	s.Require().NoError(err)

	err = marshal.UnmarshalRaw(userData, &userSlice)
	s.NoError(err)
	s.Equal(userSlice[0].Status, marshal.StatusOK)
	s.Empty(userSlice[0].Result)
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
	s.Equal(err, constants.ErrNoRow)

	_, err = marshal.SmartUnmarshal[testUser](s.db.Select("users:notexists"))
	s.Equal(err, constants.ErrNoRow)

	_, err = marshal.SmartUnmarshal[testUser](marshal.SmartMarshal(s.db.Select, user))
	s.Equal(err, constants.ErrNoRow)
}

func (s *SurrealDBTestSuite) TestSmartUnMarshalQuery() {
	user := []testUser{{
		Username: "electwix",
		Password: "1234",
	}}

	s.Run("raw create query", func() {
		QueryStr := "Create users set Username = $user, Password = $pass"
		dataArr, err := marshal.SmartUnmarshal[testUser](s.db.Query(QueryStr, map[string]interface{}{
			"user": user[0].Username,
			"pass": user[0].Password,
		}))

		s.Require().NoError(err)
		s.Equal("electwix", dataArr[0].Username)
		user = dataArr
	})

	s.Run("raw select query", func() {
		dataArr, err := marshal.SmartUnmarshal[testUser](s.db.Query("Select * from $record", map[string]interface{}{
			"record": user[0].ID,
		}))

		s.Require().NoError(err)
		s.Equal("electwix", dataArr[0].Username)
	})

	s.Run("select query", func() {
		data, err := marshal.SmartUnmarshal[testUser](s.db.Select(user[0].ID))

		s.Require().NoError(err)
		s.Equal("electwix", data[0].Username)
	})

	s.Run("select array query", func() {
		data, err := marshal.SmartUnmarshal[testUser](s.db.Select("users"))

		s.Require().NoError(err)
		s.Equal("electwix", data[0].Username)
	})

	s.Run("delete record query", func() {
		data, err := marshal.SmartUnmarshal[testUser](s.db.Delete(user[0].ID))

		s.Require().NoError(err)
		s.Len(data, 0)
	})
}

func (s *SurrealDBTestSuite) TestSmartMarshalQuery() {
	user := []testUser{{
		Username: "electwix",
		Password: "1234",
		ID:       "sometable:someid",
	}}

	s.Run("create with SmartMarshal query", func() {
		data, err := marshal.SmartUnmarshal[testUser](marshal.SmartMarshal(s.db.Create, user[0]))
		s.Require().NoError(err)
		s.Len(data, 1)
		s.Equal(user[0], data[0])
	})

	s.Run("select with SmartMarshal query", func() {
		data, err := marshal.SmartUnmarshal[testUser](marshal.SmartMarshal(s.db.Select, user[0]))
		s.Require().NoError(err)
		s.Len(data, 1)
		s.Equal(user[0], data[0])
	})

	s.Run("update with SmartMarshal query", func() {
		user[0].Password = "test123"
		data, err := marshal.SmartUnmarshal[testUser](marshal.SmartMarshal(s.db.Update, user[0]))
		s.Require().NoError(err)
		s.Len(data, 1)
		s.Equal(user[0].Password, data[0].Password)
	})

	s.Run("delete with SmartMarshal query", func() {
		data, err := marshal.SmartMarshal(s.db.Delete, user[0])
		s.Require().NoError(err)
		s.Nil(data)
	})

	s.Run("check if data deleted SmartMarshal query", func() {
		data, err := marshal.SmartUnmarshal[testUser](marshal.SmartMarshal(s.db.Select, user[0]))
		s.Require().Equal(err, constants.ErrNoRow)
		s.Len(data, 0)
	})
}

func (s *SurrealDBTestSuite) TestConcurrentOperations() {
	var wg sync.WaitGroup
	totalGoroutines := 100

	user := testUser{
		Username: "electwix",
		Password: "1234",
	}

	s.Run(fmt.Sprintf("Concurrent select non existent rows %d", totalGoroutines), func() {
		for i := 0; i < totalGoroutines; i++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				_, err := s.db.Select(fmt.Sprintf("users:%d", j))
				s.Require().Equal(err, constants.ErrNoRow)
			}(i)
		}
		wg.Wait()
	})

	s.Run(fmt.Sprintf("Concurrent create rows %d", totalGoroutines), func() {
		for i := 0; i < totalGoroutines; i++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				_, err := s.db.Create(fmt.Sprintf("users:%d", j), user)
				s.Require().NoError(err)
			}(i)
		}
		wg.Wait()
	})

	s.Run(fmt.Sprintf("Concurrent select exist rows %d", totalGoroutines), func() {
		for i := 0; i < totalGoroutines; i++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				_, err := s.db.Select(fmt.Sprintf("users:%d", j))
				s.Require().NoError(err)
			}(i)
		}
		wg.Wait()
	})
}

// assertContains performs an assertion on a list, asserting that at least one element matches a provided condition.
// All the matching elements are returned from this function, which can be used as a filter.
func assertContains[K any](s *SurrealDBTestSuite, input []K, matcher func(K) bool) []K {
	matching := make([]K, 0)
	for _, v := range input {
		if matcher(v) {
			matching = append(matching, v)
		}
	}
	s.NotEmptyf(matching, "Input %+v did not contain matching element", fmt.Sprintf("%+v", input))
	return matching
}
