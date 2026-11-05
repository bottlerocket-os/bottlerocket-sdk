// Package logging provides logging configuration for sbomtool.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Configure installs a JSON slog handler at the given level (case-insensitive, default info).
func Configure(levelStr string) {
	normalized := strings.ToLower(strings.TrimSpace(levelStr))

	var slogLevel slog.Level
	switch normalized {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo // Default to info for unknown levels
	}

	// Create a JSON handler for structured logging
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
