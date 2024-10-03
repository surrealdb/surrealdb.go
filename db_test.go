package surrealdb_test

import (
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"os"
	"testing"
)

// Default const and vars for testing
const (
	defaultURL = "ws://localhost:8000"
)

var currentURL = os.Getenv("SURREALDB_URL")

func getURL() string {
	if currentURL == "" {
		return defaultURL
	}
	return currentURL
}

// TestDBSuite is a test s for the DB struct
type SurrealDBTestSuite struct {
	suite.Suite
	db   *surrealdb.DB
	name string
}

// a simple user struct for testing
type testUser struct {
	Username string           `json:"username,omitempty"`
	Password string           `json:"password,omitempty"`
	ID       *models.RecordID `json:"id,omitempty"`
}

// a simple user struct for testing
type testUserWithFriend[I any] struct {
	Username string           `json:"username,omitempty"`
	Password string           `json:"password,omitempty"`
	ID       *models.RecordID `json:"id,omitempty"`
	Friends  []I              `json:"friends,omitempty"`
}

func TestSurrealDBSuite(t *testing.T) {
	s := new(SurrealDBTestSuite)

	s.name = "Test_DB"
	suite.Run(t, s)
}

// SetupTest is called after each test
func (s *SurrealDBTestSuite) TearDownTest() {
	err := surrealdb.Delete[models.Table](s.db, "users")
	s.Require().NoError(err)
}

// TearDownSuite is called after the s has finished running
func (s *SurrealDBTestSuite) TearDownSuite() {
	err := s.db.Close()
	s.Require().NoError(err)
}

// SetupSuite is called before the s starts running
func (s *SurrealDBTestSuite) SetupSuite() {
	db, err := surrealdb.New(getURL())
	s.Require().NoError(err, "should not return an error when initializing db")
	s.db = db

	_ = signIn(s)

	err = db.Use("test", "test")
	s.Require().NoError(err, "should not return an error when setting namespace and database")
}

// Sign with the root user
// Can be used with any user
func signIn(s *SurrealDBTestSuite) string {
	authData := &surrealdb.Auth{
		Username: "root",
		Password: "root",
	}
	token, err := s.db.SignIn(authData)
	s.Require().NoError(err)
	return token
}

func (s *SurrealDBTestSuite) TestConnectionBreak() {
	db, err := surrealdb.New(getURL())
	s.Require().NoError(err, "should not return an error when initializing db")

	// Close the connection
	err = db.Close()
	s.Require().NoError(err, "should not return an error when closing underlying connection")

	// Needs to be return error when the connection is closed or broken
	_, err = surrealdb.Select[testUser](db, "users")
	s.Require().Error(err)
}

//func (s *SurrealDBTestSuite) TestLiveViaMethod() {
//	live, err := surrealdb.Live(s.db, "users", false)
//	s.Require().NoError(err, "should not return error on live request")
//
//	//defer func() {
//	//	err = surrealdb.Kill(s.db, live.String())
//	//	s.Require().NoError(err)
//	//}()
//
//	notifications, err := surrealdb.LiveNotifications(s.db, live.String())
//	s.Require().NoError(err)
//
//	_, e := surrealdb.Create[testUser](s.db, "users", map[string]interface{}{
//		"username": "johnny",
//		"password": "123",
//	})
//	s.Require().NoError(e)
//
//	notification := <-notifications
//	s.Require().Equal(connection.CreateAction, notification.Action)
//	s.Require().Equal(live, notification.ID)
//}

