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
