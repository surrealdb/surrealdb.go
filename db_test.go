package surrealdb_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

type testPerson struct {
	FirstName string           `json:"firstname,omitempty"`
	LastName  string           `json:"lastname,omitempty"`
	ID        *models.RecordID `json:"id,omitempty"`
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

func TestSurrealDBSuite(t *testing.T) {
	s := new(SurrealDBTestSuite)

	s.name = "Test_DB"
	suite.Run(t, s)
}

// SetupTest is called after each test
func (s *SurrealDBTestSuite) TearDownTest() {
	err := surrealdb.Delete[models.Table](s.db, "users")
	s.Require().NoError(err)

	err = surrealdb.Delete[models.Table](s.db, "persons")
	s.Require().NoError(err)

	err = surrealdb.Delete[models.Table](s.db, "knows")
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

func (s *SurrealDBTestSuite) TestSend_AllowedMethods() {
	s.Run("Send method should be rejected", func() {
		err := s.db.Send(nil, "let")
		s.Require().Error(err)
	})

	s.Run("Send method should be allowed", func() {
		err := s.db.Send(nil, "query", "select * from users")
		s.Require().NoError(err)
	})
}

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

func (s *SurrealDBTestSuite) TestInsert() {
	s.Run("raw map works", func() {
		insert, err := surrealdb.Insert[testUser](s.db, "users", map[string]interface{}{
			"username": "johnny",
			"password": "123",
		})
		s.Require().NoError(err)

		s.Equal("johnny", (*insert)[0].Username)
		s.Equal("123", (*insert)[0].Password)
	})

	s.Run("Single insert works", func() {
		insert, err := surrealdb.Insert[testUser](s.db, "users", testUser{
			Username: "johnny",
			Password: "123",
		})
		s.Require().NoError(err)

		s.Equal("johnny", (*insert)[0].Username)
		s.Equal("123", (*insert)[0].Password)
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
		insert, err := surrealdb.Insert[testUser](s.db, "users", userInsert)
		s.Require().NoError(err)
		s.Len(*insert, 2)
	})
}

func (s *SurrealDBTestSuite) TestPatch() {
	_, err := surrealdb.Create[testUser](s.db, *models.ParseRecordID("users:999"), map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	s.NoError(err)

	patches := []surrealdb.PatchData{
		{Op: "add", Path: "nickname", Value: "johnny"},
		{Op: "add", Path: "age", Value: int(44)},
	}

	// Update the user
	_, err = surrealdb.Patch(s.db, models.ParseRecordID("users:999"), patches)
	s.Require().NoError(err)

	user2, err := surrealdb.Select[map[string]interface{}](s.db, *models.ParseRecordID("users:999"))
	s.Require().NoError(err)

	username := (*user2)["username"].(string)
	data := (*user2)["age"].(uint64)

	s.Equal("john999", username) // Ensure username hasn't change
	s.EqualValues(patches[1].Value, data)
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
		createdUser, err := surrealdb.Create[testUser](s.db, models.Table("users"), v)
		s.Require().NoError(err)
		createdUsers = append(createdUsers, *createdUser)
	}

	createdUsers[0].Password = newPassword

	// Update the user
	updatedUser, err := surrealdb.Update[testUser](s.db, *(createdUsers)[0].ID, createdUsers[0])
	s.Require().NoError(err)

	// Check if password changes
	s.Equal(newPassword, updatedUser.Password)

	// select controlUser
	controlUser, err := surrealdb.Select[testUser](s.db, *createdUsers[1].ID)
	s.Require().NoError(err)

	// check control user is changed or not
	s.Equal(createdUsers[1], *controlUser)
}

func (s *SurrealDBTestSuite) TestLiveViaMethod() {
	live, err := surrealdb.Live(s.db, "users", false)
	s.Require().NoError(err, "should not return error on live request")

	defer func() {
		err = surrealdb.Kill(s.db, live.String())
		s.Require().NoError(err)
	}()

	notifications, err := s.db.LiveNotifications(live.String())
	s.Require().NoError(err)

	_, e := surrealdb.Create[testUser](s.db, "users", map[string]interface{}{
		"username": "johnny",
		"password": "123",
	})
	s.Require().NoError(e)

	notification := <-notifications
	fmt.Println(notification)
	s.Require().Equal(connection.CreateAction, notification.Action)
	s.Require().Equal(live, notification.ID)
}

func (s *SurrealDBTestSuite) TestLiveViaQuery() {
	res, err := surrealdb.Query[models.UUID](s.db, "LIVE SELECT * FROM users", map[string]interface{}{})
	s.Require().NoError(err)

	liveID := (*res)[0].Result.String()

	notifications, err := s.db.LiveNotifications(liveID)
	s.Require().NoError(err)

	defer func() {
		err = surrealdb.Kill(s.db, liveID)
		s.Require().NoError(err)
	}()

	// create user
	_, e := surrealdb.Create[testUser](s.db, "users", map[string]interface{}{
		"username": "johnny",
		"password": "123",
	})
	s.Require().NoError(e)
	notification := <-notifications

	s.Require().Equal(connection.CreateAction, notification.Action)
	s.Require().Equal(liveID, notification.ID.String())
}

func (s *SurrealDBTestSuite) TestCreate() {
	s.Run("raw map works", func() {
		user, err := surrealdb.Create[testUser](s.db, "users", map[string]interface{}{
			"username": "johnny",
			"password": "123",
		})
		s.Require().NoError(err)

		s.Equal("johnny", user.Username)
		s.Equal("123", user.Password)
	})

	s.Run("Single create works", func() {
		user, err := surrealdb.Create[testUser](s.db, "users", testUser{
			Username: "johnny",
			Password: "123",
		})
		s.Require().NoError(err)

		s.Equal("johnny", user.Username)
		s.Equal("123", user.Password)
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
		users, err := surrealdb.Create[[]testUser](s.db, "users", data)
		s.Require().NoError(err)

		assertContains(s, *users, func(user testUser) bool {
			return s.Contains(users, user)
		})
	})
}

func (s *SurrealDBTestSuite) TestSelect() {
	createdUser, err := surrealdb.Create[testUser](s.db, "users", testUser{
		Username: "johnnyjohn",
		Password: "123",
	})
	s.Require().NoError(err)
	s.NotEmpty(createdUser)

	s.Run("Select many with table", func() {
		users, err := surrealdb.Select[[]testUser](s.db, "users")
		s.Require().NoError(err)

		matching := assertContains(s, *users, func(item testUser) bool {
			return item.Username == "johnnyjohn"
		})
		s.GreaterOrEqual(len(matching), 1)
	})

	s.Run("Select single record", func() {
		user, err := surrealdb.Select[testUser](s.db, *createdUser.ID)
		s.Require().NoError(err)

		s.Equal("johnnyjohn", user.Username)
		s.Equal("123", user.Password)
	})
}

func (s *SurrealDBTestSuite) TestConcurrentOperations() {
	var wg sync.WaitGroup
	totalGoroutines := 100

	s.Run(fmt.Sprintf("Concurrent select non existent rows %d", totalGoroutines), func() {
		for i := 0; i < totalGoroutines; i++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				_, _ = surrealdb.Select[testUser](s.db, models.NewRecordID("users", j))
			}(i)
		}
		wg.Wait()
	})

	s.Run(fmt.Sprintf("Concurrent create rows %d", totalGoroutines), func() {
		for i := 0; i < totalGoroutines; i++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				_, err := surrealdb.Select[testUser](s.db, models.NewRecordID("users", j))
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
				_, err := surrealdb.Select[testUser](s.db, models.NewRecordID("users", j))
				s.Require().NoError(err)
			}(i)
		}
		wg.Wait()
	})
}

