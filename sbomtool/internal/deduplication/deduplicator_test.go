package deduplication

import (
	"testing"
	"time"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeduplicatePackages_CPEBased(t *testing.T) {
	// GIVEN: Multiple packages with the same CPE
	// WHEN: DeduplicatePackages is called
	// THEN: Packages should be deduplicated using CPE as the key

	packages := []pkg.Package{
		{
			Name:    "glibc",
			Version: "2.31",
			Type:    pkg.RpmPkg,
			CPEs: []cpe.CPE{
				cpe.Must("cpe:2.3:a:gnu:glibc:2.31:*", cpe.GeneratedSource),
			},
		},
		{
			Name:    "glibc",
			Version: "2.31",
			Type:    pkg.RpmPkg,
			CPEs: []cpe.CPE{
				cpe.Must("cpe:2.3:a:gnu:glibc:2.31:*", cpe.GeneratedSource),
			},
		},
	}

	// Set IDs on packages
	for i := range packages {
		packages[i].SetID()
	}

	result := DeduplicatePackages(packages)

	assert.Equal(t, 2, result.Statistics.InputPackages)
	assert.Equal(t, 1, result.Statistics.OutputPackages)
	assert.Equal(t, 1, result.Statistics.DeduplicatedCount)
	assert.Equal(t, 1, result.Statistics.CPEBasedMatches)
	assert.Equal(t, 0, result.Statistics.FallbackMatches)
	assert.Len(t, result.CanonicalPackages, 1)
	// Since both packages are identical, they have the same ID, so only one mapping entry
	assert.Len(t, result.IDMapping, 1)
}

func TestDeduplicatePackages_FallbackStrategy(t *testing.T) {
	// GIVEN: Multiple packages without CPE but same name+version+type
	// WHEN: DeduplicatePackages is called
	// THEN: Packages should be deduplicated using fallback strategy

	packages := []pkg.Package{
		{
			Name:    "mylib",
			Version: "1.0.0",
			Type:    pkg.GoModulePkg,
		},
		{
			Name:    "mylib",
			Version: "1.0.0",
			Type:    pkg.GoModulePkg,
		},
	}

	// Set IDs on packages
	for i := range packages {
		packages[i].SetID()
	}

	result := DeduplicatePackages(packages)

	assert.Equal(t, 2, result.Statistics.InputPackages)
	assert.Equal(t, 1, result.Statistics.OutputPackages)
	assert.Equal(t, 1, result.Statistics.DeduplicatedCount)
	assert.Equal(t, 0, result.Statistics.CPEBasedMatches)
	assert.Equal(t, 1, result.Statistics.FallbackMatches)
}

func TestDeduplicatePackages_MetadataMerging(t *testing.T) {
	// GIVEN: Duplicate packages with different metadata
	// WHEN: DeduplicatePackages is called
	// THEN: All metadata should be merged from all duplicates

	packages := []pkg.Package{
		{
			Name:    "openssl",
			Version: "1.1.1",
			Type:    pkg.RpmPkg,
			CPEs: []cpe.CPE{
				cpe.Must("cpe:2.3:a:openssl:openssl:1.1.1:*", cpe.GeneratedSource),
			},
			Licenses: pkg.NewLicenseSet(
				pkg.NewLicense("MIT"),
			),
		},
		{
			Name:    "openssl",
			Version: "1.1.1",
			Type:    pkg.RpmPkg,
			CPEs: []cpe.CPE{
				cpe.Must("cpe:2.3:a:openssl:openssl:1.1.1:*", cpe.GeneratedSource),
			},
			Licenses: pkg.NewLicenseSet(
				pkg.NewLicense("Apache-2.0"),
			),
		},
	}

	// Set IDs on packages
	for i := range packages {
		packages[i].SetID()
	}

	result := DeduplicatePackages(packages)

	require.Len(t, result.CanonicalPackages, 1)

	var canonical *pkg.Package
	for _, p := range result.CanonicalPackages {
		canonical = p
		break
	}

	assert.Len(t, canonical.Licenses.ToUnorderedSlice(), 2)
	licenseValues := make([]string, 0)
	for _, license := range canonical.Licenses.ToUnorderedSlice() {
		licenseValues = append(licenseValues, license.Value)
	}
	assert.Contains(t, licenseValues, "MIT")
	assert.Contains(t, licenseValues, "Apache-2.0")
}

func TestDeduplicatePackages_NoDeduplication(t *testing.T) {
	// GIVEN: Packages with different names/versions
	// WHEN: DeduplicatePackages is called
	// THEN: No deduplication should occur

	packages := []pkg.Package{
		{
			Name:    "package1",
			Version: "1.0.0",
			Type:    pkg.RpmPkg,
		},
		{
			Name:    "package2",
			Version: "2.0.0",
			Type:    pkg.RpmPkg,
		},
	}

	result := DeduplicatePackages(packages)

	assert.Equal(t, 2, result.Statistics.InputPackages)
	assert.Equal(t, 2, result.Statistics.OutputPackages)
	assert.Equal(t, 0, result.Statistics.DeduplicatedCount)
	assert.Len(t, result.CanonicalPackages, 2)
}

func TestUpdateRelationships(t *testing.T) {
	// GIVEN: Relationships with package IDs that need updating
	// WHEN: UpdateRelationships is called with ID mapping
	// THEN: Relationships should be updated and deduplicated

	// Create mock packages for relationships
	pkg1 := &pkg.Package{Name: "pkg1", Version: "1.0"}
	pkg2 := &pkg.Package{Name: "pkg2", Version: "1.0"}
	pkg3 := &pkg.Package{Name: "pkg3", Version: "1.0"}

	// Set IDs on packages
	pkg1.SetID()
	pkg2.SetID()
	pkg3.SetID()

	relationships := []artifact.Relationship{
		{
			From: pkg1,
			To:   pkg2,
			Type: artifact.DependencyOfRelationship,
		},
		{
			From: pkg1,
			To:   pkg3,
			Type: artifact.DependencyOfRelationship,
		},
	}

	// Create canonical packages
	canonical1 := &pkg.Package{Name: "canonical1", Version: "1.0"}
	canonical2 := &pkg.Package{Name: "canonical2", Version: "1.0"}
	canonical1.SetID()
	canonical2.SetID()

	canonicalPackages := map[string]*pkg.Package{
		string(canonical1.ID()): canonical1,
		string(canonical2.ID()): canonical2,
	}

	idMapping := map[string]string{
		string(pkg1.ID()): string(canonical1.ID()),
		string(pkg2.ID()): string(canonical2.ID()),
	}

	updated := UpdateRelationships(relationships, canonicalPackages, idMapping)

	// The relationships should not be deduplicated since they have different To packages
	assert.Len(t, updated, 2)

	// Verify relationships point to canonical packages
	assert.Equal(t, canonical1, updated[0].From)
	assert.Equal(t, canonical2, updated[0].To)
}

func TestGenerateCanonicalKey_CPEPreferred(t *testing.T) {
	// GIVEN: A package with CPE
	// WHEN: generateCanonicalKey is called
	// THEN: CPE should be used as the key

	p := pkg.Package{
		Name:    "test",
		Version: "1.0",
		Type:    pkg.RpmPkg,
		CPEs: []cpe.CPE{
			cpe.Must("cpe:2.3:a:test:test:1.0:*", cpe.GeneratedSource),
		},
	}

	key := generateCanonicalKey(p)

	assert.Equal(t, "cpe:2.3:a:test:test:1.0:*:*:*:*:*:*:*", key)
	assert.NotContains(t, key, "fallback:")
}

func TestGenerateCanonicalKey_FallbackStrategy(t *testing.T) {
	// GIVEN: A package without CPE
	// WHEN: generateCanonicalKey is called
	// THEN: Fallback strategy should be used

	p := pkg.Package{
		Name:    "test",
		Version: "1.0",
		Type:    pkg.RpmPkg,
	}

	key := generateCanonicalKey(p)

	assert.Equal(t, "test|1.0|rpm", key)
}

func TestMergePackages(t *testing.T) {
	// GIVEN: Multiple packages with different metadata
	// WHEN: mergePackages is called
	// THEN: All metadata should be merged correctly

	pkg1 := &pkg.Package{
		Name:     "test",
		Version:  "1.0",
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:test:test:1.0:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("MIT")),
	}
	pkg2 := &pkg.Package{
		Name:     "test",
		Version:  "1.0",
		CPEs:     []cpe.CPE{cpe.Must("cpe:2.3:a:test:test:1.0:*", cpe.GeneratedSource)},
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("Apache-2.0")),
	}

	packages := []*pkg.Package{pkg1, pkg2}
	merged := mergePackages(packages)

	// Should have both licenses
	assert.Len(t, merged.Licenses.ToUnorderedSlice(), 2)
	licenseValues := make([]string, 0)
	for _, license := range merged.Licenses.ToUnorderedSlice() {
		licenseValues = append(licenseValues, license.Value)
	}
	assert.Contains(t, licenseValues, "MIT")
	assert.Contains(t, licenseValues, "Apache-2.0")

	// Should have CPE (deduplicated)
	assert.Len(t, merged.CPEs, 1)
	assert.Equal(t, "1.0", merged.Version)
}

