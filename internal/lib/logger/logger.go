package logger

import (
	"context"
	"log/slog"
)

type Logger struct {
	*slog.Logger
}

func New(log *slog.Logger) *Logger {
	return &Logger{log}
}

func (l *Logger) With(attrs ...any) *Logger {
	return New(l.Logger.With(attrs...))
}

func (l *Logger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

func (l *Logger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *Logger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

func (l *Logger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}