//
//func (s *SurrealDBTestSuite) TestLiveWithOptionsViaMethod() {
//	// create a user
//	user, e := surrealdb.Create[testUser](s.db, "users", map[string]interface{}{
//		"username": "johnny",
//		"password": "123",
//	})
//	s.Require().NoError(e)
//
//	live, err := surrealdb.Live(s.db, "users", true)
//	defer func() {
//		err = surrealdb.Kill(s.db, live)
//		s.Require().NoError(err)
//	}()
//
//	notifications, er := surrealdb.LiveNotifications(s.db, live)
//	s.Require().NoError(er)
//
//	// update the user
//	_, e = surrealdb.Update[testUser](s.db, user.ID, map[string]interface{}{
//		"password": "456",
//	})
//	s.Require().NoError(e)
//
//	notification := <-notifications
//	s.Require().Equal(connection.UpdateAction, notification.Action)
//	s.Require().Equal(live, notification.ID)
//}
//
////
////func (s *SurrealDBTestSuite) TestLiveViaQuery() {
////	responseArray, err := surrealdb.Query[[]interface{}](s.db, "LIVE SELECT * FROM users", map[string]interface{}{})
////	assert.NoError(s.T(), err)
////	singleResponse := responseArray[0].(map[string]interface{})
////	liveIDStruct, ok := singleResponse["result"]
////	assert.True(s.T(), ok)
////	liveID := liveIDStruct.(string)
////
////	defer func() {
////		err = surrealdb.Kill(s.db, liveID)
////		s.Require().NoError(err)
////	}()
////
////	notifications, er := surrealdb.LiveNotifications(s.db, liveID)
////	// create a user
////	s.Require().NoError(er)
////	_, e := surrealdb.Create[testUser](s.db, "users", map[string]interface{}{
////		"username": "johnny",
////		"password": "123",
////	})
////	s.Require().NoError(e)
////	notification := <-notifications
////	s.Require().Equal(connection.CreateAction, notification.Action)
////	s.Require().Equal(liveID, notification.ID)
////}

func (s *SurrealDBTestSuite) TestDelete() {
	_, err := surrealdb.Create[testUser](s.db, "users", testUser{
		Username: "johnny",
		Password: "123",
	})
	s.Require().NoError(err)

	// Delete the users...
	err = surrealdb.Delete(s.db, "users")
	s.Require().NoError(err)
}

//func (s *SurrealDBTestSuite) TestFetch() {
//	// Define initial user slice
//	userSlice := []testUserWithFriend[string]{
//		{
//			ID:       "users:arthur",
//			Username: "arthur",
//			Password: "deer",
//			Friends:  []string{"users:john"},
//		},
//		{
//			ID:       "users:john",
//			Username: "john",
//			Password: "wolf",
//			Friends:  []string{"users:arthur"},
//		},
//	}
//
//	// Initialize data using users
//	for _, v := range userSlice {
//		data, err := surrealdb.Create[testUser](s.db, v.ID, v)
//		s.NoError(err)
//		s.NotNil(data)
//	}
//
//	// User rows are individually fetched
//	s.Run("Run fetch for individual users", func() {
//		s.T().Skip("TODO(gh-116) Fetch unimplemented")
//		for _, v := range userSlice {
//			res, err := surrealdb.Query[[]interface{}](s.db, "select * from $table fetch $fetchstr;", map[string]interface{}{
//				"record":   v.ID,
//				"fetchstr": "friends.*",
//			})
//			s.NoError(err)
//			s.NotEmpty(res)
//		}
//	})
//
//	//s.Run("Run fetch on hardcoded query", func() {
//	//	query := "SELECT * from users:arthur fetch friends.*"
//	//	userSlice, err := surrealdb.Query[[]testUserWithFriend[interface{}]](s.db, query, map[string]interface{}{})
//	//	s.NoError(err)
//	//	s.NotEmpty(userSlice)
//	//
//	//	s.Require().Len(userSlice, 1)
//	//	s.Require().Len(userSlice[0].Friends, 1)
//	//	s.Require().NotEmpty(userSlice[0].Friends[0], 1)
//	//})
//
//	s.Run("Run fetch on query using map[string]interface{} for thing and fetchString", func() {
//		s.T().Skip("TODO(gh-116) Fetch unimplemented")
//		res, err := surrealdb.Query[[]interface{}](s.db, "select * from $record fetch $fetchstr;", map[string]interface{}{
//			"record":   "users",
//			"fetchstr": "friends.*",
//		})
//		s.NoError(err)
//		s.NotEmpty(res)
//	})
//
//	s.Run("Run fetch on query using map[string]interface{} for fetchString", func() {
//		s.T().Skip("TODO(gh-116) Fetch unimplemented")
//		res, err := surrealdb.Query[[]interface{}](s.db, "select * from users fetch $fetchstr;", map[string]interface{}{
//			"fetchstr": "friends.*",
//		})
//		s.NoError(err)
//		s.NotEmpty(res)
//	})
//
//	//s.Run("Run fetch on query using map[string]interface{} for thing or tableName", func() {
//	//	userSlice, err := surrealdb.Query[[]interface{}](s.db, "select * from $record fetch friends.*;", map[string]interface{}{
//	//		"record": "users:arthur",
//	//	})
//	//	s.NoError(err)
//	//	s.NotEmpty(userSlice)
//	//
//	//	s.Require().Len(userSlice, 1)
//	//	s.Require().Len(userSlice[0].Friends, 1)
//	//	s.Require().NotEmpty(userSlice[0].Friends[0], 1)
//	//})
//}

