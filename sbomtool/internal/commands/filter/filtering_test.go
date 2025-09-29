package filter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterSBOMByBuildroot(t *testing.T) {
	// GIVEN: Various SBOM scenarios with buildroot structures
	// WHEN: FilterSBOMByBuildroot is called
	// THEN: Correct filtering should be performed with transitive dependencies

	tests := []struct {
		name                  string
		setupSBOM             func() *sbom.SBOM
		setupBuildroot        func(string) error
		expectedRootCount     int
		expectedIncludedCount int
		expectedRelationships int
		expectError           bool
	}{
		{
			name: "simple filtering with direct matches",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{
					{id: "pkg-a", name: "package-a", version: "1.0.0", files: []string{"/usr/bin/app-a"}},
					{id: "pkg-b", name: "package-b", version: "2.0.0", files: []string{"/usr/lib/lib-b.so"}},
					{id: "pkg-c", name: "package-c", version: "3.0.0", files: []string{"/opt/app-c"}},
				}, []testRelationship{
					{from: "pkg-a", to: "pkg-b", relType: artifact.DependencyOfRelationship},
				})
			},
			setupBuildroot: func(buildroot string) error {
				return createTestFiles(buildroot, []string{
					"usr/bin/app-a",
					"usr/lib/lib-b.so",
				})
			},
			expectedRootCount:     2, // pkg-a and pkg-b have files in buildroot
			expectedIncludedCount: 2, // Both packages should be included
			expectedRelationships: 1, // One relationship between them
			expectError:           false,
		},
		{
			name: "transitive dependency resolution",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{
					{id: "root", name: "root-app", version: "1.0.0", files: []string{"/usr/bin/root"}},
					{id: "dep1", name: "dependency-1", version: "1.0.0", files: []string{"/usr/lib/dep1.so"}},
					{id: "dep2", name: "dependency-2", version: "1.0.0", files: []string{"/usr/lib/dep2.so"}},
					{id: "unused", name: "unused-pkg", version: "1.0.0", files: []string{"/usr/lib/unused.so"}},
				}, []testRelationship{
					{from: "root", to: "dep1", relType: artifact.DependencyOfRelationship},
					{from: "dep1", to: "dep2", relType: artifact.DependencyOfRelationship},
				})
			},
			setupBuildroot: func(buildroot string) error {
				return createTestFiles(buildroot, []string{
					"usr/bin/root", // Only root package file is in buildroot
				})
			},
			expectedRootCount:     1, // Only root package has files in buildroot
			expectedIncludedCount: 3, // root + dep1 + dep2 (transitive)
			expectedRelationships: 2, // Two dependency relationships
			expectError:           false,
		},
		{
			name: "diamond dependency pattern",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{
					{id: "app", name: "application", version: "1.0.0", files: []string{"/usr/bin/app"}},
					{id: "lib1", name: "library-1", version: "1.0.0", files: []string{"/usr/lib/lib1.so"}},
					{id: "lib2", name: "library-2", version: "1.0.0", files: []string{"/usr/lib/lib2.so"}},
					{id: "common", name: "common-lib", version: "1.0.0", files: []string{"/usr/lib/common.so"}},
				}, []testRelationship{
					{from: "app", to: "lib1", relType: artifact.DependencyOfRelationship},
					{from: "app", to: "lib2", relType: artifact.DependencyOfRelationship},
					{from: "lib1", to: "common", relType: artifact.DependencyOfRelationship},
					{from: "lib2", to: "common", relType: artifact.DependencyOfRelationship},
				})
			},
			setupBuildroot: func(buildroot string) error {
				return createTestFiles(buildroot, []string{
					"usr/bin/app",
				})
			},
			expectedRootCount:     1, // Only app has files in buildroot
			expectedIncludedCount: 4, // All packages should be included
			expectedRelationships: 4, // All relationships should be preserved
			expectError:           false,
		},
		{
			name: "no buildroot matches",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{
					{id: "pkg-a", name: "package-a", version: "1.0.0", files: []string{"/usr/bin/app-a"}},
					{id: "pkg-b", name: "package-b", version: "2.0.0", files: []string{"/usr/lib/lib-b.so"}},
				}, []testRelationship{
					{from: "pkg-a", to: "pkg-b", relType: artifact.DependencyOfRelationship},
				})
			},
			setupBuildroot: func(buildroot string) error {
				return createTestFiles(buildroot, []string{
					"opt/different-app", // No matching files
				})
			},
			expectedRootCount:     0, // No packages have files in buildroot
			expectedIncludedCount: 0, // No packages should be included
			expectedRelationships: 0, // No relationships should be preserved
			expectError:           false,
		},
		{
			name: "invalid buildroot path",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{
					{id: "pkg-a", name: "package-a", version: "1.0.0", files: []string{"/usr/bin/app-a"}},
				}, []testRelationship{})
			},
			setupBuildroot: func(buildroot string) error {
				// Don't create the buildroot directory
				return nil
			},
			expectedRootCount:     0,
			expectedIncludedCount: 0,
			expectedRelationships: 0,
			expectError:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			testDir := t.TempDir()
			buildroot := filepath.Join(testDir, "buildroot")

			if tt.name != "invalid buildroot path" {
				err := os.MkdirAll(buildroot, 0755)
				require.NoError(t, err)
			}

			err := tt.setupBuildroot(buildroot)
			require.NoError(t, err)

			inputSBOM := tt.setupSBOM()

			// Execute filtering
			result, err := FilterSBOMByBuildroot(inputSBOM, buildroot)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.expectedRootCount, len(result.RootPackages))
			assert.Equal(t, tt.expectedIncludedCount, len(result.IncludedPackages))
			assert.Equal(t, tt.expectedRelationships, len(result.FilteredSBOM.Relationships))

			// Verify statistics
			stats := result.Statistics
			assert.Equal(t, tt.expectedIncludedCount, stats.OutputPackages)
			assert.Equal(t, tt.expectedRelationships, stats.OutputRelationships)
			assert.Equal(t, tt.expectedRootCount, stats.RootMatches)
			assert.Equal(t, tt.expectedIncludedCount-tt.expectedRootCount, stats.TransitiveInclusions)
			assert.Greater(t, stats.FilteringDuration, time.Duration(0))

			// Verify SBOM integrity
			assert.NotNil(t, result.FilteredSBOM)
			assert.Equal(t, tt.expectedIncludedCount, result.FilteredSBOM.Artifacts.Packages.PackageCount())
		})
	}
}

