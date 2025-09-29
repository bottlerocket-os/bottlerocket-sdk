// Package main provides the command-line interface for sbomtool using Cobra.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/spf13/cobra"

	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/commands/filter"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/commands/generate"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/commands/merge"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/deduplication"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/logging"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/processor"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/validate"
)

// Exit codes
const (
	ExitSuccess = 0
	ExitError   = 1
)

// validationError represents an input validation failure with contextual guidance.
type validationError struct {
	Field      string
	Value      string
	Message    string
	Suggestion string
}

func (e *validationError) Error() string {
	msg := fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
	if e.Suggestion != "" {
		msg += fmt.Sprintf(" (suggestion: %s)", e.Suggestion)
	}
	return msg
}

// fileSystemError represents a file system operation failure with recovery guidance.
type fileSystemError struct {
	Operation  string
	Path       string
	Cause      error
	Suggestion string
}

func (e *fileSystemError) Error() string {
	msg := fmt.Sprintf("%s failed for path '%s': %v", e.Operation, e.Path, e.Cause)
	if e.Suggestion != "" {
		msg += fmt.Sprintf(" (suggestion: %s)", e.Suggestion)
	}
	return msg
}

func (e *fileSystemError) Unwrap() error {
	return e.Cause
}

// createRootCommand creates and configures the root Cobra command for sbomtool.
func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sbomtool",
		Short: "Software Bill of Materials (SBOM) generation and filtering tool",
		Long: `sbomtool generates and filters Software Bill of Materials (SBOM) files for software packages.

It supports SPDX 2.3 and CycloneDX 1.6 formats, can filter SBOMs based on buildroot contents, 
and includes CPE-based package deduplication for both merge and filter operations.`,

		Example: `  # Generate SPDX SBOM for a Go project
  sbomtool generate --name myapp --build-dir ./build --out-dir ./sboms --spdx

  # Generate both SPDX and CycloneDX SBOMs with debug logging
  sbomtool --log-level debug generate --name myapp --build-dir ./build --out-dir ./sboms --spdx --cyclonedx

  # Complete workflow: generate then filter
  sbomtool generate --name myapp --build-dir ./src --out-dir ./temp --spdx
  sbomtool filter --input-sbom ./temp/myapp-spdx.json --filter-by-buildroot ./rootfs --output ./final/myapp-filtered.json`,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logLevel, _ := cmd.Flags().GetString("log-level")
			logging.Configure(logLevel)
			return nil
		},
	}

	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")

	rootCmd.AddCommand(createGenerateCommand())
	rootCmd.AddCommand(createFilterCommand())
	rootCmd.AddCommand(createMergeCommand())

	return rootCmd
}

// createGenerateCommand creates and configures the generate subcommand.
func createGenerateCommand() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate SBOM files from build directories",
		Long: `Generate Software Bill of Materials (SBOM) files by analyzing build directories for components and dependencies.

Supports SPDX 2.3 and CycloneDX 1.6 formats. At least one output format must be specified.
Use --spdx and/or --cyclonedx flags to choose formats.`,

		Example: `  # Basic SPDX generation for a Go project
  sbomtool generate --name web-server --build-dir ./build --out-dir ./sboms --spdx

  # Generate both formats for complete coverage
  sbomtool generate --name api-service --build-dir ./dist --out-dir ./artifacts --spdx --cyclonedx`,

		PreRunE: validateGenerateFlags,
		RunE:    runGenerate,
	}

	generateCmd.Flags().String("name", "", "Package name for SBOM identification (used in output filenames)")
	generateCmd.Flags().String("build-dir", "", "Build directory containing source code, dependencies, and artifacts to analyze")
	generateCmd.Flags().String("out-dir", "", "Output directory for generated SBOM files (created if it doesn't exist)")
	generateCmd.Flags().Bool("spdx", false, "Generate SPDX 2.3 format SBOM (Linux Foundation standard)")
	generateCmd.Flags().Bool("cyclonedx", false, "Generate CycloneDX 1.6 format SBOM (OWASP security-focused standard)")

	if err := generateCmd.MarkFlagRequired("name"); err != nil {
		slog.Error("failed to mark name flag as required", "error", err)
		os.Exit(ExitError)
	}
	if err := generateCmd.MarkFlagRequired("build-dir"); err != nil {
		slog.Error("failed to mark build-dir flag as required", "error", err)
		os.Exit(ExitError)
	}
	if err := generateCmd.MarkFlagRequired("out-dir"); err != nil {
		slog.Error("failed to mark out-dir flag as required", "error", err)
		os.Exit(ExitError)
	}
	generateCmd.MarkFlagsOneRequired("spdx", "cyclonedx")

	return generateCmd
}

