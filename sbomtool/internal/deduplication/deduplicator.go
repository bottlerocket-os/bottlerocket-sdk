// Package deduplication provides reusable package deduplication logic for SBOM operations.
//
// The deduplication strategy uses CPE (Common Platform Enumeration) as the primary key
// for identifying duplicate packages, with fallback to name+version+type for packages
// without CPE information.
package deduplication

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/pkg"
)

// DeduplicationResult contains the results of package deduplication.
type DeduplicationResult struct {
	CanonicalPackages map[string]*pkg.Package // canonical key -> package
	IDMapping         map[string]string       // old package ID -> canonical package ID
	CPEMapping        map[string]string       // CPE -> canonical package ID
	Statistics        DeduplicationStats
}

// DeduplicationStats provides metrics about the deduplication process.
type DeduplicationStats struct {
	InputPackages        int
	OutputPackages       int
	DeduplicatedCount    int
	CPEBasedMatches      int
	FallbackMatches      int
	RelationshipsUpdated int
	ProcessingTime       time.Duration
}

// DeduplicatePackages performs deduplication on a slice of packages.
func DeduplicatePackages(packages []pkg.Package) *DeduplicationResult {
	startTime := time.Now()

	result := &DeduplicationResult{
		CanonicalPackages: make(map[string]*pkg.Package),
		IDMapping:         make(map[string]string),
		CPEMapping:        make(map[string]string),
		Statistics: DeduplicationStats{
			InputPackages: len(packages),
		},
	}

	keyToPackages := make(map[string][]*pkg.Package)

	for i := range packages {
		p := &packages[i]
		key := generateCanonicalKey(*p)
		keyToPackages[key] = append(keyToPackages[key], p)
	}

	mergedGroups := mergeOverlappingCPEGroups(keyToPackages)

	for key, pkgGroup := range mergedGroups {
		canonical := mergePackages(pkgGroup)

		result.CanonicalPackages[key] = canonical

		for _, p := range pkgGroup {
			result.IDMapping[string(p.ID())] = string(canonical.ID())
		}

		if len(canonical.CPEs) > 0 {
			result.CPEMapping[canonical.CPEs[0].Attributes.String()] = string(canonical.ID())
		}

		if len(pkgGroup) > 1 {
			if len(canonical.CPEs) > 0 {
				result.Statistics.CPEBasedMatches++
			} else {
				result.Statistics.FallbackMatches++
			}
		}
	}

	result.Statistics.OutputPackages = len(result.CanonicalPackages)
	result.Statistics.DeduplicatedCount = result.Statistics.InputPackages - result.Statistics.OutputPackages
	result.Statistics.ProcessingTime = time.Since(startTime)

	slog.Debug("Package deduplication completed",
		"input_packages", result.Statistics.InputPackages,
		"output_packages", result.Statistics.OutputPackages,
		"deduplicated_count", result.Statistics.DeduplicatedCount,
		"cpe_matches", result.Statistics.CPEBasedMatches,
		"fallback_matches", result.Statistics.FallbackMatches,
		"processing_time", result.Statistics.ProcessingTime)

	return result
}

// UpdateRelationships rebuilds relationships using canonical packages from deduplication.
func UpdateRelationships(relationships []artifact.Relationship, canonicalPackages map[string]*pkg.Package, idMapping map[string]string) []artifact.Relationship {
	updated := make([]artifact.Relationship, 0, len(relationships))
	relationshipSet := make(map[string]bool)

	idToCanonical := make(map[string]*pkg.Package)
	for _, canonical := range canonicalPackages {
		idToCanonical[string(canonical.ID())] = canonical
	}

	for _, rel := range relationships {
		newRel := rel

		if canonicalID, exists := idMapping[string(rel.From.ID())]; exists {
			if canonical, found := idToCanonical[canonicalID]; found {
				newRel.From = canonical
			}
		}

		if canonicalID, exists := idMapping[string(rel.To.ID())]; exists {
			if canonical, found := idToCanonical[canonicalID]; found {
				newRel.To = canonical
			}
		}

		relKey := fmt.Sprintf("%s|%s|%s", newRel.From.ID(), newRel.To.ID(), newRel.Type)

		if !relationshipSet[relKey] {
			relationshipSet[relKey] = true
			updated = append(updated, newRel)
		}
	}

	slog.Debug("Relationships updated",
		"input_relationships", len(relationships),
		"output_relationships", len(updated),
		"deduplicated_relationships", len(relationships)-len(updated))

	return updated
}

