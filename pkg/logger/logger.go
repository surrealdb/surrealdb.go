package logger

import (
	"log"
	"os"
)

type LogData struct {
	LogFile *os.File
	Logger  *log.Logger
}

func CreateLogFile(path string) (logData *LogData, err error) {
	logData.LogFile, err = os.Create("surrealdbgo.log")
	if err != nil {
		return
	}
	logData.Logger = log.New(logData.LogFile, "Surreal: ", log.Ldate|log.Ltime)
	return
}
