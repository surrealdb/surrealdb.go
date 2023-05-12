package logger_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

func TestLog(t *testing.T) {
	templogger, err := logger.CreateLogFile("../../surrealdb.log")
	require.NoError(t, err)
	require.NotNil(t, templogger)
	require.NotNil(t, templogger.LogFile)
	require.NotNil(t, templogger.Logger)
	// Get Stats Before
	stats, err := templogger.LogFile.Stat()
	require.NoError(t, err)
	require.Equal(t, stats.Size(), int64(0))
	templogger.Logger.Info().Msg("Test")
	// Get Stats After
	stats, err = templogger.LogFile.Stat()
	require.NoError(t, err)
	require.Greater(t, stats.Size(), int64(0))
	// Delete Log File
	_, err = templogger.LogFile.Write([]byte{})
	require.NoError(t, err)
	err = os.Remove("../../surrealdb.log")
	require.NoError(t, err)
}
