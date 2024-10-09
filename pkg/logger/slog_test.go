package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	rawslog "log/slog"

	"github.com/stretchr/testify/require"
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
	logger := New(handler)

	testMethods := []testMethod{
		{fn: logger.Error, level: rawslog.LevelError},
		{fn: logger.Warn, level: rawslog.LevelWarn},
		{fn: logger.Info, level: rawslog.LevelInfo},
		{fn: logger.Debug, level: rawslog.LevelDebug},
	}

	for _, v := range testMethods {
		t.Run(fmt.Sprintf("testing %s", v.level.String()), func(tAlt *testing.T) {
			checkMethod(v.fn, buffer, v.level.String(), tAlt)
		})
		buffer.Reset()
	}
}

func checkMethod(loggerFunc func(msg string, args ...any), buffer *bytes.Buffer, levelStr string, t *testing.T) {
	require.NotEmpty(t, buffer)

	loggerFunc(LogText, CustomFieldName, CustomFieldVal)

	line := buffer.Bytes()

	testLogJSONVal := new(testLogJSON)
	err := json.Unmarshal(line, &testLogJSONVal)
	require.NoError(t, err)

	require.Equal(t, levelStr, testLogJSONVal.Level)
	require.Equal(t, LogText, testLogJSONVal.Msg)
	require.Equal(t, CustomFieldVal, testLogJSONVal.CustomVal)
}
