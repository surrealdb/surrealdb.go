package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

const (
	permission = 0664
)

// In memory logger mode file will be nil
type LogData struct {
	LogFile *os.File
	Logger  *zerolog.Logger
}

// If path is empty it will use os.stdout/os.stderr
func NewLogger(path string) (_ *LogData, err error) {
	var writer io.Writer = os.Stderr
	var file *os.File
	if path != "" {
		file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, permission)
		if err != nil {
			return nil, err
		}
		writer = zerolog.SyncWriter(file)
	}
	return NewLoggerRaw(writer, file), err
}

func NewLoggerRaw(w io.Writer, f *os.File) *LogData {
	newlogger := zerolog.New(w).With().Timestamp().Logger()
	return &LogData{LogFile: f, Logger: &newlogger}
}
