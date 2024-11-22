package connection

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type EmbeddedConnectionTestSuite struct {
	suite.Suite
	con  *EmbeddedConnection
	name string
}

func TestEmbeddedConnectionTestSuite(t *testing.T) {
	s := new(EmbeddedConnectionTestSuite)
	s.name = "Test_Embedded_Connection"
	suite.Run(t, s)
}

// SetupSuite is called before the s starts running
func (s *EmbeddedConnectionTestSuite) SetupSuite() {
	con := NewEmbeddedConnection(NewConnectionParams{
		BaseURL:     "memory",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
	})

	err := con.Connect()
	s.Require().NoError(err, "no error during connection")

	s.con = con
}

// TearDownTest is called after each test
func (s *EmbeddedConnectionTestSuite) TearDownTest() {

}

// TearDownSuite is called after the s has finished running
func (s *EmbeddedConnectionTestSuite) TearDownSuite() {
	err := s.con.Close()
	s.Require().NoError(err)
}

func (s *EmbeddedConnectionTestSuite) TestSendRequest() {
	err := s.con.Use("test", "test")
	s.Require().NoError(err)

	var versionRes RPCResponse[string]
	err = s.con.Send(&versionRes, "version")
	s.Require().NoError(err)
}

func (s *EmbeddedConnectionTestSuite) TestLiveAndNotification() {
	err := s.con.Use("test", "test")
	s.Require().NoError(err)

	var liveRes RPCResponse[models.UUID]
	err = s.con.Send(&liveRes, "live", "users", false)
	s.Require().NoError(err, "should not return error on live request")

	liveID := liveRes.Result.String()
	defer func() {
		err = s.con.Send(nil, "kill", liveID)
		s.Require().NoError(err)
	}()

	notifications, err := s.con.LiveNotifications(liveID)
	s.Require().NoError(err)

	fmt.Println(notifications)

	// Notification reader not ready on C lib
}