func TestMergePackages_SkipUnknown(t *testing.T) {
	// GIVEN: Packages with unknown/noassertion values
	// WHEN: mergePackages is called
	// THEN: Unknown values should be skipped in favor of real values

	pkg1 := &pkg.Package{
		Name:     "test",
		Version:  "UNKNOWN",
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("NOASSERTION")),
	}
	pkg2 := &pkg.Package{
		Name:     "test",
		Version:  "1.0",
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("MIT")),
	}

	packages := []*pkg.Package{pkg1, pkg2}
	merged := mergePackages(packages)

	assert.Equal(t, "1.0", merged.Version)
	assert.Len(t, merged.Licenses.ToUnorderedSlice(), 1)
	assert.Equal(t, "MIT", merged.Licenses.ToUnorderedSlice()[0].Value)
}

func TestMergePackages_AllUnknown(t *testing.T) {
	// GIVEN: Packages where all have unknown/noassertion values
	// WHEN: mergePackages is called
	// THEN: Merged package should retain the unknown values

	pkg1 := &pkg.Package{
		Name:     "test",
		Version:  "UNKNOWN",
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("NOASSERTION")),
	}
	pkg2 := &pkg.Package{
		Name:     "test",
		Version:  "unknown",
		Licenses: pkg.NewLicenseSet(pkg.NewLicense("UNKNOWN")),
	}

	packages := []*pkg.Package{pkg1, pkg2}
	merged := mergePackages(packages)

	assert.Equal(t, "UNKNOWN", merged.Version)
	assert.Len(t, merged.Licenses.ToUnorderedSlice(), 1)
	licenseValue := merged.Licenses.ToUnorderedSlice()[0].Value
	assert.True(t, licenseValue == "NOASSERTION" || licenseValue == "UNKNOWN")
}

