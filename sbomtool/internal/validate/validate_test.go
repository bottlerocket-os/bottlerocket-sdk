package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// TestValidatePackageName verifies that package name validation correctly handles various input formats.
//
// Given: Package names from different ecosystems and edge cases
// When: ValidatePackageName is called with each test case
// Then: Validation should pass for legitimate names and fail for problematic ones
func TestValidatePackageName(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		wantErr     bool
		errContains string
	}{
		// Valid package names
		{"valid simple name", "mypackage", false, ""},
		{"valid with hyphens", "my-package", false, ""},
		{"valid with underscores", "my_package", false, ""},
		{"valid with dots", "my.package", false, ""},
		{"valid go module", "github.com/user/repo", false, ""},
		{"valid npm scoped", "@types/node", false, ""},
		{"valid rpm package", "glibc-devel", false, ""},
		{"valid rust crate", "serde_json", false, ""},
		{"valid with numbers", "python3.9", false, ""},
		{"valid complex", "gcc-9-base", false, ""},

		// Invalid package names
		{"empty name", "", true, "cannot be empty"},
		{"only whitespace", "   ", true, "cannot be only whitespace"},
		{"too long", strings.Repeat("a", 256), true, "too long"},
		{"with null byte", "test\x00name", true, "control character"},
		{"with newline", "test\nname", true, "control character"},
		{"with carriage return", "test\rname", true, "control character"},
		{"with tab", "test\tname", true, "control character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePackageName(tt.packageName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePackageName() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePackageName() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePackageName() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidatePackageNameControlCharacters verifies that control characters are properly rejected.
//
// Given: Package names containing various control characters
// When: ValidatePackageName is called with each control character
// Then: All control characters should be rejected with appropriate errors
func TestValidatePackageNameControlCharacters(t *testing.T) {
	for i := 0; i < 32; i++ {
		if i == 9 || i == 10 || i == 13 { // tab, newline, carriage return - already tested above
			continue
		}

		char := rune(i)
		if unicode.IsControl(char) {
			packageName := "test" + string(char) + "name"
			err := ValidatePackageName(packageName)
			if err == nil {
				t.Errorf("ValidatePackageName() with control character %d should have failed", i)
			}
		}
	}
}

// TestValidateDirectory verifies directory path validation and security checks.
//
// Given: Various directory paths including valid, invalid, and non-existent paths
// When: ValidateDirectory is called with different path types and existence requirements
// Then: Should accept valid directories based on existence requirements and reject files or invalid paths
func TestValidateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Path that doesn't exist
	nonexistentDir := filepath.Join(tempDir, "newdir")

	tests := []struct {
		name          string
		path          string
		purpose       string
		requireExists bool
		wantErr       bool
		errContains   string
	}{
		// Valid cases - existing directory
		{"existing directory with requireExists=true", tempDir, "test", true, false, ""},
		{"existing directory with requireExists=false", tempDir, "test", false, false, ""},

		// Valid cases - non-existent directory (only when requireExists=false)
		{"nonexistent directory with requireExists=false", nonexistentDir, "output", false, false, ""},

		// Invalid cases - empty path
		{"empty path with requireExists=true", "", "test", true, true, "cannot be empty"},
		{"empty path with requireExists=false", "", "output", false, true, "cannot be empty"},

		// Invalid cases - non-existent directory when required to exist
		{"nonexistent directory with requireExists=true", nonexistentDir, "build", true, true, "build directory does not exist"},

		// Invalid cases - file instead of directory
		{"file instead of directory with requireExists=true", testFile, "test", true, true, "test path is not a directory"},
		{"file instead of directory with requireExists=false", testFile, "output", false, true, "output path is not a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDirectory(tt.path, tt.purpose, tt.requireExists)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDirectory() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateDirectory() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDirectory() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateFilePath verifies file path validation and existence checks.
//
// Given: Various file paths including valid files, directories, and non-existent paths
// When: ValidateFilePath is called with different existence requirements
// Then: Should validate files correctly based on existence requirements
func TestValidateFilePath(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	nonexistentFile := filepath.Join(tempDir, "missing.json")

	tests := []struct {
		name        string
		path        string
		purpose     string
		mustExist   bool
		wantErr     bool
		errContains string
	}{
		{"existing file with mustExist=true", testFile, "input", true, false, ""},
		{"existing file with mustExist=false", testFile, "input", false, false, ""},
		{"nonexistent file with mustExist=false", nonexistentFile, "output", false, false, ""},
		{"nonexistent file with mustExist=true", nonexistentFile, "input", true, true, "does not exist"},
		{"empty path", "", "input", true, true, "cannot be empty"},
		{"directory instead of file", tempDir, "input", true, true, "is a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path, tt.purpose, tt.mustExist)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateFilePath() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateFilePath() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateFilePath() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateOutputPath verifies output path validation and writability checks.
//
// Given: Various output paths including valid paths and invalid scenarios
// When: ValidateOutputPath is called
// Then: Should validate output paths and check directory writability
func TestValidateOutputPath(t *testing.T) {
	tempDir := t.TempDir()

	validOutputPath := filepath.Join(tempDir, "output.json")
	existingFile := filepath.Join(tempDir, "existing.json")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{"valid new output path", validOutputPath, false, ""},
		{"valid existing file path", existingFile, false, ""},
		{"empty path", "", true, "cannot be empty"},
		{"directory as output path", tempDir, true, "is a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputPath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateOutputPath() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateOutputPath() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateOutputPath() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestDetectSBOMFormat verifies SBOM format detection from file content.
//
// Given: Files with different SBOM formats and invalid content
// When: DetectSBOMFormat is called
// Then: Should correctly identify SPDX, CycloneDX, or return empty for unknown formats
func TestDetectSBOMFormat(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files with different formats
	spdxFile := filepath.Join(tempDir, "spdx.json")
	spdxContent := `{"spdxVersion": "SPDX-2.3", "SPDXID": "SPDXRef-DOCUMENT"}`
	if err := os.WriteFile(spdxFile, []byte(spdxContent), 0644); err != nil {
		t.Fatalf("Failed to create SPDX test file: %v", err)
	}

	cyclonedxFile := filepath.Join(tempDir, "cyclonedx.json")
	cyclonedxContent := `{"bomFormat": "CycloneDX", "specVersion": "1.6"}`
	if err := os.WriteFile(cyclonedxFile, []byte(cyclonedxContent), 0644); err != nil {
		t.Fatalf("Failed to create CycloneDX test file: %v", err)
	}

	unknownFile := filepath.Join(tempDir, "unknown.json")
	unknownContent := `{"format": "unknown", "version": "1.0"}`
	if err := os.WriteFile(unknownFile, []byte(unknownContent), 0644); err != nil {
		t.Fatalf("Failed to create unknown format test file: %v", err)
	}

	nonexistentFile := filepath.Join(tempDir, "missing.json")

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{"SPDX format", spdxFile, "spdx-json", false},
		{"CycloneDX format", cyclonedxFile, "cyclonedx-json", false},
		{"unknown format", unknownFile, "", false},
		{"nonexistent file", nonexistentFile, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DetectSBOMFormat(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("DetectSBOMFormat() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("DetectSBOMFormat() unexpected error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("DetectSBOMFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestValidateSBOMFormat verifies SBOM format validation.
//
// Given: Files with valid and invalid SBOM formats
// When: ValidateSBOMFormat is called
// Then: Should pass for valid SBOM formats and fail for invalid ones
func TestValidateSBOMFormat(t *testing.T) {
	tempDir := t.TempDir()

	spdxFile := filepath.Join(tempDir, "valid-spdx.json")
	spdxContent := `{"spdxVersion": "SPDX-2.3"}`
	if err := os.WriteFile(spdxFile, []byte(spdxContent), 0644); err != nil {
		t.Fatalf("Failed to create SPDX test file: %v", err)
	}

	invalidFile := filepath.Join(tempDir, "invalid.json")
	invalidContent := `{"not": "an sbom"}`
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid SPDX format", spdxFile, false},
		{"invalid format", invalidFile, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSBOMFormat(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSBOMFormat() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSBOMFormat() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateOutputDirectoryPermissions verifies output directory permission checks.
//
// Given: Various directory scenarios including writable and non-writable directories
// When: ValidateOutputDirectoryPermissions is called
// Then: Should create directories if needed and verify write permissions
func TestValidateOutputDirectoryPermissions(t *testing.T) {
	tempDir := t.TempDir()

	existingDir := filepath.Join(tempDir, "existing")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatalf("Failed to create existing directory: %v", err)
	}

	newDir := filepath.Join(tempDir, "new")

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"existing writable directory", existingDir, false},
		{"new directory to create", newDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputDirectoryPermissions(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateOutputDirectoryPermissions() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateOutputDirectoryPermissions() unexpected error = %v", err)
				}
			}
		})
	}
}
