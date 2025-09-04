package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/commands/merge"
)

func TestMergeIntegration(t *testing.T) {
	// GIVEN: Two SPDX SBOM files with overlapping packages
	// WHEN: Merge command is executed
	// THEN: Merged SBOM should be created with proper deduplication

	tempDir := t.TempDir()

	// Create test SPDX SBOMs
	sbom1 := createTestSPDXSBOM("app1", []TestPackage{
		{Name: "pkg1", Version: "1.0.0"},
		{Name: "pkg2", Version: "2.0.0"},
	})
	sbom2 := createTestSPDXSBOM("app2", []TestPackage{
		{Name: "pkg1", Version: "1.0.0"}, // Duplicate
		{Name: "pkg3", Version: "3.0.0"},
	})

	// Write test files
	file1 := filepath.Join(tempDir, "sbom1.json")
	file2 := filepath.Join(tempDir, "sbom2.json")
	outputFile := filepath.Join(tempDir, "merged.json")

	require.NoError(t, writeJSONFile(file1, sbom1))
	require.NoError(t, writeJSONFile(file2, sbom2))

	// Execute merge command
	rootCmd := createRootCommand()
	rootCmd.SetArgs([]string{
		"merge",
		"--output", outputFile,
		file1, file2,
	})

	err := rootCmd.Execute()
	require.NoError(t, err, "Merge command should execute successfully")

	// Verify output file exists
	assert.FileExists(t, outputFile, "Merged SBOM file should be created")

	// Test the in-memory SBOM directly (like deduplication tests)
	// Re-run the merge logic to get the in-memory result
	config := merge.MergeConfig{
		Level: 0,
	}
	result, err := merge.Merge(config, []string{file1, file2})
	require.NoError(t, err, "Direct merge should work")

	// Test the in-memory SBOM packages
	packages := make([]pkg.Package, 0)
	for p := range result.MergedSBOM.Artifacts.Packages.Enumerate() {
		packages = append(packages, p)
	}

	// Basic structure validation on in-memory SBOM
	assert.Greater(t, len(packages), 0, "Should have packages after merge")
	assert.Equal(t, 3, len(packages), "Should have exactly 3 packages after deduplication (pkg1, pkg2, pkg3)")

	// Verify specific packages exist
	packageNames := make([]string, len(packages))
	for i, p := range packages {
		packageNames[i] = p.Name
	}
	assert.Contains(t, packageNames, "pkg1")
	assert.Contains(t, packageNames, "pkg2")
	assert.Contains(t, packageNames, "pkg3")
}

func TestMergeIntegrationCycloneDX(t *testing.T) {
	// GIVEN: Two CycloneDX SBOM files with overlapping packages
	// WHEN: Merge command is executed
	// THEN: Merged SBOM should be created with proper deduplication

	tempDir := t.TempDir()

	// Create test CycloneDX SBOMs
	sbom1 := createTestCycloneDXSBOM("app1", []TestPackage{
		{Name: "pkg1", Version: "1.0.0"},
		{Name: "pkg2", Version: "2.0.0"},
	})
	sbom2 := createTestCycloneDXSBOM("app2", []TestPackage{
		{Name: "pkg1", Version: "1.0.0"}, // Duplicate
		{Name: "pkg3", Version: "3.0.0"},
	})

	// Write test files
	file1 := filepath.Join(tempDir, "sbom1.json")
	file2 := filepath.Join(tempDir, "sbom2.json")
	outputFile := filepath.Join(tempDir, "merged.json")

	require.NoError(t, writeJSONFile(file1, sbom1))
	require.NoError(t, writeJSONFile(file2, sbom2))

	// Execute merge command
	rootCmd := createRootCommand()
	rootCmd.SetArgs([]string{
		"merge",
		"--output", outputFile,
		file1, file2,
	})

	err := rootCmd.Execute()
	require.NoError(t, err, "Merge command should execute successfully")

	// Verify output file exists
	assert.FileExists(t, outputFile, "Merged SBOM file should be created")

	// Test the in-memory SBOM directly (like deduplication tests)
	// Re-run the merge logic to get the in-memory result
	config := merge.MergeConfig{
		Level: 0,
	}
	result, err := merge.Merge(config, []string{file1, file2})
	require.NoError(t, err, "Direct merge should work")

	// Test the in-memory SBOM packages
	packages := make([]pkg.Package, 0)
	for p := range result.MergedSBOM.Artifacts.Packages.Enumerate() {
		packages = append(packages, p)
	}

	// Basic structure validation on in-memory SBOM
	assert.Greater(t, len(packages), 0, "Should have packages after merge")
	// CycloneDX includes metadata components (app1, app2) plus deduplicated libraries (pkg1, pkg2, pkg3)
	assert.Equal(t, 5, len(packages), "Should have 5 packages: 2 app components + 3 deduplicated libraries")

	// Verify specific library packages exist (the actual dependencies)
	packageNames := make([]string, len(packages))
	for i, p := range packages {
		packageNames[i] = p.Name
	}
	assert.Contains(t, packageNames, "pkg1")
	assert.Contains(t, packageNames, "pkg2")
	assert.Contains(t, packageNames, "pkg3")
	assert.Contains(t, packageNames, "app1") // Metadata component
	assert.Contains(t, packageNames, "app2") // Metadata component
}

// TestPackage represents a simple package for testing
type TestPackage struct {
	Name    string
	Version string
}

// createTestSPDXSBOM creates a minimal SPDX SBOM for testing
func createTestSPDXSBOM(name string, packages []TestPackage) map[string]interface{} {
	spdxPackages := make([]map[string]interface{}, len(packages))
	for i, pkg := range packages {
		spdxPackages[i] = map[string]interface{}{
			"SPDXID":      "SPDXRef-Package-" + pkg.Name,
			"name":        pkg.Name,
			"versionInfo": pkg.Version,
		}
	}

	return map[string]interface{}{
		"spdxVersion":       "SPDX-2.3",
		"dataLicense":       "CC0-1.0",
		"SPDXID":            "SPDXRef-DOCUMENT",
		"name":              name,
		"documentNamespace": "https://example.com/" + name,
		"packages":          spdxPackages,
	}
}

// createTestCycloneDXSBOM creates a minimal CycloneDX SBOM for testing
func createTestCycloneDXSBOM(name string, packages []TestPackage) map[string]interface{} {
	components := make([]map[string]interface{}, len(packages))
	for i, pkg := range packages {
		components[i] = map[string]interface{}{
			"type":    "library",
			"name":    pkg.Name,
			"version": pkg.Version,
		}
	}

	return map[string]interface{}{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.6",
		"version":     1,
		"metadata": map[string]interface{}{
			"component": map[string]interface{}{
				"type": "application",
				"name": name,
			},
		},
		"components": components,
	}
}

// writeJSONFile writes data as JSON to a file
func writeJSONFile(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0644)
}
