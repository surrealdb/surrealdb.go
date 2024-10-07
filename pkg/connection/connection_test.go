package connection

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
	"github.com/surrealdb/surrealdb.go/v2/pkg/logger"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

type testUser struct {
	Username string           `json:"username,omitempty"`
	Password string           `json:"password,omitempty"`
	ID       *models.RecordID `json:"id,omitempty"`
}

type ConnectionTestSuite struct {
	suite.Suite
	name                string
	connImplementations map[string]Connection
}

func TestConnectionTestSuite(t *testing.T) {
	ts := new(ConnectionTestSuite)
	ts.connImplementations = make(map[string]Connection)

	ts.connImplementations["ws"] = NewWebSocketConnection(NewConnectionParams{
		BaseURL:     "ws://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})

	ts.connImplementations["http"] = NewHTTPConnection(NewConnectionParams{
		BaseURL:     "http://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
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
	err := con.Connect()
	s.Require().NoError(err)

	// set namespace, database
	err = con.Use("test", "test")
	s.Require().NoError(err)

	// sign in
	var token RPCResponse[string]
	err = con.Send(&token, "signin", map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	s.Require().NoError(err)
	_ = con.Let(constants.AuthTokenKey, *token.Result)
}

func (s *ConnectionTestSuite) TearDownSuite() {
	con := s.connImplementations[s.name]
	err := con.Close()
	s.Require().NoError(err)
}

func (s *ConnectionTestSuite) Test_CRUD() {
	con := s.connImplementations[s.name]

	var createRes RPCResponse[testUser]
	err := con.Send(&createRes, "create", "users", map[string]interface{}{
		"username": "remi",
		"password": "password",
	})
	s.Require().NoError(err)

	s.Assert().Equal(createRes.Result.Username, "remi")
	s.Assert().Equal(createRes.Result.Password, "password")

	var selectRes RPCResponse[testUser]
	err = con.Send(&selectRes, "select", createRes.Result.ID)
	s.Require().NoError(err)

	s.Assert().Equal(createRes.Result.Username, "remi")
	s.Assert().Equal(createRes.Result.Password, "password")

	userToUpdate := createRes.Result
	userToUpdate.Password = "newpassword"
	var updateRes RPCResponse[testUser]
	err = con.Send(&updateRes, "update", userToUpdate.ID, userToUpdate)
	s.Require().NoError(err)

	s.Assert().Equal(userToUpdate.ID, updateRes.Result.ID)
	s.Assert().Equal(updateRes.Result.Password, "newpassword")

	err = con.Send(nil, "delete", userToUpdate.ID)
	s.Require().NoError(err)

	var selectRes1 RPCResponse[testUser]
	err = con.Send(&selectRes1, "select", createRes.Result.ID)
	s.Require().NoError(err)
	// s.Assert().Equal(nil, selectRes1.Result)
}
