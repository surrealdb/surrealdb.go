package connection

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type testUser struct {
	Username string           `json:"username,omitempty"`
	Password string           `json:"password,omitempty"`
	ID       *models.RecordID `json:"id,omitempty"`
}

type ConnectionTestSuite struct {
	suite.Suite
	name                string
	connImplementations map[string]connection.Connection
}

func TestConnectionTestSuite(t *testing.T) {
	ts := new(ConnectionTestSuite)
	ts.connImplementations = make(map[string]connection.Connection)

	ts.connImplementations["ws"] = gorillaws.New(&connection.Config{
		BaseURL:     "ws://localhost:8000",
		Marshaler:   &models.CborMarshaler{},
		Unmarshaler: &models.CborUnmarshaler{},
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})

	ts.connImplementations["http"] = http.New(&connection.Config{
		BaseURL:     "http://localhost:8000",
		Marshaler:   &models.CborMarshaler{},
		Unmarshaler: &models.CborUnmarshaler{},
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})

	for wsName := range ts.connImplementations {
		// Run the test suite
		t.Run(wsName, func(t *testing.T) {
			ts.name = wsName
			suite.Run(t, ts)
		})
	}
}

// SetupSuite is called before the s starts running
func (s *ConnectionTestSuite) SetupSuite() {
	con := s.connImplementations[s.name]

	// connect
	err := con.Connect(context.Background())
	s.Require().NoError(err)

	// set namespace, database
	err = con.Use(context.Background(), "test", "test")
	s.Require().NoError(err)

	// sign in
	var token connection.RPCResponse[string]
	err = connection.Send(con, context.Background(), &token, "signin", map[string]any{
		"user": "root",
		"pass": "root",
	})
	s.Require().NoError(err)
	_ = con.Let(context.Background(), constants.AuthTokenKey, *token.Result)
}

func (s *ConnectionTestSuite) TearDownSuite() {
	con := s.connImplementations[s.name]
	err := con.Close(context.Background())
	s.Require().NoError(err)
}

func (s *ConnectionTestSuite) Test_CRUD() {
	con := s.connImplementations[s.name]

	var createRes connection.RPCResponse[testUser]
	err := connection.Send(con, context.Background(), &createRes, "create", "users", map[string]any{
		"username": "remi",
		"password": "password",
	})
	s.Require().NoError(err)

	s.Assert().Equal(createRes.Result.Username, "remi")
	s.Assert().Equal(createRes.Result.Password, "password")

	var selectRes connection.RPCResponse[testUser]
	err = connection.Send(con, context.Background(), &selectRes, "select", createRes.Result.ID)
	s.Require().NoError(err)

	s.Assert().Equal(createRes.Result.Username, "remi")
	s.Assert().Equal(createRes.Result.Password, "password")

	userToUpdate := createRes.Result
	userToUpdate.Password = "newpassword"
	var updateRes connection.RPCResponse[testUser]
	err = connection.Send(con, context.Background(), &updateRes, "update", userToUpdate.ID, userToUpdate)
	s.Require().NoError(err)

	s.Assert().Equal(userToUpdate.ID, updateRes.Result.ID)
	s.Assert().Equal(updateRes.Result.Password, "newpassword")

	err = connection.Send[any](con, context.Background(), nil, "delete", userToUpdate.ID)
	s.Require().NoError(err)

	var selectRes1 connection.RPCResponse[testUser]
	err = connection.Send(con, context.Background(), &selectRes1, "select", createRes.Result.ID)
	s.Require().NoError(err)
}