// createFilterCommand creates and configures the filter subcommand.
func createFilterCommand() *cobra.Command {
	filterCmd := &cobra.Command{
		Use:   "filter",
		Short: "Filter SBOMs based on buildroot contents",
		Long: `Filter SBOM files to include only components present in buildroot directories.

Takes a SBOM and removes components not found in the specified buildroot.
Preserves original format (SPDX or CycloneDX) and performs CPE-based deduplication.`,

		Example: `  # Filter SPDX SBOM based on package installation
  sbomtool filter --input-sbom package-spdx.json --filter-by-buildroot /opt/myapp --output deployed.json

  # Multi-stage container workflow
  sbomtool generate --name myapp --build-dir ./src --out-dir ./temp --spdx
  sbomtool filter --input-sbom ./temp/myapp-spdx.json --filter-by-buildroot ./final-image --output ./artifacts/final.json`,

		PreRunE: validateFilterFlags,
		RunE:    runFilter,
	}

	filterCmd.Flags().String("input-sbom", "", "Path to SBOM file to filter (SPDX or CycloneDX JSON format)")
	filterCmd.Flags().String("filter-by-buildroot", "", "Path to buildroot directory containing actual deployment files")
	filterCmd.Flags().String("output", "", "Path for filtered SBOM output file (format preserved from input)")

	if err := filterCmd.MarkFlagRequired("input-sbom"); err != nil {
		slog.Error("failed to mark input-sbom flag as required", "error", err)
		os.Exit(ExitError)
	}
	if err := filterCmd.MarkFlagRequired("filter-by-buildroot"); err != nil {
		slog.Error("failed to mark filter-by-buildroot flag as required", "error", err)
		os.Exit(ExitError)
	}
	if err := filterCmd.MarkFlagRequired("output"); err != nil {
		slog.Error("failed to mark output flag as required", "error", err)
		os.Exit(ExitError)
	}

	return filterCmd
}

// validateFilterFlags performs validation of filter command flags.
func validateFilterFlags(cmd *cobra.Command, args []string) error {
	inputSBOM, _ := cmd.Flags().GetString("input-sbom")
	buildrootPath, _ := cmd.Flags().GetString("filter-by-buildroot")
	outputPath, _ := cmd.Flags().GetString("output")

	if err := validate.ValidateFilePath(inputSBOM, "input SBOM", true); err != nil {
		return &fileSystemError{
			Operation:  "input SBOM validation",
			Path:       inputSBOM,
			Cause:      err,
			Suggestion: "Ensure the SBOM file exists and is readable. Generate an SBOM first using 'sbomtool generate' if you don't have one.",
		}
	}

	if err := validate.ValidateSBOMFormat(inputSBOM); err != nil {
		return &validationError{
			Field:      "SBOM format",
			Value:      inputSBOM,
			Message:    err.Error(),
			Suggestion: "Ensure the input file is a valid SPDX 2.3 or CycloneDX 1.6 JSON file. Check the file content and format.",
		}
	}

	if err := validate.ValidateDirectory(buildrootPath, "buildroot", true); err != nil {
		return &fileSystemError{
			Operation:  "buildroot directory validation",
			Path:       buildrootPath,
			Cause:      err,
			Suggestion: "Ensure the buildroot directory exists and contains the actual files you want to filter by. This should be the deployment directory or container rootfs.",
		}
	}

	if err := validate.ValidateOutputPath(outputPath); err != nil {
		return &fileSystemError{
			Operation:  "output path validation",
			Path:       outputPath,
			Cause:      err,
			Suggestion: "Ensure the output directory exists and is writable. Check that you have permissions to create files in the target location.",
		}
	}

	return nil
}

