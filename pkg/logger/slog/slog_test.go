package slog_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	rawslog "log/slog"

	"github.com/surrealdb/surrealdb.go/pkg/logger/slog"
)

type testMethod struct {
	fn    func(msg string, args ...any)
	level rawslog.Level
}

var (
	LogText         string = "Test Log Value"
	CustomFieldName string = "Somekey"
	CustomFieldVal  any    = "SomeVal"
)

type testLogJSON struct {
	Time  time.Time `json:"time"`
	Level string    `json:"level"`
	Msg   string    `json:"msg"`
	// Json field needs to match with CustomFieldName
	CustomVal any `json:"SomeKey"`
}

func TestLogger(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{})

	// level needs to be set to debug for log all
	handler := rawslog.NewJSONHandler(buffer, &rawslog.HandlerOptions{Level: rawslog.LevelDebug})
	logger := slog.New(handler)

	testMethods := []testMethod{
		{fn: logger.Error, level: rawslog.LevelError},
		{fn: logger.Warn, level: rawslog.LevelWarn},
		{fn: logger.Info, level: rawslog.LevelInfo},
		{fn: logger.Debug, level: rawslog.LevelDebug},
	}

	for _, v := range testMethods {
		t.Run(fmt.Sprintf("testing %s", v.level.String()), func(t *testing.T) {
			err := checkMethod(v.fn, buffer, v.level.String())
			if err != nil {
				t.Fatal(err)
			}
		})
		buffer.Reset()
	}
}

func checkMethod(loggerFunc func(msg string, args ...any), buffer *bytes.Buffer, levelStr string) error {
	if buffer.Len() > 0 {
		return fmt.Errorf("buffer needs to be 0 but it is %d", buffer.Len())
	}

	loggerFunc(LogText, CustomFieldName, CustomFieldVal)

	line := buffer.Bytes()

	testLogJSONVal := new(testLogJSON)
	err := json.Unmarshal(line, &testLogJSONVal)
	if err != nil {
		return err
	}

	if testLogJSONVal.Level != levelStr {
		return fmt.Errorf("Expected %s got %s as level", levelStr, testLogJSONVal.Level)
	}

	if testLogJSONVal.Msg != LogText {
		return fmt.Errorf("Expected %s got %s as msg", LogText, testLogJSONVal.Msg)
	}

	if testLogJSONVal.CustomVal != CustomFieldVal {
		return fmt.Errorf("Expected %s got %s as CustomFieldVal", CustomFieldName, testLogJSONVal.CustomVal)
	}

	return nil
}
