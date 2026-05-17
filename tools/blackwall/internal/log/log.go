// Package log builds the blackwall slog logger writing JSON to a file.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Options struct {
	File  string
	Level string
}

// Setup opens the log file (creating parent dirs), constructs a JSON
// slog.Logger and returns it alongside a Closer for graceful shutdown.
func Setup(o Options) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(o.File), 0o700); err != nil {
		return nil, nil, fmt.Errorf("mkdir log dir: %w", err)
	}
	f, err := os.OpenFile(o.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: parseLevel(o.Level)}))
	return logger, f, nil
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
