package slog

import (
	"context"
	"log/slog"
)

type SlogHandler struct {
	logger *slog.Logger
}

func New(h slog.Handler) *SlogHandler {
	logger := slog.New(h)
	return &SlogHandler{logger: logger}
}

func (handler *SlogHandler) Error(ctx context.Context, msg string, args ...any) {
	handler.logger.ErrorContext(ctx, msg, args...)
}

func (handler *SlogHandler) Warn(ctx context.Context, msg string, args ...any) {
	handler.logger.WarnContext(ctx, msg, args...)
}

func (handler *SlogHandler) Info(ctx context.Context, msg string, args ...any) {
	handler.logger.InfoContext(ctx, msg, args...)
}

func (handler *SlogHandler) Debug(ctx context.Context, msg string, args ...any) {
	handler.logger.DebugContext(ctx, msg, args...)
}
