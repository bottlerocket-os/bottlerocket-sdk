package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandMetadata(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{
			name:     "correct use field",
			field:    "Use",
			expected: "sbomtool",
		},
		{
			name:     "correct short description",
			field:    "Short",
			expected: "Software Bill of Materials (SBOM) generation tool",
		},
		{
			name:     "long description contains key information",
			field:    "Long",
			expected: "command-line utility",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A fresh root command
			cmd := createRootCommand()

			// When: Examining command metadata
			var actual string
			switch tt.field {
			case "Use":
				actual = cmd.Use
			case "Short":
				actual = cmd.Short
			case "Long":
				actual = cmd.Long
			}

			// Then: Command should have correct metadata
			if tt.field == "Long" {
				assert.Contains(t, actual, tt.expected, "Long description should contain key information")
			} else {
				assert.Equal(t, tt.expected, actual, "Command %s should match expected value", tt.field)
			}
		})
	}
}

func TestPersistentFlags(t *testing.T) {
	t.Run("log-level flag registration", func(t *testing.T) {
		// Given: A fresh root command
		cmd := createRootCommand()

		// When: Examining persistent flags
		flag := cmd.PersistentFlags().Lookup("log-level")

		// Then: Flag should be registered with correct defaults
		require.NotNil(t, flag, "log-level flag should be registered")
		assert.Equal(t, "info", flag.DefValue, "Default log level should be 'info'")
		assert.Equal(t, "Log level (debug, info, warn, error)", flag.Usage, "Flag usage should be descriptive")
	})

	t.Run("log-level flag default value", func(t *testing.T) {
		// Given: A fresh root command
		cmd := createRootCommand()

		// When: Getting the default flag value
		logLevel, err := cmd.PersistentFlags().GetString("log-level")

		// Then: Should return default value without error
		require.NoError(t, err, "Getting log-level flag should not error")
		assert.Equal(t, "info", logLevel, "Default log level should be 'info'")
	})
}

func TestLoggingConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		wantErr  bool
	}{
		{
			name:     "valid debug level",
			logLevel: "debug",
			wantErr:  false,
		},
		{
			name:     "valid info level",
			logLevel: "info",
			wantErr:  false,
		},
		{
			name:     "valid warn level",
			logLevel: "warn",
			wantErr:  false,
		},
		{
			name:     "valid error level",
			logLevel: "error",
			wantErr:  false,
		},
		{
			name:     "invalid log level defaults gracefully",
			logLevel: "invalid",
			wantErr:  false, // logging.Configure handles invalid levels gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A root command with log level set
			cmd := createRootCommand()
			cmd.PersistentFlags().Set("log-level", tt.logLevel)

			// When: Executing PersistentPreRunE
			err := cmd.PersistentPreRunE(cmd, []string{})

			// Then: Should handle log level appropriately
			if tt.wantErr {
				assert.Error(t, err, "Should return error for invalid configuration")
			} else {
				assert.NoError(t, err, "Should not return error for valid log level")
			}
		})
	}
}

func TestRootCommandHelp(t *testing.T) {
	t.Run("help text contains expected content", func(t *testing.T) {
		// Given: A root command
		cmd := createRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// When: Executing help (simulating --help flag)
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()

		// Then: Should execute without error and contain expected content
		require.NoError(t, err, "Help command should execute without error")
		helpText := buf.String()

		// The help text should contain key information from the Long description
		expectedSections := []string{
			"command-line utility",
			"Software Bill of Materials",
			"SBOM files",
		}

		for _, section := range expectedSections {
			assert.Contains(t, helpText, section, "Help text should contain %s", section)
		}
	})

	t.Run("help formatting is consistent", func(t *testing.T) {
		// Given: A root command
		cmd := createRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// When: Executing help
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()

		// Then: Should execute without error and be properly formatted
		require.NoError(t, err, "Help command should execute without error")
		helpText := buf.String()

		assert.True(t, len(helpText) > 0, "Help text should not be empty")
		assert.True(t, strings.Contains(helpText, "\n"), "Help text should contain newlines")
	})
}

func TestInvalidLogLevel(t *testing.T) {
	t.Run("graceful handling of invalid log levels", func(t *testing.T) {
		// Given: A root command with invalid log level
		cmd := createRootCommand()
		cmd.PersistentFlags().Set("log-level", "invalid-level")

		// When: Executing PersistentPreRunE
		err := cmd.PersistentPreRunE(cmd, []string{})

		// Then: Should not return error (logging package handles gracefully)
		assert.NoError(t, err, "Invalid log levels should be handled gracefully")
	})
}

func TestRootCommandExecution(t *testing.T) {
	t.Run("root command executes without errors", func(t *testing.T) {
		// Given: A root command
		cmd := createRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// When: Executing the root command
		err := cmd.Execute()

		// Then: Should execute without errors and show help
		assert.NoError(t, err, "Root command execution should not error")
	})

	t.Run("root command with log-level flag", func(t *testing.T) {
		// Given: A root command with log-level flag
		cmd := createRootCommand()
		cmd.SetArgs([]string{"--log-level", "debug"})
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// When: Executing the command
		err := cmd.Execute()

		// Then: Should execute successfully
		assert.NoError(t, err, "Root command with log-level flag should execute successfully")
	})
}
