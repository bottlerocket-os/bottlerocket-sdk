// Package deduplication provides reusable package deduplication logic for SBOM operations.
//
// The deduplication strategy uses CPE (Common Platform Enumeration) as the primary key
// for identifying duplicate packages, with fallback to name+version+type for packages
// without CPE information.
package deduplication

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/file"
	"github.com/anchore/syft/syft/pkg"

	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/unionfind"
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

	// Filter out packages with empty names
	var realPackages []pkg.Package
	for _, p := range packages {
		if p.Name != "" {
			realPackages = append(realPackages, p)
		}
	}

	result := &DeduplicationResult{
		CanonicalPackages: make(map[string]*pkg.Package),
		IDMapping:         make(map[string]string),
		CPEMapping:        make(map[string]string),
		Statistics: DeduplicationStats{
			InputPackages: len(realPackages), // Use filtered count
		},
	}

	keyToPackages := make(map[string][]*pkg.Package)

	for i := range realPackages {
		p := &realPackages[i]
		key := generateCanonicalKey(p)
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
	relationshipSet := make(map[string]bool, len(relationships))

	idToCanonical := make(map[string]*pkg.Package, len(canonicalPackages))
	cpeToCanonical := make(map[string]*pkg.Package, len(canonicalPackages))
	nvtToCanonical := make(map[string]*pkg.Package, len(canonicalPackages))
	prefixToCanonical := make(map[string]*pkg.Package, len(canonicalPackages))
	for _, canonical := range canonicalPackages {
		if canonical == nil {
			continue
		}
		idToCanonical[string(canonical.ID())] = canonical
		for _, c := range canonical.CPEs {
			cpeToCanonical[c.Attributes.String()] = canonical
		}
		nvtKey := nvtKeyFromPackage(canonical)
		nvtToCanonical[nvtKey] = canonical
		// Build prefix lookup: SPDX ID without trailing hash
		idStr := string(canonical.ID())
		if idx := strings.LastIndex(idStr, "-"); idx > 0 {
			prefixToCanonical[idStr[:idx]] = canonical
		}
	}

	for _, rel := range relationships {
		if rel.From == nil || rel.To == nil {
			continue
		}
		newRel := rel

		if canonicalID, exists := idMapping[string(rel.From.ID())]; exists {
			if canonical, found := idToCanonical[canonicalID]; found {
				newRel.From = canonical
			}
		} else {
			newRel.From = resolveEndpoint(rel.From, cpeToCanonical, nvtToCanonical, prefixToCanonical)
		}

		if canonicalID, exists := idMapping[string(rel.To.ID())]; exists {
			if canonical, found := idToCanonical[canonicalID]; found {
				newRel.To = canonical
			}
		} else {
			newRel.To = resolveEndpoint(rel.To, cpeToCanonical, nvtToCanonical, prefixToCanonical)
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

// resolveEndpoint attempts to find a canonical package for a relationship endpoint.
func resolveEndpoint(endpoint artifact.Identifiable, cpeToCanonical, nvtToCanonical, prefixToCanonical map[string]*pkg.Package) artifact.Identifiable {
	if endpoint == nil {
		return nil
	}
	var p *pkg.Package
	switch v := endpoint.(type) {
	case *pkg.Package:
		p = v
	case pkg.Package:
		p = &v
	default:
		// Not a pkg.Package - try SPDX ID prefix matching
		idStr := string(endpoint.ID())
		if idx := strings.LastIndex(idStr, "-"); idx > 0 {
			if canonical, found := prefixToCanonical[idStr[:idx]]; found {
				return canonical
			}
		}
		return endpoint
	}

	// Try CPE lookup
	for _, c := range p.CPEs {
		if canonical, found := cpeToCanonical[c.Attributes.String()]; found {
			return canonical
		}
	}

	// Try NVT lookup
	nvtKey := nvtKeyFromPackage(p)
	if canonical, found := nvtToCanonical[nvtKey]; found {
		return canonical
	}

	return endpoint
}

// nvtKeyFromPackage creates a name+version+type key for package lookup.
func nvtKeyFromPackage(p *pkg.Package) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s", strings.ToLower(strings.TrimSpace(p.Name)), normalizeVersion(p.Version), string(p.Type))
}

// generateCanonicalKey creates a unique key for package deduplication.
func generateCanonicalKey(p *pkg.Package) string {
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

	// Merge Locations (union of all file locations)
	allLocations := make([]file.Location, 0)
	for _, p := range packages {
		allLocations = append(allLocations, p.Locations.ToSlice()...)
	}
	canonical.Locations = file.NewLocationSet(allLocations...)

	// Preserve Metadata from first package with non-nil metadata
	if canonical.Metadata == nil {
		for _, p := range packages {
			if p.Metadata != nil {
				canonical.Metadata = p.Metadata
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

// mergeOverlappingCPEGroups merges package groups that share CPEs using Union-Find.
//
// Problem: Packages from different SBOMs may represent the same software but have
// different canonical keys (e.g., different CPE variants). If any CPE appears in
// multiple groups, those groups should be merged.
//
// Solution: Union-Find (disjoint set) provides O(α(n)) amortized operations where
// α is the inverse Ackermann function (effectively constant). This replaces the
// previous O(n²) pairwise comparison approach.
//
// Algorithm:
//  1. Build an inverted index: CPE string -> list of group indices that contain it
//  2. Initialize Union-Find with each group as its own set
//  3. For each CPE that appears in multiple groups, union those groups together
//  4. Collect all packages belonging to the same root into merged groups
func mergeOverlappingCPEGroups(keyToPackages map[string][]*pkg.Package) map[string][]*pkg.Package {
	if len(keyToPackages) == 0 {
		return keyToPackages
	}

	// Convert map to slices for index-based Union-Find operations
	// Sort keys for deterministic results
	keys := make([]string, 0, len(keyToPackages))
	for k := range keyToPackages {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	groups := make([][]*pkg.Package, 0, len(keyToPackages))
	for _, k := range keys {
		groups = append(groups, keyToPackages[k])
	}

	// Step 1: Build inverted index from CPE -> group indices
	cpeToGroups := make(map[string][]int)
	for idx, group := range groups {
		for _, p := range group {
			for _, c := range p.CPEs {
				cpeStr := c.Attributes.String()
				cpeToGroups[cpeStr] = append(cpeToGroups[cpeStr], idx)
			}
		}
	}

	// Step 2: Initialize Union-Find
	uf := unionfind.New(len(groups))

	// Step 3: Union all groups that share any CPE
	for _, indices := range cpeToGroups {
		for i := 1; i < len(indices); i++ {
			uf.Union(indices[0], indices[i], keys)
		}
	}

	// Step 3b: Build inverted index from name+version+type -> group indices
	// This catches packages that should merge but have different/no CPEs
	nvtToGroups := make(map[string][]int)
	for idx, group := range groups {
		for _, p := range group {
			nvtKey := fmt.Sprintf("%s|%s|%s", strings.ToLower(strings.TrimSpace(p.Name)), normalizeVersion(p.Version), string(p.Type))
			nvtToGroups[nvtKey] = append(nvtToGroups[nvtKey], idx)
		}
	}

	// Step 3c: Union all groups that share name+version+type
	for _, indices := range nvtToGroups {
		for i := 1; i < len(indices); i++ {
			uf.Union(indices[0], indices[i], keys)
		}
	}

	// Step 4: Collect packages by their root group
	merged := make(map[int][]*pkg.Package)
	for i, group := range groups {
		root := uf.Find(i)
		merged[root] = append(merged[root], group...)
	}

	// Convert back to map keyed by the root's canonical key
	result := make(map[string][]*pkg.Package)
	for root, pkgs := range merged {
		result[keys[root]] = pkgs
	}
	return result
}