func TestFindPackagesWithBuildrootFiles(t *testing.T) {
	// GIVEN: Packages with various file locations and buildroot paths
	// WHEN: findPackagesWithBuildrootFiles is called
	// THEN: Correct packages should be matched based on file coordinates

	tests := []struct {
		name           string
		packages       []testPackage
		buildrootPaths []string
		expectedCount  int
		expectedNames  []string
	}{
		{
			name: "exact path matching",
			packages: []testPackage{
				{id: "match1", name: "matching-pkg-1", files: []string{"/usr/bin/app1"}},
				{id: "match2", name: "matching-pkg-2", files: []string{"/usr/lib/lib2.so"}},
				{id: "nomatch", name: "non-matching-pkg", files: []string{"/opt/other"}},
			},
			buildrootPaths: []string{"/usr/bin/app1", "/usr/lib/lib2.so"},
			expectedCount:  2,
			expectedNames:  []string{"matching-pkg-1", "matching-pkg-2"},
		},
		{
			name: "partial matching",
			packages: []testPackage{
				{id: "match", name: "matching-pkg", files: []string{"/usr/bin/app", "/opt/config"}},
				{id: "nomatch", name: "non-matching-pkg", files: []string{"/var/log/app.log"}},
			},
			buildrootPaths: []string{"/usr/bin/app"},
			expectedCount:  1,
			expectedNames:  []string{"matching-pkg"},
		},
		{
			name: "no matches",
			packages: []testPackage{
				{id: "pkg1", name: "package-1", files: []string{"/usr/bin/app1"}},
				{id: "pkg2", name: "package-2", files: []string{"/usr/lib/lib2.so"}},
			},
			buildrootPaths: []string{"/opt/different"},
			expectedCount:  0,
			expectedNames:  []string{},
		},
		{
			name: "empty buildroot paths",
			packages: []testPackage{
				{id: "pkg1", name: "package-1", files: []string{"/usr/bin/app1"}},
			},
			buildrootPaths: []string{},
			expectedCount:  0,
			expectedNames:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packages := createTestPackageCollection(tt.packages)

			result, err := findPackagesWithBuildrootFiles(packages, tt.buildrootPaths)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result))

			// Verify expected package names
			var actualNames []string
			for _, p := range result {
				actualNames = append(actualNames, p.Name)
			}
			assert.ElementsMatch(t, tt.expectedNames, actualNames)
		})
	}
}