//func (s *SurrealDBTestSuite) TestInsert() {
//	s.Run("raw map works", func() {
//		insert, err := surrealdb.Insert[interface{}](s.db, "users", map[string]interface{}{
//			"username": "johnny",
//			"password": "123",
//		})
//		s.Require().NoError(err)
//
//		s.Equal("johnny", user[0].Username)
//		s.Equal("123", user[0].Password)
//	})
//
//	s.Run("Single insert works", func() {
//		userData, err := s.db.Insert("user", testUser{
//			Username: "johnny",
//			Password: "123",
//		})
//		s.Require().NoError(err)
//
//		// unmarshal the data into a user struct
//		var user []testUser
//		err = marshal.Unmarshal(userData, &user)
//		s.Require().NoError(err)
//
//		s.Equal("johnny", user[0].Username)
//		s.Equal("123", user[0].Password)
//	})
//
//	s.Run("Multiple insert works", func() {
//		userInsert := make([]testUser, 0)
//		userInsert = append(userInsert, testUser{
//			Username: "johnny1",
//			Password: "123",
//		}, testUser{
//			Username: "johnny2",
//			Password: "123",
//		})
//		userData, err := s.db.Insert("user", userInsert)
//		s.Require().NoError(err)
//
//		// unmarshal the data into a user struct
//		var users []testUser
//		err = marshal.Unmarshal(userData, &users)
//		s.Require().NoError(err)
//		s.Len(users, 2)
//
//		assertContains(s, users, func(user testUser) bool {
//			return s.Contains(users, user)
//		})
//	})
//}
//
//func (s *SurrealDBTestSuite) TestCreate() {
//	s.Run("raw map works", func() {
//		userData, err := s.db.Create("users", map[string]interface{}{
//			"username": "johnny",
//			"password": "123",
//		})
//		s.Require().NoError(err)
//
//		// unmarshal the data into a user struct
//		var userSlice []testUser
//		err = marshal.Unmarshal(userData, &userSlice)
//		s.Require().NoError(err)
//		s.Len(userSlice, 1)
//
//		s.Equal("johnny", userSlice[0].Username)
//		s.Equal("123", userSlice[0].Password)
//	})
//
//	s.Run("Single create works", func() {
//		userData, err := s.db.Create("users", testUser{
//			Username: "johnny",
//			Password: "123",
//		})
//		s.Require().NoError(err)
//
//		// unmarshal the data into a user struct
//		var userSlice []testUser
//		err = marshal.Unmarshal(userData, &userSlice)
//		s.Require().NoError(err)
//		s.Len(userSlice, 1)
//
//		s.Equal("johnny", userSlice[0].Username)
//		s.Equal("123", userSlice[0].Password)
//	})
//
//	s.Run("Multiple creates works", func() {
//		s.T().Skip("Creating multiple records is not supported yet")
//		data := make([]testUser, 0)
//		data = append(data,
//			testUser{
//				Username: "johnny",
//				Password: "123",
//			},
//			testUser{
//				Username: "joe",
//				Password: "123",
//			})
//		userData, err := s.db.Create("users", data)
//		s.Require().NoError(err)
//
//		// unmarshal the data into a user struct
//		var users []testUser
//		err = marshal.Unmarshal(userData, &users)
//		s.Require().NoError(err)
//
//		assertContains(s, users, func(user testUser) bool {
//			return s.Contains(users, user)
//		})
//	})
//}

