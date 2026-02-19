package logger

import (
	"log/slog"
	"os"
)

var log *slog.Logger

func init() {
	level := slog.LevelInfo
	if os.Getenv("KORA_DEBUG") == "true" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stderr, opts)
	log = slog.New(handler)
}

func Debug(msg string, args ...any) {
	log.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	log.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	log.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	log.Error(msg, args...)
}

func Fatal(msg string, args ...any) {
	log.Error(msg, args...)
	os.Exit(1)
}
