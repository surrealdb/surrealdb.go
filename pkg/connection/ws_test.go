package connection

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type WsTestSuite struct {
	suite.Suite
	name string
}

func TestWsTestSuite(t *testing.T) {
	ts := new(WsTestSuite)
	ts.name = "WS Test Suite"

	suite.Run(t, ts)
}

// SetupSuite is called before the s starts running
func (s *WsTestSuite) SetupSuite() {

}

func (s *WsTestSuite) TearDownSuite() {

}
