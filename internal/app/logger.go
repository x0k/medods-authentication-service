package app

import (
	"log"
	"log/slog"
	"os"

	"github.com/x0k/medods-authentication-service/internal/lib/logger"
)

const (
	DebugLevel = "debug"
	InfoLevel  = "info"
	WarnLevel  = "warn"
	ErrorLevel = "error"
)

const (
	TextHandler = "text"
	JSONHandler = "json"
)

func mustNewLogger(cfg *LoggerConfig) *logger.Logger {
	var level slog.Leveler
	switch cfg.Level {
	case DebugLevel:
		level = slog.LevelDebug
	case InfoLevel:
		level = slog.LevelInfo
	case WarnLevel:
		level = slog.LevelWarn
	case ErrorLevel:
		level = slog.LevelError
	default:
		log.Fatalf("Unknown level: %s, expect %q, %q, %q or %q", cfg.Level, DebugLevel, InfoLevel, WarnLevel, ErrorLevel)
	}
	options := &slog.HandlerOptions{
		Level: level,
	}
	var handler slog.Handler
	switch cfg.HandlerType {
	case TextHandler:
		handler = slog.NewTextHandler(os.Stdout, options)
	case JSONHandler:
		handler = slog.NewJSONHandler(os.Stdout, options)
	default:
		log.Fatalf("Unknown handler type: %s, expect %q or %q", cfg.HandlerType, TextHandler, JSONHandler)
	}
	return logger.New(slog.New(handler))
}