// generateCanonicalKey creates a unique key for package deduplication.
func generateCanonicalKey(p pkg.Package) string {
	// Use CPE as primary deduplication key when available
	if len(p.CPEs) > 0 {
		return p.CPEs[0].Attributes.String()
	}

	// Fallback to name+version+type for packages without CPE
	name := strings.ToLower(strings.TrimSpace(p.Name))
	version := normalizeVersion(p.Version)
	pkgType := string(p.Type)

	return fmt.Sprintf("%s|%s|%s", name, version, pkgType)
}

// normalizeVersion normalizes version strings for consistent matching.
func normalizeVersion(version string) string {
	if version == "" {
		return "unknown"
	}
	return strings.ToLower(strings.TrimSpace(version))
}

// mergePackages merges multiple packages into a single canonical package.
func mergePackages(packages []*pkg.Package) *pkg.Package {
	if len(packages) == 1 {
		return packages[0]
	}

	// Start with the first package as base
	canonical := *packages[0]
	canonical.SetID() // Generate new ID for merged package

	// Merge CPEs (union, deduplicated)
	cpeSet := make(map[string]cpe.CPE)
	for _, p := range packages {
		for _, c := range p.CPEs {
			cpeSet[c.Attributes.String()] = c
		}
	}
	canonical.CPEs = make([]cpe.CPE, 0, len(cpeSet))
	for _, c := range cpeSet {
		canonical.CPEs = append(canonical.CPEs, c)
	}

	// Merge licenses (union, deduplicated, skip UNKNOWN/NOASSERTION)
	licenseSet := make(map[string]pkg.License)
	for _, p := range packages {
		for _, license := range p.Licenses.ToUnorderedSlice() {
			if !isUnknownOrNoAssertion(license.Value) {
				licenseSet[license.Value] = license
			}
		}
	}
	// If no valid licenses found, keep one UNKNOWN/NOASSERTION if it exists
	if len(licenseSet) == 0 {
		for _, p := range packages {
			for _, license := range p.Licenses.ToUnorderedSlice() {
				if isUnknownOrNoAssertion(license.Value) {
					licenseSet[license.Value] = license
					break
				}
			}
			if len(licenseSet) > 0 {
				break
			}
		}
	}
	licenses := make([]pkg.License, 0, len(licenseSet))
	for _, license := range licenseSet {
		licenses = append(licenses, license)
	}
	canonical.Licenses = pkg.NewLicenseSet(licenses...)

	// Use non-empty version if available, otherwise keep unknown if all are unknown
	foundValidVersion := false
	for _, p := range packages {
		if !isUnknownOrNoAssertion(p.Version) {
			canonical.Version = p.Version
			foundValidVersion = true
			break
		}
	}
	if !foundValidVersion {
		for _, p := range packages {
			if p.Version != "" {
				canonical.Version = p.Version
				break
			}
		}
	}

	return &canonical
}

// isUnknownOrNoAssertion checks if a value represents unknown or no assertion.
func isUnknownOrNoAssertion(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "unknown" || normalized == "noassertion" || normalized == ""
}

// mergeOverlappingCPEGroups merges package groups that have overlapping CPEs
func mergeOverlappingCPEGroups(keyToPackages map[string][]*pkg.Package) map[string][]*pkg.Package {
	groups := make([][]*pkg.Package, 0, len(keyToPackages))
	for _, group := range keyToPackages {
		groups = append(groups, group)
	}

	merged := make([][]*pkg.Package, 0)
	used := make([]bool, len(groups))

	for i, group1 := range groups {
		if used[i] {
			continue
		}

		mergedGroup := make([]*pkg.Package, len(group1))
		copy(mergedGroup, group1)
		used[i] = true

		for j := i + 1; j < len(groups); j++ {
			if used[j] {
				continue
			}

			if hasOverlappingCPEs(mergedGroup, groups[j]) {
				mergedGroup = append(mergedGroup, groups[j]...)
				used[j] = true
			}
		}

		merged = append(merged, mergedGroup)
	}

	result := make(map[string][]*pkg.Package)
	for i, group := range merged {
		key := fmt.Sprintf("merged_group_%d", i)
		result[key] = group
	}

	return result
}

// hasOverlappingCPEs checks if two package groups have any overlapping CPEs
func hasOverlappingCPEs(group1, group2 []*pkg.Package) bool {
	cpes1 := make(map[string]bool)
	for _, pkg := range group1 {
		for _, cpe := range pkg.CPEs {
			cpes1[cpe.Attributes.String()] = true
		}
	}

	for _, pkg := range group2 {
		for _, cpe := range pkg.CPEs {
			if cpes1[cpe.Attributes.String()] {
				return true
			}
		}
	}

	return false
}