func TestResolveTransitiveDependencies(t *testing.T) {
	// GIVEN: SBOM with complex dependency relationships
	// WHEN: resolveTransitiveDependencies is called
	// THEN: All transitive dependencies should be resolved correctly

	tests := []struct {
		name             string
		packages         []testPackage
		relationships    []testRelationship
		rootPackageNames []string // Use names instead of IDs
		expectedCount    int
		expectedPackages []string
	}{
		{
			name: "linear dependency chain",
			packages: []testPackage{
				{id: "a", name: "pkg-a", version: "1.0.0"},
				{id: "b", name: "pkg-b", version: "1.0.0"},
				{id: "c", name: "pkg-c", version: "1.0.0"},
				{id: "isolated", name: "pkg-isolated", version: "1.0.0"},
			},
			relationships: []testRelationship{
				{from: "a", to: "b", relType: artifact.DependencyOfRelationship},
				{from: "b", to: "c", relType: artifact.DependencyOfRelationship},
			},
			rootPackageNames: []string{"pkg-a"},
			expectedCount:    3,
			expectedPackages: []string{"pkg-a", "pkg-b", "pkg-c"},
		},
		{
			name: "diamond dependency pattern",
			packages: []testPackage{
				{id: "root", name: "root-pkg", version: "1.0.0"},
				{id: "left", name: "left-pkg", version: "1.0.0"},
				{id: "right", name: "right-pkg", version: "1.0.0"},
				{id: "common", name: "common-pkg", version: "1.0.0"},
			},
			relationships: []testRelationship{
				{from: "root", to: "left", relType: artifact.DependencyOfRelationship},
				{from: "root", to: "right", relType: artifact.DependencyOfRelationship},
				{from: "left", to: "common", relType: artifact.DependencyOfRelationship},
				{from: "right", to: "common", relType: artifact.DependencyOfRelationship},
			},
			rootPackageNames: []string{"root-pkg"},
			expectedCount:    4,
			expectedPackages: []string{"root-pkg", "left-pkg", "right-pkg", "common-pkg"},
		},
		{
			name: "multiple root packages",
			packages: []testPackage{
				{id: "root1", name: "root-pkg-1", version: "1.0.0"},
				{id: "root2", name: "root-pkg-2", version: "1.0.0"},
				{id: "shared", name: "shared-pkg", version: "1.0.0"},
				{id: "dep1", name: "dep-pkg-1", version: "1.0.0"},
				{id: "dep2", name: "dep-pkg-2", version: "1.0.0"},
			},
			relationships: []testRelationship{
				{from: "root1", to: "shared", relType: artifact.DependencyOfRelationship},
				{from: "root2", to: "shared", relType: artifact.DependencyOfRelationship},
				{from: "root1", to: "dep1", relType: artifact.DependencyOfRelationship},
				{from: "root2", to: "dep2", relType: artifact.DependencyOfRelationship},
			},
			rootPackageNames: []string{"root-pkg-1", "root-pkg-2"},
			expectedCount:    5,
			expectedPackages: []string{"root-pkg-1", "root-pkg-2", "shared-pkg", "dep-pkg-1", "dep-pkg-2"},
		},
		{
			name: "no dependencies",
			packages: []testPackage{
				{id: "standalone", name: "standalone-pkg", version: "1.0.0"},
			},
			relationships:    []testRelationship{},
			rootPackageNames: []string{"standalone-pkg"},
			expectedCount:    1,
			expectedPackages: []string{"standalone-pkg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputSBOM := createTestSBOM(tt.packages, tt.relationships)

			// Get root packages by name
			var rootPackages []pkg.Package
			for _, p := range inputSBOM.Artifacts.Packages.Sorted() {
				for _, rootName := range tt.rootPackageNames {
					if p.Name == rootName {
						rootPackages = append(rootPackages, p)
						break
					}
				}
			}

			result, err := resolveTransitiveDependencies(inputSBOM, rootPackages)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(result))

			// Verify expected package names
			var actualNames []string
			for _, p := range result {
				actualNames = append(actualNames, p.Name)
			}
			assert.ElementsMatch(t, tt.expectedPackages, actualNames)
		})
	}
}

