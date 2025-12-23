package merge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
)

func TestMerge(t *testing.T) {
	// GIVEN: Multiple SBOM files with overlapping packages
	// WHEN: Merge is called with valid configuration
	// THEN: SBOMs should be merged with proper deduplication

	tests := []struct {
		name          string
		config        MergeConfig
		inputFiles    []string
		expectedError bool
	}{
		{
			name: "insufficient input files",
			config: MergeConfig{
				Level: 0,
			},
			inputFiles:    []string{"single.json"},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Merge(tt.config, tt.inputFiles)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Statistics.ProcessingTime == 0 {
				t.Error("expected non-zero processing time")
			}

			if result.OutputFormat == "" {
				t.Error("expected output format to be set")
			}
		})
	}
}

func TestValidateInputs(t *testing.T) {
	// GIVEN: Various input configurations
	// WHEN: validateInputs is called
	// THEN: Appropriate validation results should be returned

	// Create temporary files for testing
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.json")
	file2 := filepath.Join(tmpDir, "file2.json")
	singleFile := filepath.Join(tmpDir, "single.json")

	// Create the test files
	for _, file := range []string{file1, file2, singleFile} {
		if err := os.WriteFile(file, []byte(`{"test": "data"}`), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name        string
		config      MergeConfig
		inputFiles  []string
		expectError bool
	}{
		{
			name:        "valid configuration",
			config:      MergeConfig{Level: 0},
			inputFiles:  []string{file1, file2},
			expectError: false,
		},
		{
			name:        "insufficient files",
			config:      MergeConfig{Level: 0},
			inputFiles:  []string{singleFile},
			expectError: true,
		},
		{
			name:        "nonexistent input file",
			config:      MergeConfig{Level: 0},
			inputFiles:  []string{file1, "nonexistent.json"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInputs(tt.config, tt.inputFiles)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMergeSBOMs(t *testing.T) {
	// GIVEN: Multiple SBOMs with packages and relationships
	// WHEN: mergeSBOMs is called
	// THEN: All packages and relationships should be collected

	tests := []struct {
		name                      string
		sboms                     []*sbom.SBOM
		expectedPackageCount      int
		expectedRelationshipCount int
	}{
		{
			name: "multiple SBOMs with packages",
			sboms: []*sbom.SBOM{
				createTestSBOM("test1", []pkg.Package{
					createTestPackage("pkg1", "1.0.0", "cpe:2.3:a:vendor:pkg1:1.0.0:*"),
					createTestPackage("pkg2", "2.0.0", ""),
				}),
				createTestSBOM("test2", []pkg.Package{
					createTestPackage("pkg1", "1.0.0", "cpe:2.3:a:vendor:pkg1:1.0.0:*"), // Duplicate
					createTestPackage("pkg3", "3.0.0", ""),
				}),
			},
			// 4 original packages + 2 source packages (test1, test2) = 6
			// 2 original relationships + 4 contains relationships (2 pkgs × 2 sources) = 6
			expectedPackageCount:      6,
			expectedRelationshipCount: 6,
		},
		{
			name:                      "empty SBOM slice",
			sboms:                     []*sbom.SBOM{},
			expectedPackageCount:      0,
			expectedRelationshipCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, packages, relationships := mergeSBOMs(tt.sboms)

			if merged == nil {
				t.Fatal("expected merged SBOM, got nil")
			}

			if len(packages) != tt.expectedPackageCount {
				t.Errorf("expected %d packages, got %d", tt.expectedPackageCount, len(packages))
			}

			if len(relationships) != tt.expectedRelationshipCount {
				t.Errorf("expected %d relationships, got %d", tt.expectedRelationshipCount, len(relationships))
			}
		})
	}
}

func TestCreateFinalSBOM(t *testing.T) {
	// GIVEN: Base SBOM, canonical packages, and relationships
	// WHEN: createFinalSBOM is called
	// THEN: Final SBOM should be properly constructed

	base := &sbom.SBOM{
		Descriptor: sbom.Descriptor{Name: "test"},
	}

	canonicalPackages := map[string]*pkg.Package{
		"key1": createTestPackagePtr("pkg1", "1.0.0", ""),
		"key2": createTestPackagePtr("pkg2", "2.0.0", ""),
	}

	relationships := []artifact.Relationship{
		{
			From: pkg.Package{Name: "pkg1"},
			To:   pkg.Package{Name: "pkg2"},
			Type: artifact.DependencyOfRelationship,
		},
	}

	final := createFinalSBOM(base, canonicalPackages, relationships)

	if final.Descriptor.Name != "test" {
		t.Errorf("expected descriptor name 'test', got '%s'", final.Descriptor.Name)
	}

	if final.Artifacts.Packages.PackageCount() != 2 {
		t.Errorf("expected 2 packages, got %d", final.Artifacts.Packages.PackageCount())
	}

	if len(final.Relationships) != 1 {
		t.Errorf("expected 1 relationship, got %d", len(final.Relationships))
	}
}

// Helper functions for testing

func createTestSBOM(name string, packages []pkg.Package) *sbom.SBOM {
	collection := pkg.NewCollection()
	for _, p := range packages {
		collection.Add(p)
	}

	return &sbom.SBOM{
		Descriptor: sbom.Descriptor{Name: name},
		Source: source.Description{
			Name: name,
		},
		Artifacts: sbom.Artifacts{
			Packages: collection,
		},
		Relationships: []artifact.Relationship{
			{
				From: packages[0],
				To:   packages[len(packages)-1],
				Type: artifact.DependencyOfRelationship,
			},
		},
	}
}

func createTestPackage(name, version, cpeStr string) pkg.Package {
	p := pkg.Package{
		Name:    name,
		Version: version,
		Type:    pkg.RpmPkg,
		Locations: file.NewLocationSet(
			file.NewLocation("/test/path"),
		),
	}

	if cpeStr != "" {
		c, _ := cpe.New(cpeStr, cpe.GeneratedSource)
		p.CPEs = []cpe.CPE{c}
	}

	p.SetID()
	return p
}

func createTestPackagePtr(name, version, cpeStr string) *pkg.Package {
	p := createTestPackage(name, version, cpeStr)
	return &p
}

func TestSourceToPackage(t *testing.T) {
	// GIVEN: Various source descriptions
	// WHEN: sourceToPackage is called
	// THEN: Appropriate packages should be created

	tests := []struct {
		name       string
		src        source.Description
		expectNil  bool
		expectName string
		expectVer  string
	}{
		{
			name:       "valid source with name and version",
			src:        source.Description{Name: "libelf", Version: "0.194"},
			expectNil:  false,
			expectName: "libelf",
			expectVer:  "0.194",
		},
		{
			name:       "valid source with name only",
			src:        source.Description{Name: "mypackage"},
			expectNil:  false,
			expectName: "mypackage",
			expectVer:  "",
		},
		{
			name:      "empty source",
			src:       source.Description{},
			expectNil: true,
		},
		{
			name:      "source with empty name",
			src:       source.Description{Name: "", Version: "1.0"},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sourceToPackage(tt.src)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got package with name %s", result.Name)
				}
				return
			}

			if result == nil {
				t.Fatal("expected package, got nil")
			}

			if result.Name != tt.expectName {
				t.Errorf("expected name %s, got %s", tt.expectName, result.Name)
			}

			if result.Version != tt.expectVer {
				t.Errorf("expected version %s, got %s", tt.expectVer, result.Version)
			}
		})
	}
}

func TestMergeSBOMsPreservesSource(t *testing.T) {
	// GIVEN: Multiple SBOMs with Source (metadata.component) set
	// WHEN: mergeSBOMs is called
	// THEN: Source from each SBOM should be converted to a package with contains relationships

	sbom1 := createTestSBOM("sbom1", []pkg.Package{
		createTestPackage("pkg1", "1.0.0", ""),
	})
	sbom1.Source = source.Description{Name: "libelf", Version: "0.194"}

	sbom2 := createTestSBOM("sbom2", []pkg.Package{
		createTestPackage("pkg2", "2.0.0", ""),
	})
	sbom2.Source = source.Description{Name: "glibc", Version: "2.38"}

	_, packages, relationships := mergeSBOMs([]*sbom.SBOM{sbom1, sbom2})

	// Should have: pkg1, pkg2, libelf (from source), glibc (from source) = 4 packages
	if len(packages) != 4 {
		t.Errorf("expected 4 packages (2 original + 2 from source), got %d", len(packages))
	}

	// Verify libelf and glibc are in the packages
	foundLibelf := false
	foundGlibc := false
	for _, p := range packages {
		if p.Name == "libelf" && p.Version == "0.194" {
			foundLibelf = true
		}
		if p.Name == "glibc" && p.Version == "2.38" {
			foundGlibc = true
		}
	}

	if !foundLibelf {
		t.Error("expected libelf package from source, not found")
	}
	if !foundGlibc {
		t.Error("expected glibc package from source, not found")
	}

	// Verify contains relationships were created
	// Each SBOM has 1 package + 1 existing relationship from createTestSBOM
	// Plus 1 contains relationship from source to package = 2 contains relationships total
	containsCount := 0
	for _, rel := range relationships {
		if rel.Type == artifact.ContainsRelationship {
			containsCount++
		}
	}
	if containsCount != 2 {
		t.Errorf("expected 2 contains relationships (one per source), got %d", containsCount)
	}
}
