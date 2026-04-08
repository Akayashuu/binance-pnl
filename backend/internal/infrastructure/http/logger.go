package http

import (
	"log/slog"
	"os"

	"github.com/binancetracker/binancetracker/internal/application/ports"
)

// Logger adapts log/slog to the application ports.Logger interface.
type Logger struct {
	inner *slog.Logger
}

// NewLogger constructs a slog-backed logger at the given level.
func NewLogger(level string) *Logger {
	lvl := slog.LevelInfo
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return &Logger{inner: slog.New(h)}
}

// Compile-time port assertion.
var _ ports.Logger = (*Logger)(nil)

// Info logs at info level.
func (l *Logger) Info(msg string, kv ...any) { l.inner.Info(msg, kv...) }

// Warn logs at warn level.
func (l *Logger) Warn(msg string, kv ...any) { l.inner.Warn(msg, kv...) }

// Error logs at error level.
func (l *Logger) Error(msg string, kv ...any) { l.inner.Error(msg, kv...) }
