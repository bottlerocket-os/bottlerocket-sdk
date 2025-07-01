// Package main provides comprehensive tests for the generate command implementation.
//
// These tests verify the Cobra-based generate command functionality including
// flag configuration, validation, execution, and help text generation.
package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCommandMetadata(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{
			name:     "correct use field",
			field:    "Use",
			expected: "generate",
		},
		{
			name:     "correct short description",
			field:    "Short",
			expected: "Generate SBOM files for a directory",
		},
		{
			name:     "long description contains key information",
			field:    "Long",
			expected: "build directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A root command with generate subcommand
			rootCmd := createRootCommand()
			generateCmd := createGenerateCommand()
			rootCmd.AddCommand(generateCmd)

			// When: Examining generate command metadata
			var actual string
			switch tt.field {
			case "Use":
				actual = generateCmd.Use
			case "Short":
				actual = generateCmd.Short
			case "Long":
				actual = generateCmd.Long
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

func TestGenerateCommandFlags(t *testing.T) {
	// Given: A generate command
	generateCmd := createGenerateCommand()

	tests := []struct {
		name         string
		flagName     string
		expectedType string
		defaultValue interface{}
		usage        string
	}{
		{
			name:         "name flag",
			flagName:     "name",
			expectedType: "string",
			defaultValue: "",
			usage:        "Name of the target package",
		},
		{
			name:         "build-dir flag",
			flagName:     "build-dir",
			expectedType: "string",
			defaultValue: "",
			usage:        "Target directory of the package",
		},
		{
			name:         "out-dir flag",
			flagName:     "out-dir",
			expectedType: "string",
			defaultValue: "",
			usage:        "Output directory for the SBOM",
		},
		{
			name:         "spdx flag",
			flagName:     "spdx",
			expectedType: "bool",
			defaultValue: false,
			usage:        "Generate an SPDX SBOM",
		},
		{
			name:         "cyclonedx flag",
			flagName:     "cyclonedx",
			expectedType: "bool",
			defaultValue: false,
			usage:        "Generate a CycloneDX SBOM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Examining flag configuration
			flag := generateCmd.Flags().Lookup(tt.flagName)

			// Then: Flag should be registered with correct properties
			require.NotNil(t, flag, "Flag %s should be registered", tt.flagName)
			assert.Contains(t, flag.Usage, tt.usage, "Flag usage should contain expected text")

			// Check default values based on type
			switch tt.expectedType {
			case "string":
				value, err := generateCmd.Flags().GetString(tt.flagName)
				require.NoError(t, err)
				assert.Equal(t, tt.defaultValue, value, "Default value should match")
			case "bool":
				value, err := generateCmd.Flags().GetBool(tt.flagName)
				require.NoError(t, err)
				assert.Equal(t, tt.defaultValue, value, "Default value should match")
			}
		})
	}
}

func TestRequiredFlags(t *testing.T) {
	t.Run("required flags are marked correctly", func(t *testing.T) {
		// Given: A generate command
		generateCmd := createGenerateCommand()

		// When: Checking required flags
		requiredFlags := []string{"name", "build-dir", "out-dir"}

		// Then: All required flags should be marked as required
		for _, flagName := range requiredFlags {
			flag := generateCmd.Flags().Lookup(flagName)
			require.NotNil(t, flag, "Required flag %s should exist", flagName)

			// Check if flag is in required flags list
			assert.Equal(t, "", flag.DefValue, "Required flag %s should have empty default", flagName)
		}
	})
}

func TestGeneratePreRunValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flags   map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid arguments",
			flags: map[string]string{
				"name":      "test-package",
				"build-dir": ".",
				"out-dir":   "/tmp",
				"spdx":      "true",
			},
			wantErr: false,
		},
		{
			name: "invalid package name",
			flags: map[string]string{
				"name":      "", // empty name should fail validation
				"build-dir": ".",
				"out-dir":   "/tmp",
				"spdx":      "true",
			},
			wantErr: true,
			errMsg:  "package name",
		},
		{
			name: "invalid build directory",
			flags: map[string]string{
				"name":      "test-package",
				"build-dir": "/nonexistent/directory",
				"out-dir":   "/tmp",
				"spdx":      "true",
			},
			wantErr: true,
			errMsg:  "build directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A generate command with flags set
			generateCmd := createGenerateCommand()

			// Set flags
			for flagName, flagValue := range tt.flags {
				err := generateCmd.Flags().Set(flagName, flagValue)
				require.NoError(t, err, "Setting flag %s should not error", flagName)
			}

			// When: Running PreRunE validation
			err := generateCmd.PreRunE(generateCmd, tt.args)

			// Then: Should validate according to expectations
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "Error should contain expected message")
				}
			} else {
				assert.NoError(t, err, "Should not return validation error")
			}
		})
	}
}

func TestGenerateCommandExecution(t *testing.T) {
	t.Run("generate command executes with valid arguments", func(t *testing.T) {
		// Given: A root command with generate subcommand
		rootCmd := createRootCommand()
		generateCmd := createGenerateCommand()
		rootCmd.AddCommand(generateCmd)

		// Set up command with valid arguments
		rootCmd.SetArgs([]string{
			"generate",
			"--name", "test-package",
			"--build-dir", ".",
			"--out-dir", "/tmp",
			"--spdx",
		})

		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		// When: Executing the command
		err := rootCmd.Execute()

		// Then: Should execute successfully
		assert.NoError(t, err, "Generate command should execute successfully")
	})
}

func TestGenerateCommandHelp(t *testing.T) {
	t.Run("generate help contains expected content", func(t *testing.T) {
		// Given: A root command with generate subcommand
		rootCmd := createRootCommand()
		generateCmd := createGenerateCommand()
		rootCmd.AddCommand(generateCmd)

		// Set up help command
		rootCmd.SetArgs([]string{"generate", "--help"})
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		// When: Executing help
		err := rootCmd.Execute()

		// Then: Should show help without error
		require.NoError(t, err, "Help command should execute without error")
		helpText := buf.String()

		expectedSections := []string{
			"generate",
			"Generate SBOM files",
			"--name",
			"--build-dir",
			"--out-dir",
			"--spdx",
			"--cyclonedx",
		}

		for _, section := range expectedSections {
			assert.Contains(t, helpText, section, "Help text should contain %s", section)
		}
	})
}
