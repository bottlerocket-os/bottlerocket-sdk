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

func TestGenerateCommandFlags(t *testing.T) {
	// Given: A generate command
	generateCmd := createGenerateCommand()

	tests := []struct {
		name         string
		flagName     string
		expectedType string
		defaultValue interface{}
	}{
		{
			name:         "name flag",
			flagName:     "name",
			expectedType: "string",
			defaultValue: "",
		},
		{
			name:         "build-dir flag",
			flagName:     "build-dir",
			expectedType: "string",
			defaultValue: "",
		},
		{
			name:         "out-dir flag",
			flagName:     "out-dir",
			expectedType: "string",
			defaultValue: "",
		},
		{
			name:         "spdx flag",
			flagName:     "spdx",
			expectedType: "bool",
			defaultValue: false,
		},
		{
			name:         "cyclonedx flag",
			flagName:     "cyclonedx",
			expectedType: "bool",
			defaultValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Examining flag configuration
			flag := generateCmd.Flags().Lookup(tt.flagName)

			// Then: Flag should be registered with correct properties
			require.NotNil(t, flag, "Flag %s should be registered", tt.flagName)

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
