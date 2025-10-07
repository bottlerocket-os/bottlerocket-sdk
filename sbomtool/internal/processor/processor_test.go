package processor

import (
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/stretchr/testify/assert"
)

func TestNewBottlerocketSyftProcessor(t *testing.T) {
	// GIVEN: A request to create a new processor
	// WHEN: format.Encoders() and format.Decoders() are called
	// THEN: Encoders and decoders should be available

	decoders := format.Decoders()
	encoders := format.Encoders()

	assert.NotNil(t, decoders)
	assert.NotNil(t, encoders)

	// Verify we have some decoders and encoders available
	assert.Greater(t, len(decoders), 0)
	assert.Greater(t, len(encoders), 0)
}

func TestLoadSBOM_NonexistentFile(t *testing.T) {
	// GIVEN: A nonexistent file path
	// WHEN: LoadSBOM is called
	// THEN: An error should be returned

	nonexistentFile := "/nonexistent/file.json"

	sbom, format, err := LoadSBOM(nonexistentFile)

	assert.Error(t, err)
	assert.Nil(t, sbom)
	assert.Empty(t, format)
	assert.Contains(t, err.Error(), "failed to open SBOM file")
}

func TestSaveSBOM_UnsupportedFormat(t *testing.T) {
	// GIVEN: An SBOM and an unsupported format
	// WHEN: SaveSBOM is called
	// THEN: An error should be returned

	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "output.txt")

	// Create a minimal SBOM
	testSBOM := &sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name:    "test",
			Version: "1.0.0",
		},
		Artifacts: sbom.Artifacts{
			Packages: pkg.NewCollection(),
		},
	}

	err := SaveSBOM(testSBOM, testFile, "unsupported-format")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestSaveSBOM_InvalidPath(t *testing.T) {
	// GIVEN: An SBOM and an invalid file path
	// WHEN: SaveSBOM is called
	// THEN: An error should be returned

	invalidPath := "/nonexistent/directory/output.json"

	// Create a minimal SBOM
	testSBOM := &sbom.SBOM{
		Descriptor: sbom.Descriptor{
			Name:    "test",
			Version: "1.0.0",
		},
		Artifacts: sbom.Artifacts{
			Packages: pkg.NewCollection(),
		},
	}

	// Try to find a valid format
	var validFormat string
	encoders := format.Encoders()
	if len(encoders) > 0 {
		validFormat = string(encoders[0].ID())
	} else {
		validFormat = "syft-json" // fallback
	}

	err := SaveSBOM(testSBOM, invalidPath, validFormat)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write SBOM to file")
}

func TestCountPackagesByType(t *testing.T) {
	// GIVEN: A package collection with various types
	// WHEN: countPackagesByType is called
	// THEN: Correct counts should be returned

	tests := []struct {
		name          string
		packages      []pkg.Package
		targetType    pkg.Type
		expectedCount int
	}{
		{
			name: "count go packages",
			packages: []pkg.Package{
				{
					Type: pkg.GoModulePkg,
					Name: "go-package-1",
				},
				{
					Type: pkg.GoModulePkg,
					Name: "go-package-2",
				},
				{
					Type: pkg.RustPkg,
					Name: "rust-package-1",
				},
			},
			targetType:    pkg.GoModulePkg,
			expectedCount: 2,
		},
		{
			name: "count rust packages",
			packages: []pkg.Package{
				{
					Type: pkg.GoModulePkg,
					Name: "go-package-1",
				},
				{
					Type: pkg.RustPkg,
					Name: "rust-package-1",
				},
				{
					Type: pkg.RustPkg,
					Name: "rust-package-2",
				},
			},
			targetType:    pkg.RustPkg,
			expectedCount: 2,
		},
		{
			name: "count nonexistent type",
			packages: []pkg.Package{
				{
					Type: pkg.GoModulePkg,
					Name: "go-package-1",
				},
			},
			targetType:    pkg.PythonPkg,
			expectedCount: 0,
		},
		{
			name:          "empty collection",
			packages:      []pkg.Package{},
			targetType:    pkg.GoModulePkg,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := pkg.NewCollection(tt.packages...)

			count := countPackagesByType(catalog, tt.targetType)

			assert.Equal(t, tt.expectedCount, count)
		})
	}
}
