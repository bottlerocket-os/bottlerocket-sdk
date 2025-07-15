// Package main provides the command-line interface for sbomtool using Cobra.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/pkg/generate"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/pkg/logging"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/pkg/merge"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/pkg/validate"
	"github.com/spf13/cobra"
)

// createRootCommand creates and configures the root Cobra command for sbomtool.
func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sbomtool",
		Short: "Software Bill of Materials (SBOM) generation tool",
		Long: `sbomtool is a command-line utility that generates standardized Software Bill of Materials (SBOM) files for software packages.

It analyzes a build directory to identify all components and dependencies, then produces SBOM files in industry-standard formats including SPDX 2.3 and CycloneDX 1.6.`,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logLevel, _ := cmd.Flags().GetString("log-level")
			logging.Configure(logLevel)
			return nil
		},
	}

	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")

	rootCmd.AddCommand(createGenerateCommand())
	rootCmd.AddCommand(createMergeCommand())

	return rootCmd
}

// createGenerateCommand creates and configures the generate subcommand.
func createGenerateCommand() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate SBOM files for a directory",
		Long: `Generate SBOM files for a build directory in specified formats.

The generate command scans the specified build directory to identify all 
components and dependencies, then creates SBOM files in the requested formats 
(SPDX 2.3 and/or CycloneDX 1.6).

At least one output format (--spdx or --cyclonedx) must be specified.`,
		Example: `  sbomtool generate --name mypackage --build-dir ./build --out-dir ./sbom --spdx
  sbomtool generate --name mypackage --build-dir ./build --out-dir ./sbom --cyclonedx
  sbomtool --log-level debug generate --name mypackage --build-dir ./build --out-dir ./sbom --spdx --cyclonedx`,
		PreRunE: validateGenerateFlags,
		RunE:    runGenerate,
	}

	generateCmd.Flags().String("name", "", "Name of the target package")
	generateCmd.Flags().String("build-dir", "", "Target directory of the package")
	generateCmd.Flags().String("out-dir", "", "Output directory for the SBOM")
	generateCmd.Flags().Bool("spdx", false, "Generate an SPDX SBOM")
	generateCmd.Flags().Bool("cyclonedx", false, "Generate a CycloneDX SBOM")

	generateCmd.MarkFlagRequired("name")
	generateCmd.MarkFlagRequired("build-dir")
	generateCmd.MarkFlagRequired("out-dir")
	generateCmd.MarkFlagsOneRequired("spdx", "cyclonedx")

	return generateCmd
}

// validateGenerateFlags performs validation of generate command flags.
func validateGenerateFlags(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	buildDir, _ := cmd.Flags().GetString("build-dir")
	outDir, _ := cmd.Flags().GetString("out-dir")

	if err := validate.ValidatePackageName(name); err != nil {
		return fmt.Errorf("invalid package name: %w", err)
	}

	if err := validate.ValidateDirectory(buildDir, "build", true); err != nil {
		return fmt.Errorf("invalid build directory: %w", err)
	}

	if err := validate.ValidateDirectory(outDir, "output", false); err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}

	return nil
}

// runGenerate executes the SBOM generation process.
func runGenerate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	buildDir, _ := cmd.Flags().GetString("build-dir")
	outDir, _ := cmd.Flags().GetString("out-dir")
	spdx, _ := cmd.Flags().GetBool("spdx")
	cyclonedx, _ := cmd.Flags().GetBool("cyclonedx")

	slog.Debug("Starting sbomtool generate",
		"name", name,
		"build_dir", buildDir,
		"out_dir", outDir,
		"spdx", spdx,
		"cyclonedx", cyclonedx)

	success, err := generate.Generate(name, spdx, cyclonedx, buildDir, outDir)
	if err != nil {
		return fmt.Errorf("SBOM generation failed: %w", err)
	}
	if !success {
		return fmt.Errorf("No SBOM formats were generated")
	}

	return nil
}

// createMergeCommand creates and configures the merge subcommand.
func createMergeCommand() *cobra.Command {
	mergeCmd := &cobra.Command{
		Use:   "merge [flags] file1 file2 [file3...]",
		Short: "Merge multiple SBOM files",
		Long: `Merge multiple SBOM files into a single comprehensive SBOM.

This feature is not yet implemented and will return an error.
Future versions will support merging SBOM files with configurable merge levels.`,
		Args: cobra.MinimumNArgs(2),
		RunE: runMerge,
	}

	mergeCmd.Flags().Int("level", 0, "Merge level")

	return mergeCmd
}

// runMerge executes the SBOM merge process.
//
// Currently returns ErrNotImplemented as the merge functionality is planned for future implementation.
func runMerge(cmd *cobra.Command, args []string) error {
	level, _ := cmd.Flags().GetInt("level")

	slog.Debug("Starting sbomtool merge",
		"level", level,
		"file_count", len(args))

	_, err := merge.Merge(level, args)
	if err != nil {
		return fmt.Errorf("SBOM merge failed: %w", err)
	}

	return nil
}

func main() {
	rootCmd := createRootCommand()
	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}