// runFilter executes the SBOM filtering process.
func runFilter(cmd *cobra.Command, args []string) error {
	inputSBOM, _ := cmd.Flags().GetString("input-sbom")
	buildrootPath, _ := cmd.Flags().GetString("filter-by-buildroot")
	outputPath, _ := cmd.Flags().GetString("output")

	slog.Debug("Starting sbomtool filter",
		"input_sbom", inputSBOM,
		"buildroot_path", buildrootPath,
		"output_path", outputPath)

	slog.Info("Starting SBOM filtering process",
		"input_sbom", inputSBOM,
		"buildroot_directory", buildrootPath)

	format, err := validate.DetectSBOMFormat(inputSBOM)
	if err != nil {
		return &fileSystemError{
			Operation:  "SBOM format detection",
			Path:       inputSBOM,
			Cause:      err,
			Suggestion: "Ensure the SBOM file is readable and contains valid JSON. Check file permissions and content.",
		}
	}
	slog.Info("Detected SBOM format", "format", format)

	slog.Info("Validating buildroot directory", "path", buildrootPath)
	if err := validate.ValidateDirectory(buildrootPath, "buildroot", true); err != nil {
		return &fileSystemError{
			Operation:  "buildroot validation",
			Path:       buildrootPath,
			Cause:      err,
			Suggestion: "Ensure the buildroot directory exists and is accessible. Check directory permissions.",
		}
	}

	slog.Info("Filtering SBOM based on buildroot contents")

	// Load SBOM
	sbomData, detectedFormat, err := processor.LoadSBOM(inputSBOM)
	if err != nil {
		return &fileSystemError{
			Operation:  "SBOM loading",
			Path:       inputSBOM,
			Cause:      err,
			Suggestion: "Ensure the input SBOM file is valid and readable.",
		}
	}

	// Filter SBOM
	result, err := filter.FilterSBOMByBuildroot(sbomData, buildrootPath)
	if err != nil {
		return &fileSystemError{
			Operation:  "SBOM filtering",
			Path:       buildrootPath,
			Cause:      err,
			Suggestion: "Ensure the buildroot directory is accessible and the SBOM contains valid package data.",
		}
	}

	// Apply deduplication
	slog.Info("Applying deduplication to filtered packages",
		"filtered_packages", len(result.FilteredSBOM.Artifacts.Packages.Sorted()))

	packages := result.FilteredSBOM.Artifacts.Packages.Sorted()
	dedupResult := deduplication.DeduplicatePackages(packages)

	// Update relationships with deduplicated package IDs
	updatedRelationships := deduplication.UpdateRelationships(result.FilteredSBOM.Relationships, dedupResult.CanonicalPackages, dedupResult.IDMapping)

	// Create new SBOM with deduplicated packages and updated relationships
	canonicalPackages := make([]pkg.Package, 0, len(dedupResult.CanonicalPackages))
	for _, p := range dedupResult.CanonicalPackages {
		canonicalPackages = append(canonicalPackages, *p)
	}

	deduplicatedSBOM := &sbom.SBOM{
		Source:        result.FilteredSBOM.Source,
		Artifacts:     sbom.Artifacts{Packages: pkg.NewCollection(canonicalPackages...)},
		Relationships: updatedRelationships,
		Descriptor:    result.FilteredSBOM.Descriptor,
	}

	slog.Info("Package deduplication completed",
		"input_packages", dedupResult.Statistics.InputPackages,
		"output_packages", dedupResult.Statistics.OutputPackages,
		"deduplicated_count", dedupResult.Statistics.DeduplicatedCount,
		"cpe_based_matches", dedupResult.Statistics.CPEBasedMatches,
		"fallback_matches", dedupResult.Statistics.FallbackMatches)

	// Save filtered SBOM
	if err := processor.SaveSBOM(deduplicatedSBOM, outputPath, detectedFormat); err != nil {
		return &fileSystemError{
			Operation:  "SBOM saving",
			Path:       outputPath,
			Cause:      err,
			Suggestion: "Ensure you have write permissions to the output location.",
		}
	}

	slog.Info("SBOM filtering completed successfully",
		"output_file", outputPath,
		"format", detectedFormat)

	return nil
}

