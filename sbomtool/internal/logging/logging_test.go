package logging

import (
	"context"
	"log/slog"
	"testing"
)

// TestLogLevelConversion verifies that Configure correctly normalizes input strings and sets the proper log level.
//
// Given: Various string inputs including valid levels, case variations, and invalid inputs
// When: Configure is called with each input
// Then: Should set up logging with the correct level or default to info for unknown levels
func TestLogLevelConversion(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	tests := []struct {
		name          string
		input         string
		expectedLevel slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"warning level", "warning", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"uppercase debug", "DEBUG", slog.LevelDebug},
		{"mixed case warn", "WaRn", slog.LevelWarn},
		{"unknown level", "unknown", slog.LevelInfo}, // Default to info for unknown levels
		{"empty string", "", slog.LevelInfo},         // Default to info for empty string
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Configure(tc.input)

			// Verify that a new default logger was set
			logger := slog.Default()
			if logger == nil {
				t.Errorf("Configure(%q) did not set a default logger", tc.input)
				return
			}

			// Test that the expected level is enabled and levels below it are disabled
			handler := logger.Handler()

			// The expected level should be enabled
			if !handler.Enabled(context.TODO(), tc.expectedLevel) {
				t.Errorf("Configure(%q) did not enable expected level %v", tc.input, tc.expectedLevel)
			}

			// Test specific level behaviors based on what was set
			switch tc.expectedLevel {
			case slog.LevelDebug:
				// Debug level should enable all levels
				if !handler.Enabled(context.TODO(), slog.LevelDebug) || !handler.Enabled(context.TODO(), slog.LevelInfo) ||
					!handler.Enabled(context.TODO(), slog.LevelWarn) || !handler.Enabled(context.TODO(), slog.LevelError) {
					t.Errorf("Configure(%q) with debug level should enable all levels", tc.input)
				}
			case slog.LevelInfo:
				// Info level should disable debug but enable info, warn, error
				if handler.Enabled(context.TODO(), slog.LevelDebug) {
					t.Errorf("Configure(%q) with info level should disable debug", tc.input)
				}
				if !handler.Enabled(context.TODO(), slog.LevelInfo) || !handler.Enabled(context.TODO(), slog.LevelWarn) || !handler.Enabled(context.TODO(), slog.LevelError) {
					t.Errorf("Configure(%q) with info level should enable info, warn, and error", tc.input)
				}
			case slog.LevelWarn:
				// Warn level should disable debug and info but enable warn and error
				if handler.Enabled(context.TODO(), slog.LevelDebug) || handler.Enabled(context.TODO(), slog.LevelInfo) {
					t.Errorf("Configure(%q) with warn level should disable debug and info", tc.input)
				}
				if !handler.Enabled(context.TODO(), slog.LevelWarn) || !handler.Enabled(context.TODO(), slog.LevelError) {
					t.Errorf("Configure(%q) with warn level should enable warn and error", tc.input)
				}
			case slog.LevelError:
				// Error level should only enable error
				if handler.Enabled(context.TODO(), slog.LevelDebug) || handler.Enabled(context.TODO(), slog.LevelInfo) || handler.Enabled(context.TODO(), slog.LevelWarn) {
					t.Errorf("Configure(%q) with error level should disable debug, info, and warn", tc.input)
				}
				if !handler.Enabled(context.TODO(), slog.LevelError) {
					t.Errorf("Configure(%q) with error level should enable error", tc.input)
				}
			}
		})
	}
}
