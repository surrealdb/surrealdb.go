package logger_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

func TestLog(t *testing.T) {
	buff := bytes.NewBuffer([]byte{})
	templogger, err := logger.New().FromBuffer(buff).Make()
	require.NoError(t, err)
	require.NotNil(t, templogger)
	require.NotNil(t, templogger.Logger)
	// Get Stats Before
	require.Equal(t, buff.Len(), 0)
	templogger.Logger.Info().Msg("Test")
	// Get Stats After
	require.Contains(t, buff.String(), "Test")
}
