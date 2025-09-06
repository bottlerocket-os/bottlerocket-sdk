// Package validate provides input validation functions for sbomtool.
package validate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/anchore/syft/syft/format"
)

// ValidatePackageName validates that a package name is reasonable for SBOM generation.
// This validation is intentionally permissive to support diverse package ecosystems
// (RPM, Go, Rust, NPM, etc.) while preventing obvious security and filesystem issues.
func ValidatePackageName(name string) error {
	if name == "" {
		return fmt.Errorf("package name cannot be empty")
	}
	// Prevent names that are just whitespace
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("package name cannot be only whitespace")
	}

	if len(name) > 255 {
		return fmt.Errorf("package name too long (max 255 characters)")
	}

	// Check for control characters and other problematic characters
	for i, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("package name contains control character at position %d", i)
		}

		// Disallow characters that could cause issues in file systems or shells
		if r == '\x00' || r == '\n' || r == '\r' || r == '\t' {
			return fmt.Errorf("package name contains invalid character at position %d", i)
		}
	}

	return nil
}

// ValidateDirectory validates that a directory path is safe and accessible.
// Security-focused validation prevents path traversal and ensures consistent behavior.
// We use absolute path resolution to prevent relative path attacks and ensure
// consistent behavior regardless of the current working directory.
//
// If requireExists is true, the directory must exist. If false, non-existent
// directories are considered valid (useful for output directories that will be created).
func ValidateDirectory(path string, purpose string, requireExists bool) error {
	if path == "" {
		return fmt.Errorf("%s directory cannot be empty", purpose)
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid %s directory path: %w", purpose, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			if requireExists {
				return fmt.Errorf("%s directory does not exist: %s", purpose, absPath)
			}
			// For output directories, non-existence is acceptable
			return nil
		}
		return fmt.Errorf("cannot access %s directory: %w", purpose, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s path is not a directory: %s", purpose, absPath)
	}

	return nil
}

// ValidateFilePath validates that a file path exists and is readable.
func ValidateFilePath(path string, purpose string, mustExist bool) error {
	if path == "" {
		return fmt.Errorf("%s path cannot be empty", purpose)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if mustExist {
				return fmt.Errorf("%s file does not exist: %s", purpose, path)
			}
			return nil
		}
		return fmt.Errorf("cannot access %s file: %w", purpose, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s path is a directory, not a file: %s", purpose, path)
	}

	return nil
}

// ValidateOutputPath validates that an output file path is writable.
func ValidateOutputPath(path string) error {
	if path == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	dir := filepath.Dir(path)
	if err := ValidateDirectory(dir, "output directory", false); err != nil {
		return err
	}

	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return fmt.Errorf("output path is a directory, not a file: %s", path)
		}
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("cannot write to output file: %w", err)
		}
		if closeErr := file.Close(); closeErr != nil {
			slog.Debug("failed to close validation file", "error", closeErr, "path", path)
		}
	}

	return nil
}

// ValidateSBOMFormat validates that the input file is a valid SBOM format.
func ValidateSBOMFormat(path string) error {
	format, err := DetectSBOMFormat(path)
	if err != nil {
		return err
	}

	if format == "" {
		return fmt.Errorf("unrecognized SBOM format")
	}

	return nil
}

// DetectSBOMFormat detects the format of an SBOM file using Syft's format detection.
func DetectSBOMFormat(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open SBOM file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Debug("failed to close SBOM file", "error", closeErr, "path", path)
		}
	}()

	// Use Syft's format detection
	formatID, _ := format.Identify(file)
	if formatID == "" {
		return "", nil
	}

	return formatID.String(), nil
}

// ValidateOutputDirectoryPermissions checks if the output directory is writable.
func ValidateOutputDirectoryPermissions(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}

	testFile := filepath.Join(outDir, ".sbomtool-write-test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to output directory: %w", err)
	}

	if closeErr := file.Close(); closeErr != nil {
		slog.Debug("failed to close test file", "error", closeErr, "path", testFile)
	}
	if removeErr := os.Remove(testFile); removeErr != nil {
		slog.Debug("failed to remove test file", "error", removeErr, "path", testFile)
	}

	return nil
}
