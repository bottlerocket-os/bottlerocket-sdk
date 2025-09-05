package deduplication

import (
	"testing"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/stretchr/testify/assert"
)

func TestDeduplication_EndToEnd(t *testing.T) {
	// GIVEN: An SBOM with duplicate packages and relationships
	// WHEN: Deduplication is performed
	// THEN: Output SBOM should have merged packages and updated relationships

	// Create a realistic SBOM with duplicates
	inputSBOM := createTestSBOM()

	// Extract packages and relationships
	packages := make([]pkg.Package, 0)
	for p := range inputSBOM.Artifacts.Packages.Enumerate() {
		packages = append(packages, p)
	}
	relationships := inputSBOM.Relationships

	// Perform deduplication
	result := DeduplicatePackages(packages)
	updatedRelationships := UpdateRelationships(relationships, result.CanonicalPackages, result.IDMapping)

	// Create output SBOM
	outputSBOM := createOutputSBOM(result.CanonicalPackages, updatedRelationships, inputSBOM)

	// Verify deduplication results
	assert.Less(t, len(result.CanonicalPackages), len(packages), "Should have fewer packages after deduplication")
	assert.Greater(t, result.Statistics.CPEBasedMatches+result.Statistics.FallbackMatches, 0, "Should have some matches")

	// Verify output SBOM structure
	outputPackages := make([]pkg.Package, 0)
	for p := range outputSBOM.Artifacts.Packages.Enumerate() {
		outputPackages = append(outputPackages, p)
	}

	assert.Equal(t, len(result.CanonicalPackages), len(outputPackages), "Output SBOM should have canonical packages")
	assert.Len(t, outputSBOM.Relationships, len(updatedRelationships), "Output SBOM should have updated relationships")

	// Verify specific deduplication scenarios
	assert.Equal(t, 4, len(outputPackages), "Should have 4 unique packages (2 openssl merged, 2 curl merged, 1 openssl-dev, 1 unique)")
	assert.Equal(t, 2, result.Statistics.CPEBasedMatches, "Should have 2 CPE-based matches")
	assert.Equal(t, 0, result.Statistics.FallbackMatches, "Should have 0 fallback matches in this test")

	// Verify no duplicate packages remain for same name+version+type
	packageKeys := make(map[string]int)
	for _, p := range outputPackages {
		key := p.Name + "|" + p.Version + "|" + string(p.Type)
		packageKeys[key]++
	}

	for name, count := range packageKeys {
		assert.Equal(t, 1, count, "Package %s should appear only once", name)
	}

	// Verify relationships point to canonical packages
	canonicalIDs := make(map[string]bool)
	for _, canonical := range result.CanonicalPackages {
		canonicalIDs[string(canonical.ID())] = true
	}

	for _, rel := range outputSBOM.Relationships {
		if pkg, ok := rel.From.(*pkg.Package); ok {
			assert.True(t, canonicalIDs[string(pkg.ID())], "Relationship From should point to canonical package")
		}
		if pkg, ok := rel.To.(*pkg.Package); ok {
			assert.True(t, canonicalIDs[string(pkg.ID())], "Relationship To should point to canonical package")
		}
	}
}

// createTestSBOM creates a realistic SBOM with duplicate packages for testing
func createTestSBOM() *sbom.SBOM {
	// Create packages with intentional duplicates
	pkg1 := pkg.Package{
		Name:     "openssl",
		Version:  "1.1.1",
		Type:     pkg.RpmPkg,
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:openssl:openssl:1.1.1:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("OpenSSL")),
	}
	pkg1.SetID()

	// Duplicate of pkg1 with same CPE but different license info
	pkg2 := pkg.Package{
		Name:     "openssl",
		Version:  "1.1.1",
		Type:     pkg.RpmPkg,
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:openssl:openssl:1.1.1:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("Apache-2.0")),
	}
	pkg2.SetID()

	// Similar but different package - should NOT match (different version)
	pkg3 := pkg.Package{
		Name:     "openssl-dev",
		Version:  "1.1.1",
		Type:     pkg.RpmPkg,
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:openssl:openssl-dev:1.1.1:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("OpenSSL")),
	}
	pkg3.SetID()

	// Another package
	pkg4 := pkg.Package{
		Name:     "curl",
		Version:  "7.68.0",
		Type:     pkg.RpmPkg,
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:curl:curl:7.68.0:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("MIT")),
	}
	pkg4.SetID()

	// Duplicate of pkg4 with same CPE but different license (should match)
	pkg5 := pkg.Package{
		Name:     "curl",
		Version:  "7.68.0",
		Type:     pkg.RpmPkg,
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:curl:curl:7.68.0:*", cpe.GeneratedSource)}, // Same CPE as pkg4
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("BSD-3-Clause")),
	}
	pkg5.SetID()

	// Similar but different package - should NOT match (different version)
	pkg6 := pkg.Package{
		Name:     "curl",
		Version:  "7.69.0", // Different version
		Type:     pkg.RpmPkg,
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:curl:curl:7.69.0:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("MIT")),
	}
	pkg6.SetID()

	// Create package collection
	packages := pkg.NewCollection(pkg1, pkg2, pkg3, pkg4, pkg5, pkg6)

	// Create relationships
	relationships := []artifact.Relationship{
		{
			From: &pkg1,
			To:   &pkg4,
			Type: artifact.DependencyOfRelationship,
		},
		{
			From: &pkg2, // Duplicate package
			To:   &pkg5, // Another duplicate
			Type: artifact.DependencyOfRelationship,
		},
		{
			From: &pkg3, // Should remain separate
			To:   &pkg6, // Should remain separate
			Type: artifact.DependencyOfRelationship,
		},
	}

	return &sbom.SBOM{
		Artifacts: sbom.Artifacts{
			Packages: packages,
		},
		Relationships: relationships,
		Descriptor: sbom.Descriptor{
			Name:    "test-sbom",
			Version: "1.0.0",
		},
	}
}

