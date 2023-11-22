package slog

import (
	"log/slog"
)

type SlogHandler struct {
	logger *slog.Logger
}

func New(h slog.Handler) *SlogHandler {
	logger := slog.New(h)
	return &SlogHandler{logger: logger}
}

func (handler *SlogHandler) Error(msg string, args ...any) {
	handler.logger.Error(msg, args...)
}

func (handler *SlogHandler) Warn(msg string, args ...any) {
	handler.logger.Warn(msg, args...)
}

func (handler *SlogHandler) Info(msg string, args ...any) {
	handler.logger.Info(msg, args...)
}

func (handler *SlogHandler) Debug(msg string, args ...any) {
	handler.logger.Debug(msg, args...)
}
