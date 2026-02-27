package buildroot

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestScanDirectory(t *testing.T) {
	// GIVEN: Various buildroot directory structures
	// WHEN: Directory scanning is performed
	// THEN: All files should be discovered correctly and file types should be handled appropriately

	tests := []struct {
		name           string
		setupBuildroot func(string) error
		expectedFiles  []string
		expectedTypes  map[string]int // "regular", "symlinks", "directories"
	}{
		{
			name: "basic directory structure",
			setupBuildroot: func(buildroot string) error {
				dirs := []string{"usr/bin", "usr/lib", "etc"}
				for _, dir := range dirs {
					if err := os.MkdirAll(filepath.Join(buildroot, dir), 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					"usr/bin/app":    "binary",
					"usr/lib/lib.so": "library",
					"etc/config":     "config",
				}

				for file, content := range files {
					path := filepath.Join(buildroot, file)
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			expectedFiles: []string{
				"/usr",
				"/usr/bin",
				"/usr/bin/app",
				"/usr/lib",
				"/usr/lib/lib.so",
				"/etc",
				"/etc/config",
			},
			expectedTypes: map[string]int{
				"regular":     3,
				"symlinks":    0,
				"directories": 3, // usr, usr/bin, usr/lib, etc
			},
		},
		{
			name: "symbolic links",
			setupBuildroot: func(buildroot string) error {
				if err := os.MkdirAll(filepath.Join(buildroot, "usr/lib"), 0755); err != nil {
					return err
				}

				// Create target file
				target := filepath.Join(buildroot, "usr/lib/libtest.so.1.0")
				if err := os.WriteFile(target, []byte("library"), 0644); err != nil {
					return err
				}

				// Create symlink
				link := filepath.Join(buildroot, "usr/lib/libtest.so.1")
				return os.Symlink("libtest.so.1.0", link)
			},
			expectedFiles: []string{
				"/usr",
				"/usr/lib",
				"/usr/lib/libtest.so.1.0",
				"/usr/lib/libtest.so.1",
			},
			expectedTypes: map[string]int{
				"regular":     1,
				"symlinks":    1,
				"directories": 2, // usr, usr/lib
			},
		},
		{
			name: "nested directory structure",
			setupBuildroot: func(buildroot string) error {
				dirs := []string{
					"usr/share/doc/myapp",
					"var/log/myapp",
					"opt/myapp/bin",
				}
				for _, dir := range dirs {
					if err := os.MkdirAll(filepath.Join(buildroot, dir), 0755); err != nil {
						return err
					}
				}

				files := map[string]string{
					"usr/share/doc/myapp/README": "readme",
					"var/log/myapp/app.log":      "log",
					"opt/myapp/bin/myapp":        "binary",
				}

				for file, content := range files {
					path := filepath.Join(buildroot, file)
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						return err
					}
				}
				return nil
			},
			expectedFiles: []string{
				"/usr",
				"/usr/share",
				"/usr/share/doc",
				"/usr/share/doc/myapp",
				"/usr/share/doc/myapp/README",
				"/var",
				"/var/log",
				"/var/log/myapp",
				"/var/log/myapp/app.log",
				"/opt",
				"/opt/myapp",
				"/opt/myapp/bin",
				"/opt/myapp/bin/myapp",
			},
			expectedTypes: map[string]int{
				"regular":     3,
				"symlinks":    0,
				"directories": 8, // usr, usr/share, usr/share/doc, usr/share/doc/myapp, var, var/log, var/log/myapp, opt, opt/myapp, opt/myapp/bin
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()
			buildroot := filepath.Join(testDir, "buildroot")

			err := tt.setupBuildroot(buildroot)
			if err != nil {
				t.Fatalf("Failed to setup buildroot: %v", err)
			}

			scanner := NewScanner(10, []string{"*.tmp", "*.bak"})
			result, err := scanner.ScanDirectory(buildroot)
			if err != nil {
				t.Fatalf("ScanDirectory failed: %v", err)
			}

			// Verify normalized paths
			normalizedFiles := result.GetNormalizedPaths()
			if len(normalizedFiles) != len(tt.expectedFiles) {
				t.Errorf("Expected %d normalized files, got %d", len(tt.expectedFiles), len(normalizedFiles))
			}

			// Check that all expected files are present
			normalizedSet := make(map[string]bool)
			for _, file := range normalizedFiles {
				normalizedSet[file] = true
			}

			for _, expectedFile := range tt.expectedFiles {
				if !normalizedSet[expectedFile] {
					t.Errorf("Expected file %s not found in normalized paths", expectedFile)
				}
			}

			// Verify file type counts
			if len(result.RegularFiles) != tt.expectedTypes["regular"] {
				t.Errorf("Expected %d regular files, got %d", tt.expectedTypes["regular"], len(result.RegularFiles))
			}

			if len(result.SymbolicLinks) != tt.expectedTypes["symlinks"] {
				t.Errorf("Expected %d symbolic links, got %d", tt.expectedTypes["symlinks"], len(result.SymbolicLinks))
			}

			// Verify scan duration is recorded
			if result.ScanDuration == 0 {
				t.Error("Expected scan duration to be recorded")
			}

			// Verify total size is calculated
			if result.TotalSize == 0 && len(result.RegularFiles) > 0 {
				t.Error("Expected total size to be calculated for regular files")
			}
		})
	}
}

func TestScanDirectoryWithExclusions(t *testing.T) {
	// GIVEN: A buildroot with files matching exclusion patterns
	// WHEN: Directory scanning is performed with exclusions
	// THEN: Excluded files should not appear in results

	testDir := t.TempDir()
	buildroot := filepath.Join(testDir, "buildroot")

	// Create directory structure
	err := os.MkdirAll(filepath.Join(buildroot, "usr/bin"), 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create files, some matching exclusion patterns
	files := map[string]string{
		"usr/bin/app":        "binary",
		"usr/bin/temp.tmp":   "temporary",
		"usr/bin/debug.log":  "log",
		"usr/bin/config.cfg": "config",
	}

	for file, content := range files {
		path := filepath.Join(buildroot, file)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	scanner := NewScanner(10, []string{"*.tmp", "*.log"})
	result, err := scanner.ScanDirectory(buildroot)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	normalizedFiles := result.GetNormalizedPaths()

	// Should include app and config.cfg, but exclude temp.tmp and debug.log
	expectedIncluded := []string{"/usr", "/usr/bin", "/usr/bin/app", "/usr/bin/config.cfg"}
	excludedPatterns := []string{"/usr/bin/temp.tmp", "/usr/bin/debug.log"}

	normalizedSet := make(map[string]bool)
	for _, file := range normalizedFiles {
		normalizedSet[file] = true
	}

	for _, expectedFile := range expectedIncluded {
		if !normalizedSet[expectedFile] {
			t.Errorf("Expected file %s should be included", expectedFile)
		}
	}

	for _, excludedFile := range excludedPatterns {
		if normalizedSet[excludedFile] {
			t.Errorf("File %s should be excluded", excludedFile)
		}
	}
}

func TestScanDirectoryWithDepthLimit(t *testing.T) {
	// GIVEN: A deeply nested directory structure
	// WHEN: Directory scanning is performed with depth limit
	// THEN: Files beyond the depth limit should not be scanned

	testDir := t.TempDir()
	buildroot := filepath.Join(testDir, "buildroot")

	// Create deeply nested structure
	deepPath := filepath.Join(buildroot, "level1/level2/level3/level4/level5")
	err := os.MkdirAll(deepPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create deep directory: %v", err)
	}

	// Create files at different levels
	files := map[string]string{
		"level1/file1.txt":                             "content1",
		"level1/level2/file2.txt":                      "content2",
		"level1/level2/level3/file3.txt":               "content3",
		"level1/level2/level3/level4/file4.txt":        "content4",
		"level1/level2/level3/level4/level5/file5.txt": "content5",
	}

	for file, content := range files {
		path := filepath.Join(buildroot, file)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	// Scan with depth limit of 3
	scanner := NewScanner(3, []string{})
	result, err := scanner.ScanDirectory(buildroot)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	normalizedFiles := result.GetNormalizedPaths()
	normalizedSet := make(map[string]bool)
	for _, file := range normalizedFiles {
		normalizedSet[file] = true
	}

	// Should include files up to level 3
	expectedIncluded := []string{
		"/level1/file1.txt",
		"/level1/level2/file2.txt",
		"/level1/level2/level3/file3.txt",
	}

	// Should exclude files beyond level 3
	expectedExcluded := []string{
		"/level1/level2/level3/level4/file4.txt",
		"/level1/level2/level3/level4/level5/file5.txt",
	}

	for _, expectedFile := range expectedIncluded {
		if !normalizedSet[expectedFile] {
			t.Errorf("Expected file %s should be included within depth limit", expectedFile)
		}
	}

	for _, excludedFile := range expectedExcluded {
		if normalizedSet[excludedFile] {
			t.Errorf("File %s should be excluded due to depth limit", excludedFile)
		}
	}
}

func TestPathNormalizer(t *testing.T) {
	// GIVEN: Various buildroot file paths
	// WHEN: Path normalization is applied
	// THEN: All paths should be converted to installed paths correctly

	tests := []struct {
		name          string
		buildrootFile string
		buildrootPath string
		expectedPath  string
		expectError   bool
	}{
		{
			name:          "simple file path",
			buildrootFile: "/tmp/buildroot/usr/bin/app",
			buildrootPath: "/tmp/buildroot",
			expectedPath:  "/usr/bin/app",
			expectError:   false,
		},
		{
			name:          "nested file path",
			buildrootFile: "/build/buildroot/usr/share/doc/myapp/README",
			buildrootPath: "/build/buildroot",
			expectedPath:  "/usr/share/doc/myapp/README",
			expectError:   false,
		},
		{
			name:          "root level file",
			buildrootFile: "/tmp/buildroot/config",
			buildrootPath: "/tmp/buildroot",
			expectedPath:  "/config",
			expectError:   false,
		},
		{
			name:          "file not under buildroot",
			buildrootFile: "/other/path/file",
			buildrootPath: "/tmp/buildroot",
			expectedPath:  "",
			expectError:   true,
		},
		{
			name:          "buildroot path without trailing slash",
			buildrootFile: "/tmp/buildroot/usr/bin/app",
			buildrootPath: "/tmp/buildroot",
			expectedPath:  "/usr/bin/app",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildrootToInstalled(tt.buildrootFile, tt.buildrootPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, result)
			}
		})
	}
}

func TestScanDirectoryErrorHandling(t *testing.T) {
	// GIVEN: Various error conditions
	// WHEN: Directory scanning encounters errors
	// THEN: Errors should be handled gracefully

	t.Run("nonexistent directory", func(t *testing.T) {
		scanner := NewScanner(10, []string{})
		_, err := scanner.ScanDirectory("/nonexistent/path")
		if err == nil {
			t.Error("Expected error for nonexistent directory")
		}
	})

	t.Run("permission denied directory", func(t *testing.T) {
		testDir := t.TempDir()
		buildroot := filepath.Join(testDir, "buildroot")
		restrictedDir := filepath.Join(buildroot, "restricted")

		// Create directory structure
		err := os.MkdirAll(restrictedDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Create a file in restricted directory
		restrictedFile := filepath.Join(restrictedDir, "file.txt")
		err = os.WriteFile(restrictedFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// Remove read permission from directory
		err = os.Chmod(restrictedDir, 0000)
		if err != nil {
			t.Fatalf("Failed to change permissions: %v", err)
		}

		// Restore permissions after test
		defer func() {
			if err := os.Chmod(restrictedDir, 0755); err != nil {
				t.Logf("Failed to restore permissions during cleanup: %v", err)
			}
		}()

		scanner := NewScanner(10, []string{})
		result, err := scanner.ScanDirectory(buildroot)

		// Should fail when encountering permission errors
		if err == nil {
			t.Error("Expected error when encountering permission denied directory")
		}

		// Result should be nil when error occurs
		if result != nil {
			t.Error("Expected nil result when scanner encounters permission errors")
		}
	})
}

func TestGetNormalizedPaths(t *testing.T) {
	// GIVEN: A scan result with normalized paths
	// WHEN: GetNormalizedPaths is called
	// THEN: All normalized paths should be returned

	result := &ScanResult{
		NormalizedPaths: map[string]string{
			"/tmp/buildroot/usr/bin/app":    "/usr/bin/app",
			"/tmp/buildroot/usr/lib/lib.so": "/usr/lib/lib.so",
			"/tmp/buildroot/etc/config":     "/etc/config",
		},
		mu: &sync.RWMutex{},
	}

	normalizedPaths := result.GetNormalizedPaths()

	if len(normalizedPaths) != 3 {
		t.Errorf("Expected 3 normalized paths, got %d", len(normalizedPaths))
	}

	expectedPaths := []string{"/usr/bin/app", "/usr/lib/lib.so", "/etc/config"}
	pathSet := make(map[string]bool)
	for _, path := range normalizedPaths {
		pathSet[path] = true
	}

	for _, expectedPath := range expectedPaths {
		if !pathSet[expectedPath] {
			t.Errorf("Expected path %s not found in normalized paths", expectedPath)
		}
	}
}