// createOutputSBOM creates an SBOM from deduplicated packages and relationships
func createOutputSBOM(canonicalPackages map[string]*pkg.Package, relationships []artifact.Relationship, originalSBOM *sbom.SBOM) *sbom.SBOM {
	// Convert canonical packages to slice
	packages := make([]pkg.Package, 0, len(canonicalPackages))
	for _, p := range canonicalPackages {
		packages = append(packages, *p)
	}

	// Create new package collection
	packageCollection := pkg.NewCollection(packages...)

	return &sbom.SBOM{
		Artifacts: sbom.Artifacts{
			Packages: packageCollection,
		},
		Relationships: relationships,
		Descriptor:    originalSBOM.Descriptor,
	}
}

func TestDeduplication_FallbackMatching(t *testing.T) {
	// GIVEN: Packages without CPEs that should match by name+version+type
	// WHEN: Deduplication is performed
	// THEN: Packages should be deduplicated using fallback matching

	pkg1 := pkg.Package{
		Name:     "libssl",
		Version:  "1.1.1",
		Type:     pkg.RpmPkg,
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("OpenSSL")),
	}
	pkg1.SetID()

	pkg2 := pkg.Package{
		Name:     "libssl",
		Version:  "1.1.1",
		Type:     pkg.RpmPkg,
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("Apache-2.0")),
	}
	pkg2.SetID()

	packages := []pkg.Package{pkg1, pkg2}
	result := DeduplicatePackages(packages)

	assert.Equal(t, 1, len(result.CanonicalPackages), "Should have 1 canonical package")
	assert.Equal(t, 0, result.Statistics.CPEBasedMatches, "Should have 0 CPE-based matches")
	assert.Equal(t, 1, result.Statistics.FallbackMatches, "Should have 1 fallback match")

	// Verify merged licenses
	var canonical *pkg.Package
	for _, p := range result.CanonicalPackages {
		canonical = p
		break
	}

	licenses := canonical.Licenses.ToUnorderedSlice()
	assert.Len(t, licenses, 2, "Should have both licenses merged")

	licenseValues := make([]string, len(licenses))
	for i, license := range licenses {
		licenseValues[i] = license.Value
	}
	assert.Contains(t, licenseValues, "OpenSSL")
	assert.Contains(t, licenseValues, "Apache-2.0")
}

func TestDeduplication_OverlappingCPEs(t *testing.T) {
	// GIVEN: Packages with overlapping CPE lists
	// WHEN: Deduplication is performed
	// THEN: Packages should be merged with union of all CPEs

	pkg1 := pkg.Package{
		Name:    "openssl",
		Version: "1.1.1",
		Type:    pkg.RpmPkg,
		CPEs: []cpe.CPE{
			cpe.Must("cpe:2.3:a:openssl:openssl:1.1.1:*", cpe.GeneratedSource),
			cpe.Must("cpe:2.3:a:openssl_project:openssl:1.1.1:*", cpe.GeneratedSource),
		},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("OpenSSL")),
	}
	pkg1.SetID()

	pkg2 := pkg.Package{
		Name:    "openssl-lib",
		Version: "1.1.1",
		Type:    pkg.RpmPkg,
		CPEs: []cpe.CPE{
			cpe.Must("cpe:2.3:a:openssl_project:openssl:1.1.1:*", cpe.GeneratedSource), // Overlaps with pkg1
			cpe.Must("cpe:2.3:a:openssl:libssl:1.1.1:*", cpe.GeneratedSource),
		},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("Apache-2.0")),
	}
	pkg2.SetID()

	packages := []pkg.Package{pkg1, pkg2}
	result := DeduplicatePackages(packages)

	// Should have 1 canonical package (merged)
	assert.Equal(t, 1, len(result.CanonicalPackages), "Should have 1 canonical package")
	assert.Equal(t, 1, result.Statistics.CPEBasedMatches, "Should have 1 CPE-based match")

	// Get the merged package
	var merged *pkg.Package
	for _, canonical := range result.CanonicalPackages {
		merged = canonical
		break
	}

	// Verify union of CPEs
	assert.Len(t, merged.CPEs, 3, "Should have all 3 unique CPEs")

	cpeStrings := make([]string, len(merged.CPEs))
	for i, cpe := range merged.CPEs {
		cpeStrings[i] = cpe.Attributes.String()
	}

	assert.Contains(t, cpeStrings, "cpe:2.3:a:openssl:openssl:1.1.1:*:*:*:*:*:*:*")
	assert.Contains(t, cpeStrings, "cpe:2.3:a:openssl_project:openssl:1.1.1:*:*:*:*:*:*:*")
	assert.Contains(t, cpeStrings, "cpe:2.3:a:openssl:libssl:1.1.1:*:*:*:*:*:*:*")

	// Should have both licenses
	licenses := merged.Licenses.ToUnorderedSlice()
	assert.Len(t, licenses, 2, "Should have both licenses")
}
