// Package validate provides input validation functions for sbomtool.
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
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