////func (s *SurrealDBTestSuite) TestSelect() {
////	createdUsers, err := s.db.Create("users", testUser{
////		Username: "johnnyjohn",
////		Password: "123",
////	})
////	s.Require().NoError(err)
////	s.NotEmpty(createdUsers)
////	var createdUsersUnmarshalled []testUser
////	s.Require().NoError(marshal.Unmarshal(createdUsers, &createdUsersUnmarshalled))
////	s.NotEmpty(createdUsersUnmarshalled)
////	s.NotEmpty(createdUsersUnmarshalled[0].ID, "The ID should have been set by the database")
////
////	s.Run("Select many with table", func() {
////		userData, err := s.db.Select("users")
////		s.Require().NoError(err)
////
////		// unmarshal the data into a user slice
////		var users []testUser
////		err = marshal.Unmarshal(userData, &users)
////		s.NoError(err)
////		matching := assertContains(s, users, func(item testUser) bool {
////			return item.Username == "johnnyjohn"
////		})
////		s.GreaterOrEqual(len(matching), 1)
////	})
////
////	s.Run("Select single record", func() {
////		userData, err := s.db.Select(createdUsersUnmarshalled[0].ID)
////		s.Require().NoError(err)
////
////		// unmarshal the data into a user struct
////		var user testUser
////		err = marshal.Unmarshal(userData, &user)
////		s.Require().NoError(err)
////
////		s.Equal("johnnyjohn", user.Username)
////		s.Equal("123", user.Password)
////	})
////}
////
////func (s *SurrealDBTestSuite) TestUpdate() {
////	newPassword := "456"
////	users := []testUser{
////		{Username: "Johnny", Password: "123"},
////		{Username: "Mat", Password: "555"},
////	}
////
////	// create users
////	var createdUsers []testUser
////	for _, v := range users {
////		createdUser, err := s.db.Create("users", v)
////		s.Require().NoError(err)
////		var tempUserArr []testUser
////		err = marshal.Unmarshal(createdUser, &tempUserArr)
////		s.Require().NoError(err)
////		createdUsers = append(createdUsers, tempUserArr...)
////	}
////
////	createdUsers[0].Password = newPassword
////
////	// Update the user
////	UpdatedUserRaw, err := s.db.Update(createdUsers[0].ID, createdUsers[0])
////	s.Require().NoError(err)
////
////	// unmarshal the data into a user struct
////	var updatedUser testUser
////	err = marshal.Unmarshal(UpdatedUserRaw, &updatedUser)
////	s.Require().NoError(err)
////
////	// Check if password changes
////	s.Equal(newPassword, updatedUser.Password)
////
////	// select controlUser
////	controlUserRaw, err := s.db.Select(createdUsers[1].ID)
////	s.Require().NoError(err)
////
////	// unmarshal the data into a user struct
////	var controlUser testUser
////	err = marshal.Unmarshal(controlUserRaw, &controlUser)
////	s.Require().NoError(err)
////
////	// check control user is changed or not
////	s.Equal(createdUsers[1], controlUser)
////}
////
////func (s *SurrealDBTestSuite) TestMerge() {
////	_, err := s.db.Create("users:999", map[string]interface{}{
////		"username": "john999",
////		"password": "123",
////	})
////	s.NoError(err)
////
////	// Update the user
////	_, err = s.db.Merge("users:999", map[string]string{
////		"password": "456",
////	})
////	s.Require().NoError(err)
////
////	user2, err := s.db.Select("users:999")
////	s.Require().NoError(err)
////
////	username := (user2).(map[string]interface{})["username"].(string)
////	password := (user2).(map[string]interface{})["password"].(string)
////
////	s.Equal("john999", username) // Ensure username hasn't change.
////	s.Equal("456", password)
////}