// validateGenerateFlags performs validation of generate command flags.
func validateGenerateFlags(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	buildDir, _ := cmd.Flags().GetString("build-dir")
	outDir, _ := cmd.Flags().GetString("out-dir")
	spdx, _ := cmd.Flags().GetBool("spdx")
	cyclonedx, _ := cmd.Flags().GetBool("cyclonedx")

	if err := validate.ValidatePackageName(name); err != nil {
		return &validationError{
			Field:      "package name",
			Value:      name,
			Message:    err.Error(),
			Suggestion: "Use a descriptive name like 'web-server', 'api-client', or 'data-processor'. Avoid empty strings, special characters, or overly generic names.",
		}
	}

	if err := validate.ValidateDirectory(buildDir, "build", true); err != nil {
		return &fileSystemError{
			Operation:  "build directory validation",
			Path:       buildDir,
			Cause:      err,
			Suggestion: "Ensure the build directory exists and contains the source code, dependencies, or artifacts you want to analyze. Check that you have read permissions.",
		}
	}

	if err := validate.ValidateDirectory(outDir, "output", false); err != nil {
		return &fileSystemError{
			Operation:  "output directory validation",
			Path:       outDir,
			Cause:      err,
			Suggestion: "Ensure the parent directory exists and you have write permissions. The output directory will be created automatically if it doesn't exist.",
		}
	}

	if !spdx && !cyclonedx {
		return &validationError{
			Field:      "output format",
			Value:      "none",
			Message:    "at least one output format must be specified",
			Suggestion: "Add --spdx for SPDX 2.3 format, --cyclonedx for CycloneDX 1.6 format, or both for comprehensive coverage.",
		}
	}

	if err := validate.ValidateOutputDirectoryPermissions(outDir); err != nil {
		return &fileSystemError{
			Operation:  "output directory permission check",
			Path:       outDir,
			Cause:      err,
			Suggestion: "Check that you have write permissions to the output directory. Try creating the directory manually or changing permissions.",
		}
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

	slog.Info("Starting SBOM generation",
		"package_name", name,
		"build_directory", buildDir)

	var formats []string
	if spdx {
		formats = append(formats, "SPDX 2.3")
	}
	if cyclonedx {
		formats = append(formats, "CycloneDX 1.6")
	}
	slog.Info("Generating SBOM formats", "formats", formats)

	success, err := generate.Generate(name, spdx, cyclonedx, buildDir, outDir)
	if err != nil {
		return &fileSystemError{
			Operation:  "SBOM generation",
			Path:       buildDir,
			Cause:      err,
			Suggestion: "Ensure the build directory contains analyzable files (source code, package manifests, binaries). Try running with --log-level debug for detailed analysis information.",
		}
	}
	if !success {
		return &validationError{
			Field:      "SBOM generation result",
			Value:      "no formats generated",
			Message:    "no SBOM formats were successfully generated",
			Suggestion: "Check that the build directory contains recognizable package formats. Supported formats include Go modules, npm packages, Python requirements, and more.",
		}
	}

	slog.Info("SBOM generation completed successfully")

	return nil
}

// createMergeCommand creates and configures the merge subcommand.
func createMergeCommand() *cobra.Command {
	mergeCmd := &cobra.Command{
		Use:   "merge [flags] file1 file2 [file3...]",
		Short: "Merge multiple SBOM files",
		Long: `Merge multiple SBOM files into a single SBOM.

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
		os.Exit(ExitError)
	}
}
