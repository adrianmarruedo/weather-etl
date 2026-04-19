package logger

import (
	"log/slog"
	"os"
)

func New(logLevel, logFile string) (*slog.Logger, *os.File, error) {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, err
	}

	level := slog.LevelInfo
	if logLevel == "debug" {
		level = slog.LevelDebug
	} else if logLevel == "warn" {
		level = slog.LevelWarn
	} else if logLevel == "error" {
		level = slog.LevelError
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	return slog.New(handler), f, nil
}