func (s *SurrealDBTestSuite) TestMerge() {
	_, err := surrealdb.Create[testUser](s.db, *models.ParseRecordID("users:999"), map[string]interface{}{
		"username": "john999",
		"password": "123",
	})
	s.NoError(err)

	// Update the user
	_, err = surrealdb.Merge[testUser](s.db, *models.ParseRecordID("users:999"), map[string]string{
		"password": "456",
	})
	s.Require().NoError(err)

	user, err := surrealdb.Select[testUser](s.db, *models.ParseRecordID("users:999"))
	s.Require().NoError(err)
	s.Equal("john999", user.Username) // Ensure username hasn't change.
	s.Equal("456", user.Password)
}

func (s *SurrealDBTestSuite) TestInsertRelation() {
	persons, err := surrealdb.Insert[testPerson](s.db, "person", []testPerson{
		{FirstName: "Mary", LastName: "Doe"},
		{FirstName: "John", LastName: "Doe"},
	})
	s.Require().NoError(err)

	relationship := surrealdb.Relationship{
		In:       *(*persons)[0].ID,
		Out:      *(*persons)[1].ID,
		Relation: "knows",
		Data: map[string]any{
			"since": time.Now(),
		},
	}
	err = surrealdb.InsertRelation(s.db, &relationship)
	s.Require().NoError(err)
	s.Assert().NotNil(relationship.ID)
}

func (s *SurrealDBTestSuite) TestRelate() {
	persons, err := surrealdb.Insert[testPerson](s.db, "person", []testPerson{
		{FirstName: "Mary", LastName: "Doe"},
		{FirstName: "John", LastName: "Doe"},
	})
	s.Require().NoError(err)

	relationship := surrealdb.Relationship{
		In:       *(*persons)[0].ID,
		Out:      *(*persons)[1].ID,
		Relation: "knows",
		Data: map[string]any{
			"since": time.Now(),
		},
	}
	err = surrealdb.Relate(s.db, &relationship)
	s.Require().NoError(err)
	s.Assert().NotNil(relationship.ID)
}
