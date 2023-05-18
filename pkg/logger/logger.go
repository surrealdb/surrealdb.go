package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

const (
	permission = 0664
)

type LogBuild struct {
	writer     io.Writer
	path       string
	LogChannel chan string
}

type LogData struct {
	writer     io.Writer
	LogFile    *os.File
	Logger     zerolog.Logger
	LogChannel chan string
}

func New() *LogBuild {
	return &LogBuild{}
}

func (build *LogBuild) FromPath(path string) *LogBuild {
	build.path = path
	return build
}

func (build *LogBuild) FromBuffer(w io.Writer) *LogBuild {
	build.writer = w
	return build
}

func (build *LogBuild) FromChannel(chn chan string) *LogBuild {
	build.LogChannel = chn
	return build
}

func (build *LogBuild) Make() (logData *LogData, err error) {
	logData = new(LogData)
	logData.writer = os.Stdout
	logData.writer = build.writer
	logData.LogChannel = build.LogChannel
	if build.path != "" {
		logData.LogFile, err = os.OpenFile(build.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, permission)
		if err != nil {
			return nil, err
		}
		logData.writer = zerolog.SyncWriter(logData.LogFile)
	}
	logData.Logger = zerolog.New(logData.writer).With().Timestamp().Logger()
	return
}
