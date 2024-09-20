package surrealdb_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"io"
	rawslog "log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/marshal"
)

// Default consts and vars for testing
const (
	defaultURL = "ws://localhost:8000/rpc"
)

var currentURL = os.Getenv("SURREALDB_URL")

var newConParams = connection.NewConnectionParams{
	Marshaler:   models.CborMarshaler{},
	Unmarshaler: models.CborUnmarshaler{},
}

//

// TestDBSuite is a test s for the DB struct
type SurrealDBTestSuite struct {
	suite.Suite
	db                  *surrealdb.DB
	name                string
	connImplementations map[string]connection.Connection
	logBuffer           *bytes.Buffer
}

// a simple user struct for testing
type testUser struct {
	marshal.Basemodel `table:"test"`
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	ID                string `json:"id,omitempty"`
}

// a simple user struct for testing
type testUserWithFriend[I any] struct {
	marshal.Basemodel `table:"user_with_friend"`
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	ID                string `json:"id,omitempty"`
	Friends           []I    `json:"friends,omitempty"`
}

func TestSurrealDBSuite(t *testing.T) {
	SurrealDBSuite := new(SurrealDBTestSuite)
	SurrealDBSuite.connImplementations = make(map[string]connection.Connection)

	// Without options
	buff := bytes.NewBufferString("")
	logData := createLogger(t, buff)
	SurrealDBSuite.connImplementations["ws"] = connection.
		NewWebSocketConnection(newConParams).
		Logger(logData)
	SurrealDBSuite.logBuffer = buff

	// With options
	buffOpt := bytes.NewBufferString("")
	logDataOpt := createLogger(t, buff)
	SurrealDBSuite.connImplementations["ws_opt"] = connection.
		NewWebSocketConnection(newConParams).
		SetTimeOut(time.Minute).
		SetCompression(true).
		Logger(logDataOpt)
	SurrealDBSuite.logBuffer = buffOpt

	RunWsMap(t, SurrealDBSuite)
}

func createLogger(t *testing.T, writer io.Writer) logger.Logger {
	t.Helper()
	handler := rawslog.NewJSONHandler(writer, &rawslog.HandlerOptions{Level: rawslog.LevelDebug})
	return logger.New(handler)
}

