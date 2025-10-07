// Package logging provides logging configuration for sbomtool.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Configure sets up structured logging with the specified level string.
// It handles case-insensitive input and defaults to info level for unknown values.
// After calling this, use slog directly throughout the application.
// We use JSON structured logging to enable better log parsing and analysis
// in automated systems, which is essential for SBOM generation workflows.
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
