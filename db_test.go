package surrealdb_test

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/pkg/connection"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
	"os"
	"sync"
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
				// s.Require().Equal(err, constants.ErrNoRow)
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

type Person struct {
	ID       *models.RecordID     `json:"id,omitempty"`
	Name     string               `json:"name"`
	Surname  string               `json:"surname"`
	Location models.GeometryPoint `json:"location"`
}

func TestReadme(t *testing.T) {
	// Connect to SurrealDB
	db, err := surrealdb.New("ws://localhost:8000")
	if err != nil {
		panic(err)
	}

	// Set the namespace and database
	if err = db.Use("testNS", "testDB"); err != nil {
		panic(err)
	}

	// Sign in to authentication `db`
	authData := &surrealdb.Auth{
		Username: "root", // use your setup username
		Password: "root", // use your setup password
	}
	token, err := db.SignIn(authData)
	if err != nil {
		panic(err)
	}

	// Check token validity. This is not necessary if you called `SignIn` before. This authenticates the `db` instance too if sign in was
	// not previously called
	if err := db.Authenticate(token); err != nil {
		panic(err)
	}

	// And we can later on invalidate the token if desired
	defer func(token string) {
		if err := db.Invalidate(); err != nil {
			panic(err)
		}
	}(token)

	// Create an entry
	person1, err := surrealdb.Create[Person](db, models.Table("persons"), surrealdb.H{
		"Name":     "John",
		"Surname":  "Doe",
		"Location": models.NewGeometryPoint(-0.11, 22.00),
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created person with a map: %+v\n", person1)

	// Or use structs
	person2, err := surrealdb.Create[Person](db, models.Table("persons"), Person{
		Name:     "John",
		Surname:  "Doe",
		Location: models.NewGeometryPoint(-0.11, 22.00),
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created person with a struvt: %+v\n", person2)

	// Get entry by Record ID
	person, err := surrealdb.Select[Person, models.RecordID](db, *person1.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected a person by record id: %+v\n", person)

	// Or retrieve the entire table
	persons, err := surrealdb.Select[[]Person, models.Table](db, models.Table("persons"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected all in persons table: %+v\n", persons)

	// Delete an entry by ID
	if err = surrealdb.Delete[models.RecordID](db, *person2.ID); err != nil {
		panic(err)
	}

	// Delete all entries
	if err = surrealdb.Delete[models.Table](db, models.Table("persons")); err != nil {
		panic(err)
	}

	// Confirm empty table
	persons, err = surrealdb.Select[[]Person](db, models.Table("persons"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("No Selected person: %+v\n", persons)
}
