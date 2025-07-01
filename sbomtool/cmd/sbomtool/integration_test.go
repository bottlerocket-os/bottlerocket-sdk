// Package main provides integration tests for the complete CLI implementation.
//
// These tests verify the overall command hierarchy, flag inheritance,
// and end-to-end functionality of the Cobra-based CLI.
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandHierarchy(t *testing.T) {
	t.Run("all commands are properly registered", func(t *testing.T) {
		// Given: A root command with all subcommands
		rootCmd := createRootCommand()

		// When: Examining available commands
		commands := rootCmd.Commands()

		// Then: Should have expected commands (Cobra adds completion and help automatically)
		commandNames := make([]string, len(commands))
		for i, cmd := range commands {
			commandNames[i] = cmd.Name()
		}

		// Check for our explicitly added commands
		expectedCommands := []string{"generate", "merge"}
		for _, expected := range expectedCommands {
			assert.Contains(t, commandNames, expected, "Should contain %s command", expected)
		}

		// Verify we have at least our commands (Cobra may add others)
		assert.GreaterOrEqual(t, len(commandNames), 2, "Should have at least generate and merge commands")
	})
}

func TestGlobalFlagInheritance(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{
			name:    "generate command inherits log-level",
			command: "generate",
			args:    []string{"--log-level", "debug", "generate", "--help"},
		},
		{
			name:    "merge command inherits log-level",
			command: "merge",
			args:    []string{"--log-level", "debug", "merge", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A root command with subcommands
			rootCmd := createRootCommand()
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)

			// When: Executing command with global flag
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			// Then: Should execute without error (global flag inherited)
			assert.NoError(t, err, "Command should inherit global flags")
		})
	}
}

func TestCommandSuggestions(t *testing.T) {
	t.Run("suggests correct command for typos", func(t *testing.T) {
		// Given: A root command
		rootCmd := createRootCommand()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		// When: Executing with typo
		rootCmd.SetArgs([]string{"generat"}) // Missing 'e'
		err := rootCmd.Execute()

		// Then: Should suggest correct command
		assert.Error(t, err, "Should return error for unknown command")
		output := buf.String()
		assert.Contains(t, output, "generate", "Should suggest 'generate' command")
	})
}

func TestHelpTextConsistency(t *testing.T) {
	t.Run("all commands have consistent help formatting", func(t *testing.T) {
		// Given: A root command with all subcommands
		rootCmd := createRootCommand()

		commands := []string{"generate", "merge"}
		for _, cmdName := range commands {
			t.Run(cmdName+" help formatting", func(t *testing.T) {
				// When: Getting help for command
				buf := new(bytes.Buffer)
				rootCmd.SetOut(buf)
				rootCmd.SetErr(buf)
				rootCmd.SetArgs([]string{cmdName, "--help"})

				err := rootCmd.Execute()

				// Then: Should have consistent formatting
				require.NoError(t, err, "Help should execute without error")
				helpText := buf.String()

				// Check for consistent sections
				expectedSections := []string{"Usage:", "Flags:", "Global Flags:"}
				for _, section := range expectedSections {
					assert.Contains(t, helpText, section, "Help should contain %s section", section)
				}
			})
		}
	})
}

func TestEndToEndCLI(t *testing.T) {
	t.Run("complete CLI workflow", func(t *testing.T) {
		// Given: A complete CLI application
		rootCmd := createRootCommand()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		// When: Testing main help
		rootCmd.SetArgs([]string{"--help"})
		err := rootCmd.Execute()

		// Then: Should show main help with all commands
		require.NoError(t, err, "Main help should execute without error")
		helpText := buf.String()

		// Verify main help contains all expected elements
		expectedElements := []string{
			"sbomtool",
			"Software Bill of Materials",
			"Available Commands:",
			"generate",
			"merge",
			"--log-level",
		}

		for _, element := range expectedElements {
			assert.Contains(t, helpText, element, "Main help should contain %s", element)
		}
	})
}

func TestFlagCompatibility(t *testing.T) {
	t.Run("all original flags work identically", func(t *testing.T) {
		// Given: Original flag combinations that should work
		testCases := []struct {
			name string
			args []string
		}{
			{
				name: "generate with all flags",
				args: []string{"generate", "--name", "test", "--build-dir", ".", "--out-dir", "/tmp", "--spdx", "--cyclonedx"},
			},
			{
				name: "generate with log level",
				args: []string{"--log-level", "debug", "generate", "--name", "test", "--build-dir", ".", "--out-dir", "/tmp", "--spdx"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// When: Executing with original flag patterns
				rootCmd := createRootCommand()
				buf := new(bytes.Buffer)
				rootCmd.SetOut(buf)
				rootCmd.SetErr(buf)
				rootCmd.SetArgs(tc.args)

				err := rootCmd.Execute()

				// Then: Should work as expected (generate will succeed, merge will fail with not implemented)
				if strings.Contains(tc.name, "generate") {
					assert.NoError(t, err, "Generate command should work with original flags")
				}
			})
		}
	})
}

func TestErrorMessageCompatibility(t *testing.T) {
	t.Run("error messages are clear and actionable", func(t *testing.T) {
		// Given: Invalid command usage
		rootCmd := createRootCommand()
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		// When: Executing with missing required flags
		rootCmd.SetArgs([]string{"generate", "--name", "test"})
		err := rootCmd.Execute()

		// Then: Should provide clear error message
		assert.Error(t, err, "Should return error for missing required flags")
		assert.Contains(t, err.Error(), "directory", "Error should mention missing directory")
	})
}

func TestExitCodeCompatibility(t *testing.T) {
	t.Run("proper exit codes for different scenarios", func(t *testing.T) {
		testCases := []struct {
			name      string
			args      []string
			expectErr bool
		}{
			{
				name:      "help command succeeds",
				args:      []string{"--help"},
				expectErr: false,
			},
			{
				name:      "invalid command fails",
				args:      []string{"invalid-command"},
				expectErr: true,
			},
			{
				name:      "missing required flags fails",
				args:      []string{"generate"},
				expectErr: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// When: Executing command
				rootCmd := createRootCommand()
				buf := new(bytes.Buffer)
				rootCmd.SetOut(buf)
				rootCmd.SetErr(buf)
				rootCmd.SetArgs(tc.args)

				err := rootCmd.Execute()

				// Then: Should have appropriate exit behavior
				if tc.expectErr {
					assert.Error(t, err, "Should return error")
				} else {
					assert.NoError(t, err, "Should not return error")
				}
			})
		}
	})
}