func (s *SurrealDBTestSuite) TestPatch() {
	_, err := surrealdb.Create[testUser](s.db, models.NewRecordID("users:999"), map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	s.NoError(err)

	patches := []surrealdb.PatchData{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: int(44)},
	}

	// Update the user
	_, err = surrealdb.Patch(s.db, models.NewRecordID("users:999"), patches)
	s.Require().NoError(err)

	user2, err := surrealdb.Select[map[string]interface{}](s.db, models.NewRecordID("users:999"))
	s.Require().NoError(err)

	username := (*user2)["username"].(string)
	data := (*user2)["age"].(uint64)

	s.Equal("john999", username) // Ensure username hasn't change
	s.EqualValues(patches[1].Value, data)
}

func (s *SurrealDBTestSuite) TestNonRowSelect() {
	_, err := surrealdb.Select[testUser](s.db, models.NewRecordID("users:notexists"))
	s.Equal(err, constants.ErrNoRow)
}

////func (s *SurrealDBTestSuite) TestConcurrentOperations() {
////	var wg sync.WaitGroup
////	totalGoroutines := 100
////
////	s.Run(fmt.Sprintf("Concurrent select non existent rows %d", totalGoroutines), func() {
////		for i := 0; i < totalGoroutines; i++ {
////			wg.Add(1)
////			go func(j int) {
////				defer wg.Done()
////				_, err := surrealdb.Select[testUser](s.db, models.NewRecordID(fmt.Sprintf("users:%d", j)))
////				s.Require().Equal(err, constants.ErrNoRow)
////			}(i)
////		}
////		wg.Wait()
////	})
////
////	s.Run(fmt.Sprintf("Concurrent create rows %d", totalGoroutines), func() {
////		for i := 0; i < totalGoroutines; i++ {
////			wg.Add(1)
////			go func(j int) {
////				defer wg.Done()
////				_, err := surrealdb.Select[testUser](s.db, models.NewRecordID(fmt.Sprintf("users:%d", j)))
////				s.Require().NoError(err)
////			}(i)
////		}
////		wg.Wait()
////	})
////
////	s.Run(fmt.Sprintf("Concurrent select exist rows %d", totalGoroutines), func() {
////		for i := 0; i < totalGoroutines; i++ {
////			wg.Add(1)
////			go func(j int) {
////				defer wg.Done()
////				_, err := surrealdb.Select[testUser](s.db, models.NewRecordID(fmt.Sprintf("users:%d", j)))
////				s.Require().NoError(err)
////			}(i)
////		}
////		wg.Wait()
////	})
////}
//
//
//
//// assertContains performs an assertion on a list, asserting that at least one element matches a provided condition.
//// All the matching elements are returned from this function, which can be used as a filter.
//func assertContains[K any](s *SurrealDBTestSuite, input []K, matcher func(K) bool) []K {
//	matching := make([]K, 0)
//	for _, v := range input {
//		if matcher(v) {
//			matching = append(matching, v)
//		}
//	}
//	s.NotEmptyf(matching, "Input %+v did not contain matching element", fmt.Sprintf("%+v", input))
//	return matching
//}
//
//func TestDb(t *testing.T) {
//	db, err := surrealdb.New("http://localhost:8000")
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	err = db.Use("test", "test")
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	bearer, err := db.SignIn(&surrealdb.Auth{Username: "pass", Password: "pass"})
//	if err != nil {
//		fmt.Println(err)
//	}
//	fmt.Println(bearer)
//
//	newUser, err := surrealdb.Create[testUser](db, models.RecordID{Table: "users", ID: "ttrrddrr"}, surrealdb.H{
//		"Username": "remi",
//		"Password": "1234",
//	})
//	if err != nil {
//		fmt.Println(err)
//	}
//	fmt.Println(newUser)
//
//	//selectRes, err := surrealdb.Select[testUser, models.Table](db, "users")
//	selectRes, err := surrealdb.Select[testUser](db, models.Table("users"))
//	fmt.Println(selectRes)
//
//	queryRes, err := surrealdb.Query[testUser](db, "select * from $table", map[string]interface{}{
//		"table": models.Table("users"),
//	})
//
//	fmt.Println(queryRes)
//	fmt.Println(err)
//}
