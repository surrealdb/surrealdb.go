package logger

import (
	"os"

	"github.com/rs/zerolog"
)

const (
	permission = 0664
)

type LogData struct {
	LogFile *os.File
	Logger  *zerolog.Logger
}

func CreateLogFile(path string) (*LogData, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, permission)
	if err != nil {
		return nil, err
	}

	newlogger := zerolog.New(zerolog.SyncWriter(file)).With().Timestamp().Logger()
	return &LogData{LogFile: file, Logger: &newlogger}, err
}
