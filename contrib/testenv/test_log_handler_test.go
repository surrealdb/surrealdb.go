package testenv

import (
	"context"
	"log/slog"
	"time"
)

func ExampleNewTestLogHandler() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	logger.Info("Application started")
	logger.Warn("Cache miss", slog.String("key", "user:123"))
	logger.Error("Database connection failed", slog.Int("retry", 3))

	// Output:
	// [0] INFO: Application started
	// [1] WARN: Cache miss key=user:123
	// [2] ERROR: Database connection failed retry=3
}

func ExampleNewTestLogHandler_withAttributes() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	logger.Info("message with attrs",
		slog.String("key1", "value1"),
		slog.Int("count", 42),
		slog.Bool("enabled", true))

	// Output:
	// [0] INFO: message with attrs key1=value1, count=42, enabled=true
}

func ExampleNewTestLogHandler_multipleMessages() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	logger.Info("first message")
	logger.Warn("warning message")
	logger.Error("error message")
	logger.Debug("debug message")

	// Output:
	// [0] INFO: first message
	// [1] WARN: warning message
	// [2] ERROR: error message
	// [3] DEBUG: debug message
}

func ExampleNewTestLogHandler_indexIncrement() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	for i := 0; i < 5; i++ {
		logger.Info("test message")
	}

	// Output:
	// [0] INFO: test message
	// [1] INFO: test message
	// [2] INFO: test message
	// [3] INFO: test message
	// [4] INFO: test message
}

func ExampleNewTestLogHandler_withAttrs() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	logger.Info("first")
	loggerWithAttrs := logger.With(slog.String("request_id", "123"))
	loggerWithAttrs.Info("second")
	loggerWithAttrs.Info("third", slog.Int("count", 42))

	// Output:
	// [0] INFO: first
	// [1] INFO: second request_id=123
	// [2] INFO: third request_id=123, count=42
}

func ExampleNewTestLogHandler_withGroup() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	groupedLogger := logger.WithGroup("mygroup")
	groupedLogger.Info("grouped message", slog.String("key", "value"))

	// Output:
	// [0] INFO: grouped message mygroup.key=value
}

func ExampleNewTestLogHandler_nestedGroups() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	logger.
		WithGroup("app").
		WithGroup("db").
		Info("query executed",
			slog.String("query", "SELECT * FROM users"),
			slog.Duration("duration", 100*time.Millisecond))

	// Output:
	// [0] INFO: query executed app.db.query=SELECT * FROM users, app.db.duration=100ms
}

func ExampleNewTestLogHandler_emptyGroup() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	// Empty group name should be ignored as per slog documentation
	logger.
		WithGroup("").
		Info("message", slog.String("key", "value"))

	// Output:
	// [0] INFO: message key=value
}

func ExampleNewTestLogHandler_withAttrsAndGroup() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	// Combine WithAttrs and WithGroup
	logger.
		With(slog.String("service", "api")).
		WithGroup("request").
		With(slog.String("method", "GET")).
		Info("handled", slog.Int("status", 200))

	// Output:
	// [0] INFO: handled service=api, request.method=GET, request.status=200
}

func ExampleNewTestLogHandler_enabled() {
	handler := NewTestLogHandler()
	ctx := context.Background()

	// All levels are enabled
	if handler.Enabled(ctx, slog.LevelDebug) {
		slog.New(handler).Debug("debug enabled")
	}
	if handler.Enabled(ctx, slog.LevelInfo) {
		slog.New(handler).Info("info enabled")
	}
	if handler.Enabled(ctx, slog.LevelWarn) {
		slog.New(handler).Warn("warn enabled")
	}
	if handler.Enabled(ctx, slog.LevelError) {
		slog.New(handler).Error("error enabled")
	}

	// Output:
	// [0] DEBUG: debug enabled
	// [1] INFO: info enabled
	// [2] WARN: warn enabled
	// [3] ERROR: error enabled
}

func ExampleNewTestLogHandlerWithFrames() {
	handler := NewTestLogHandlerWithFrames()
	logger := slog.New(handler)

	logger.Info("Debugging with frame info")

	// Output will include file:line information before the level
	// The exact output varies based on the execution context
}

func ExampleNewTestLogHandler_complexAttributes() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	timestamp, _ := time.Parse(time.RFC3339, "2024-01-01T12:00:00Z")
	logger.Info("user action",
		slog.Group("user",
			slog.String("id", "123"),
			slog.String("name", "John"),
		),
		slog.Group("action",
			slog.String("type", "login"),
			slog.Time("timestamp", timestamp),
		),
	)

	// Output:
	// [0] INFO: user action user.id=123, user.name=John, action.type=login, action.timestamp=2024-01-01 12:00:00 +0000 UTC
}

func ExampleNewTestLogHandlerWithOptions_ignoreErrors() {
	handler := NewTestLogHandlerWithOptions(
		WithIgnoreErrorPrefixes("Failed to unmarshal", "Connection error"),
	)
	logger := slog.New(handler)

	// These errors will be ignored
	logger.Error("Failed to unmarshal response", "error", "invalid data")
	logger.Error("Connection error: timeout")

	// This error will be logged
	logger.Error("Database error", "table", "users")

	// Info messages are always logged
	logger.Info("Server started")

	// Output:
	// [0] ERROR: Database error table=users
	// [1] INFO: Server started
}

func ExampleNewTestLogHandler_nestedGroupAttribute() {
	handler := NewTestLogHandler()
	logger := slog.New(handler)

	// Test nested groups: WithGroup context + slog.Group attribute
	logger.
		WithGroup("server").
		Info("request handled",
			slog.Group("request",
				slog.String("method", "POST"),
				slog.String("path", "/api/users"),
				slog.Group("headers",
					slog.String("content-type", "application/json"),
					slog.Int("content-length", 42),
				),
			),
			slog.Int("status", 201),
		)

	// Output:
	// [0] INFO: request handled server.request.method=POST, server.request.path=/api/users, server.request.headers.content-type=application/json, server.request.headers.content-length=42, server.status=201
}

func ExampleNewTestLogHandlerWithOptions_ignoreDebug() {
	handler := NewTestLogHandlerWithOptions(
		WithIgnoreDebug(),
	)
	logger := slog.New(handler)

	logger.Debug("This debug message will be ignored")
	logger.Info("Application started")
	logger.Debug("Another debug message that will be ignored")
	logger.Warn("Warning: Low memory")
	logger.Error("Connection failed")

	// Output:
	// [0] INFO: Application started
	// [1] WARN: Warning: Low memory
	// [2] ERROR: Connection failed
}
