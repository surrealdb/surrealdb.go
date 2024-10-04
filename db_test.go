package surrealdb_test

import (
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/v2"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
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