func TestIsDependencyRelationship(t *testing.T) {
	// GIVEN: Various relationship types
	// WHEN: isDependencyRelationship is called
	// THEN: Correct classification should be returned

	tests := []struct {
		name         string
		relType      artifact.RelationshipType
		isDependency bool
	}{
		{"dependency relationship", artifact.DependencyOfRelationship, true},
		{"contains relationship", artifact.ContainsRelationship, true},
		{"ownership relationship", artifact.OwnershipByFileOverlapRelationship, false},
		{"described by relationship", artifact.DescribedByRelationship, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDependencyRelationship(tt.relType)
			assert.Equal(t, tt.isDependency, result)
		})
	}
}

func TestValidateFilteredSBOM(t *testing.T) {
	// GIVEN: Various SBOM structures
	// WHEN: validateFilteredSBOM is called
	// THEN: Appropriate validation results should be returned

	tests := []struct {
		name        string
		setupSBOM   func() *sbom.SBOM
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid SBOM",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{
					{id: "pkg-a", name: "package-a", version: "1.0.0"},
					{id: "pkg-b", name: "package-b", version: "1.0.0"},
				}, []testRelationship{
					{from: "pkg-a", to: "pkg-b", relType: artifact.DependencyOfRelationship},
				})
			},
			expectError: false,
		},
		{
			name: "empty SBOM",
			setupSBOM: func() *sbom.SBOM {
				return createTestSBOM([]testPackage{}, []testRelationship{})
			},
			expectError: false, // Empty SBOMs are now valid
		},
		{
			name: "relationship references non-existent package",
			setupSBOM: func() *sbom.SBOM {
				s := createTestSBOM([]testPackage{
					{id: "pkg-a", name: "package-a", version: "1.0.0"},
				}, []testRelationship{})

				// Add a relationship that references a non-existent package
				nonExistentPkg := pkg.Package{}
				nonExistentPkg.SetID()

				s.Relationships = append(s.Relationships, artifact.Relationship{
					From: s.Artifacts.Packages.Sorted()[0],
					To:   nonExistentPkg,
					Type: artifact.DependencyOfRelationship,
				})

				return s
			},
			expectError: true,
			errorMsg:    "relationship references non-existent package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSBOM := tt.setupSBOM()
			err := validateFilteredSBOM(testSBOM)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper types and functions for testing

type testPackage struct {
	id      string
	name    string
	version string
	files   []string
}

type testRelationship struct {
	from    string
	to      string
	relType artifact.RelationshipType
}

func createTestSBOM(packages []testPackage, relationships []testRelationship) *sbom.SBOM {
	var pkgs []pkg.Package
	packageMap := make(map[string]pkg.Package)
	idMap := make(map[string]artifact.ID) // Map test IDs to actual package IDs

	// Create packages
	for _, tp := range packages {
		p := pkg.Package{
			Name:    tp.name,
			Version: tp.version,
			Type:    pkg.UnknownPkg,
		}
		p.SetID()

		// Add file locations
		for _, filePath := range tp.files {
			location := file.NewLocation(filePath)
			p.Locations.Add(location)
		}

		pkgs = append(pkgs, p)
		packageMap[tp.id] = p
		idMap[tp.id] = p.ID()
	}

	// Create relationships
	var rels []artifact.Relationship
	for _, tr := range relationships {
		if fromPkg, exists := packageMap[tr.from]; exists {
			if toPkg, exists := packageMap[tr.to]; exists {
				rels = append(rels, artifact.Relationship{
					From: fromPkg,
					To:   toPkg,
					Type: tr.relType,
				})
			}
		}
	}

	return &sbom.SBOM{
		Source: source.Description{
			Name: "test-source",
		},
		Artifacts: sbom.Artifacts{
			Packages: pkg.NewCollection(pkgs...),
		},
		Relationships: rels,
		Descriptor: sbom.Descriptor{
			Name:    "test-sbom",
			Version: "1.0.0",
		},
	}
}

func createTestPackageCollection(packages []testPackage) *pkg.Collection {
	var pkgs []pkg.Package

	for _, tp := range packages {
		p := pkg.Package{
			Name:    tp.name,
			Version: "1.0.0",
			Type:    pkg.UnknownPkg,
		}
		p.SetID()

		// Add file locations
		for _, filePath := range tp.files {
			location := file.NewLocation(filePath)
			p.Locations.Add(location)
		}

		pkgs = append(pkgs, p)
	}

	return pkg.NewCollection(pkgs...)
}

func createTestFiles(buildroot string, files []string) error {
	for _, file := range files {
		fullPath := filepath.Join(buildroot, file)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			return err
		}
	}
	return nil
}