func RunWsMap(t *testing.T, s *SurrealDBTestSuite) {
	for wsName := range s.connImplementations {
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

	if s.logBuffer.Len() > 0 {
		s.T().Logf("Log output:\n%s", s.logBuffer.String())
	}
}

// TearDownSuite is called after the s has finished running
func (s *SurrealDBTestSuite) TearDownSuite() {
	err := s.db.Close()
	s.Require().NoError(err)
}

func (t testUser) String() (str string, err error) {
	byteData, err := json.Marshal(t)
	if err != nil {
		return
	}
	str = string(byteData)
	return
}

func (s *SurrealDBTestSuite) createTestDB() *surrealdb.DB {
	url := os.Getenv("SURREALDB_URL")
	if url == "" {
		url = "ws://localhost:8000/rpc"
	}
	impl := s.connImplementations[s.name]
	db := s.openConnection(url, impl)
	return db
}

// openConnection opens a new connection to the database
func (s *SurrealDBTestSuite) openConnection(url string, impl connection.Connection) *surrealdb.DB {
	require.NotNil(s.T(), impl)
	db, err := surrealdb.New(url)
	s.Require().NoError(err)
	return db
}

// SetupSuite is called before the s starts running
func (s *SurrealDBTestSuite) SetupSuite() {
	db := s.createTestDB()
	s.Require().NotNil(db)
	s.db = db
	_ = signin(s)
	_, err := db.Use("test", "test")
	s.Require().NoError(err)
}

// Sign with the root user
// Can be used with any user
func signin(s *SurrealDBTestSuite) interface{} {
	authData := &models.Auth{
		Username: "root",
		Password: "root",
	}
	signin, err := s.db.Signin(authData)
	s.Require().NoError(err)
	return signin
}

func (s *SurrealDBTestSuite) TestLiveViaMethod() {
	live, err := s.db.Live("users", false)
	defer func() {
		_, err = s.db.Kill(live)
		s.Require().NoError(err)
	}()

	notifications, er := s.db.LiveNotifications(live)
	// create a user
	s.Require().NoError(er)
	_, e := s.db.Create("users", map[string]interface{}{
		"username": "johnny",
		"password": "123",
	})
	s.Require().NoError(e)
	notification := <-notifications
	s.Require().Equal(connection.CreateAction, notification.Action)
	s.Require().Equal(live, notification.ID)
}

func (s *SurrealDBTestSuite) TestLiveWithOptionsViaMethod() {
	// create a user
	userData, e := s.db.Create("users", map[string]interface{}{
		"username": "johnny",
		"password": "123",
	})
	s.Require().NoError(e)
	var user []testUser
	err := marshal.Unmarshal(userData, &user)
	s.Require().NoError(err)

	live, err := s.db.Live("users", true)
	defer func() {
		_, err = s.db.Kill(live)
		s.Require().NoError(err)
	}()

	notifications, er := s.db.LiveNotifications(live)
	s.Require().NoError(er)

	// update the user
	_, e = s.db.Update(user[0].ID, map[string]interface{}{
		"password": "456",
	})
	s.Require().NoError(e)

	notification := <-notifications
	s.Require().Equal(connection.UpdateAction, notification.Action)
	s.Require().Equal(live, notification.ID)
}

func (s *SurrealDBTestSuite) TestLiveViaQuery() {
	liveResponse, err := s.db.Query("LIVE SELECT * FROM users", map[string]interface{}{})
	assert.NoError(s.T(), err)
	responseArray, ok := liveResponse.([]interface{})
	assert.True(s.T(), ok)
	singleResponse := responseArray[0].(map[string]interface{})
	liveIDStruct, ok := singleResponse["result"]
	assert.True(s.T(), ok)
	liveID := liveIDStruct.(string)

	defer func() {
		_, err = s.db.Kill(liveID)
		s.Require().NoError(err)
	}()

	notifications, er := s.db.LiveNotifications(liveID)
	// create a user
	s.Require().NoError(er)
	_, e := s.db.Create("users", map[string]interface{}{
		"username": "johnny",
		"password": "123",
	})
	s.Require().NoError(e)
	notification := <-notifications
	s.Require().Equal(connection.CreateAction, notification.Action)
	s.Require().Equal(liveID, notification.ID)
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

func (s *SurrealDBTestSuite) TestFetch() {
	// Define initial user slice
	userSlice := []testUserWithFriend[string]{
		{
			ID:       "users:arthur",
			Username: "arthur",
			Password: "deer",
			Friends:  []string{"users:john"},
		},
		{
			ID:       "users:john",
			Username: "john",
			Password: "wolf",
			Friends:  []string{"users:arthur"},
		},
	}

	// Initialize data using users
	for _, v := range userSlice {
		data, err := s.db.Create(v.ID, v)
		s.NoError(err)
		s.NotNil(data)
	}

	// User rows are individually fetched
	s.Run("Run fetch for individual users", func() {
		s.T().Skip("TODO(gh-116) Fetch unimplemented")
		for _, v := range userSlice {
			res, err := s.db.Query("select * from $table fetch $fetchstr;", map[string]interface{}{
				"record":   v.ID,
				"fetchstr": "friends.*",
			})
			s.NoError(err)
			s.NotEmpty(res)
		}
	})

	s.Run("Run fetch on hardcoded query", func() {
		query := "SELECT * from users:arthur fetch friends.*"
		res, err := s.db.Query(query, map[string]interface{}{})
		s.NoError(err)
		s.NotEmpty(res)

		userSlice, err := marshal.SmartUnmarshal[testUserWithFriend[testUserWithFriend[interface{}]]](res, err)
		s.NoError(err)

		s.Require().Len(userSlice, 1)
		s.Require().Len(userSlice[0].Friends, 1)
		s.Require().NotEmpty(userSlice[0].Friends[0], 1)
	})

	s.Run("Run fetch on query using map[string]interface{} for thing and fetchString", func() {
		s.T().Skip("TODO(gh-116) Fetch unimplemented")
		res, err := s.db.Query("select * from $record fetch $fetchstr;", map[string]interface{}{
			"record":   "users",
			"fetchstr": "friends.*",
		})
		s.NoError(err)
		s.NotEmpty(res)
	})

	s.Run("Run fetch on query using map[string]interface{} for fetchString", func() {
		s.T().Skip("TODO(gh-116) Fetch unimplemented")
		res, err := s.db.Query("select * from users fetch $fetchstr;", map[string]interface{}{
			"fetchstr": "friends.*",
		})
		s.NoError(err)
		s.NotEmpty(res)
	})

	s.Run("Run fetch on query using map[string]interface{} for thing or tableName", func() {
		res, err := s.db.Query("select * from $record fetch friends.*;", map[string]interface{}{
			"record": "users:arthur",
		})
		s.NoError(err)
		s.NotEmpty(res)

		userSlice, err := marshal.SmartUnmarshal[testUserWithFriend[testUserWithFriend[interface{}]]](res, err)
		s.NoError(err)

		s.Require().Len(userSlice, 1)
		s.Require().Len(userSlice[0].Friends, 1)
		s.Require().NotEmpty(userSlice[0].Friends[0], 1)
	})
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
				Password: "123",
			},
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

func (s *SurrealDBTestSuite) TestMerge() {
	_, err := s.db.Create("users:999", map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	s.NoError(err)

	// Update the user
	_, err = s.db.Merge("users:999", map[string]string{
		"password": "456",
	})
	s.Require().NoError(err)

	user2, err := s.db.Select("users:999")
	s.Require().NoError(err)

	username := (user2).(map[string]interface{})["username"].(string)
	password := (user2).(map[string]interface{})["password"].(string)

	s.Equal("john999", username) // Ensure username hasn't change.
	s.Equal("456", password)
}

func (s *SurrealDBTestSuite) TestPatch() {
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
	_, err = s.db.Patch("users:999", patches)
	s.Require().NoError(err)

	user2, err := s.db.Select("users:999")
	s.Require().NoError(err)

	username := (user2).(map[string]interface{})["username"].(string)
	data := (user2).(map[string]interface{})["age"].(float64)

	s.Equal("john999", username) // Ensure username hasn't change.
	s.EqualValues(patches[1].Value, data)
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

func (s *SurrealDBTestSuite) TestConnectionBreak() {
	ws := connection.NewWebSocketConnection(connection.NewConnectionParams{})
	var url string
	if currentURL == "" {
		url = defaultURL
	} else {
		url = currentURL
	}

	db := s.openConnection(url, ws)
	// Close the connection hard from ws
	ws.Conn.Close()

	// Needs to be return error when the connection is closed or broken
	_, err := db.Select("users")
	s.Require().Error(err)
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