func TestDeduplicationStatistics(t *testing.T) {
	// GIVEN: A deduplication operation
	// WHEN: DeduplicatePackages completes
	// THEN: Statistics should be accurately recorded

	packages := []pkg.Package{
		{Name: "pkg1", Version: "1.0", Type: pkg.RpmPkg},
		{Name: "pkg1", Version: "1.0", Type: pkg.RpmPkg},
		{Name: "pkg2", Version: "2.0", Type: pkg.RpmPkg},
	}

	// Set IDs on packages
	for i := range packages {
		packages[i].SetID()
	}

	startTime := time.Now()
	result := DeduplicatePackages(packages)
	endTime := time.Now()

	assert.Equal(t, 3, result.Statistics.InputPackages)
	assert.Equal(t, 2, result.Statistics.OutputPackages)
	assert.Equal(t, 1, result.Statistics.DeduplicatedCount)
	assert.True(t, result.Statistics.ProcessingTime > 0)
	assert.True(t, result.Statistics.ProcessingTime < endTime.Sub(startTime)+time.Millisecond)
}

func TestDeduplicationEvents(t *testing.T) {
	// GIVEN: Packages that will be deduplicated
	// WHEN: DeduplicatePackages is called
	// THEN: Statistics should be updated correctly

	packages := []pkg.Package{
		{
			Name:    "test",
			Version: "1.0",
			Type:    pkg.RpmPkg,
			CPEs:    []cpe.CPE{cpe.Must("cpe:2.3:a:test:test:1.0:*", cpe.GeneratedSource)},
		},
		{
			Name:    "test",
			Version: "1.0",
			Type:    pkg.RpmPkg,
			CPEs:    []cpe.CPE{cpe.Must("cpe:2.3:a:test:test:1.0:*", cpe.GeneratedSource)},
		},
	}

	// Set IDs on packages
	for i := range packages {
		packages[i].SetID()
	}

	result := DeduplicatePackages(packages)

	assert.Equal(t, 1, result.Statistics.CPEBasedMatches)
	assert.Equal(t, 0, result.Statistics.FallbackMatches)
}

func TestMergeOverlappingCPEGroups(t *testing.T) {
	// GIVEN: Package groups with overlapping CPEs
	// WHEN: mergeOverlappingCPEGroups is called
	// THEN: Groups with shared CPEs should be merged

	foo := &pkg.Package{
		Name: "foo",
		CPEs: []cpe.CPE{cpe.Must("cpe:2.3:a:vendor:foo:1.0:*", cpe.GeneratedSource)},
	}
	bar := &pkg.Package{
		Name: "bar",
		CPEs: []cpe.CPE{
			cpe.Must("cpe:2.3:a:vendor:foo:1.0:*", cpe.GeneratedSource), // Overlaps with foo
			cpe.Must("cpe:2.3:a:vendor:bar:1.0:*", cpe.GeneratedSource),
		},
	}
	baz := &pkg.Package{
		Name: "baz",
		CPEs: []cpe.CPE{cpe.Must("cpe:2.3:a:vendor:baz:1.0:*", cpe.GeneratedSource)},
	}

	keyToPackages := map[string][]*pkg.Package{
		"group1": {foo},
		"group2": {bar},
		"group3": {baz},
	}

	result := mergeOverlappingCPEGroups(keyToPackages)

	// Should have 2 groups: one merged (foo+bar) and one separate (baz)
	assert.Len(t, result, 2)

	// Find the merged group and separate group
	var mergedGroup, separateGroup []*pkg.Package
	for _, group := range result {
		if len(group) == 2 {
			mergedGroup = group
		} else if len(group) == 1 {
			separateGroup = group
		}
	}

	require.NotNil(t, mergedGroup, "Should have one merged group with 2 packages")
	require.NotNil(t, separateGroup, "Should have one separate group with 1 package")

	// Verify merged group contains foo and bar
	assert.Contains(t, mergedGroup, foo)
	assert.Contains(t, mergedGroup, bar)

	// Verify separate group contains baz
	assert.Contains(t, separateGroup, baz)
}
