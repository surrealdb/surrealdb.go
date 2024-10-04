package connection

import (
	"bytes"
	"github.com/stretchr/testify/suite"
	"github.com/surrealdb/surrealdb.go/v2/pkg/logger"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
	"log/slog"
	"os"
	"testing"
	"time"
)

type WsTestSuite struct {
	suite.Suite
	name                string
	connImplementations map[string]*WebSocketConnection
	logBuffer           *bytes.Buffer
}

func TestWsTestSuite(t *testing.T) {
	ts := new(WsTestSuite)
	ts.connImplementations = make(map[string]*WebSocketConnection)

	// Default
	ts.connImplementations["ws"] = NewWebSocketConnection(NewConnectionParams{
		BaseURL:     "ws://localhost:8000",
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	})

	RunWsMap(t, ts)
}

func RunWsMap(t *testing.T, s *WsTestSuite) {
	for wsName := range s.connImplementations {
		// Run the test suite
		t.Run(wsName, func(t *testing.T) {
			s.name = wsName
			suite.Run(t, s)
		})
	}
}

// SetupSuite is called before the s starts running
func (s *WsTestSuite) SetupSuite() {
	con := s.connImplementations[s.name]

	// connect
	err := con.Connect()
	s.Require().NoError(err)

	// set namespace, database
	err = con.Use("test", "test")
	s.Require().NoError(err)

	// sign in
	err = con.Send(nil, "signin", map[string]interface{}{
		"user": "root",
		"pass": "root",
	})
	s.Require().NoError(err)
}

func (s *WsTestSuite) TearDownSuite() {
	con := s.connImplementations[s.name]
	err := con.Close()
	s.Require().NoError(err)
}

func (s *WsTestSuite) TestEngine_WsMakeRequest() {
	con := s.connImplementations[s.name]

	params := []interface{}{
		"SELECT marketing, count() FROM $tb GROUP BY marketing",
		map[string]interface{}{
			"datetime": time.Now(),
			"testnil":  nil,
		},
	}

	var res RPCResponse[interface{}]
	err := con.Send(&res, "query", params...)
	s.Require().NoError(err, "no error returned when sending a query")
}
