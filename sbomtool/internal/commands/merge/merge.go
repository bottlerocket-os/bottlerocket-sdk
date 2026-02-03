// Package merge provides functionality for merging multiple SBOM files.
//
// The merge command combines multiple SBOM files of the same format into a single
// comprehensive SBOM with proper deduplication and relationship preservation.
package merge

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"golang.org/x/sync/errgroup"

	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/deduplication"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/processor"
	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/validate"
)

// MergeConfig holds configuration for merge operations.
type MergeConfig struct {
	OutputFormat string // auto-detect from inputs if empty
	Level        int    // merge level (future extensibility)
}

// MergeResult contains the complete results of a merge operation.
type MergeResult struct {
	MergedSBOM          *sbom.SBOM
	DeduplicationResult *deduplication.DeduplicationResult
	OutputFormat        string
	Statistics          MergeStatistics
}

// MergeStatistics provides comprehensive metrics.
type MergeStatistics struct {
	InputSBOMs           int
	TotalInputPackages   int
	OutputPackages       int
	DeduplicatedPackages int
	InputRelationships   int
	OutputRelationships  int
	ProcessingTime       time.Duration
}

// Merge combines multiple SBOM files into a single comprehensive SBOM.
func Merge(config MergeConfig, inputFiles []string) (*MergeResult, error) {
	startTime := time.Now()

	// Phase 1: Validation
	if err := validateInputs(config, inputFiles); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Phase 2: Load
	sboms, format, err := loadSBOMs(inputFiles)
	if err != nil {
		return nil, fmt.Errorf("loading SBOMs failed: %w", err)
	}

	// Phase 3: Merge
	mergedSBOM, allPackages, allRelationships := mergeSBOMs(sboms)

	// Phase 4: Deduplication
	dedupResult := deduplication.DeduplicatePackages(allPackages)

	// Phase 5: Relationship Processing
	updatedRelationships := deduplication.UpdateRelationships(allRelationships, dedupResult.CanonicalPackages, dedupResult.IDMapping)

	// Phase 6: Create Final SBOM
	finalSBOM := createFinalSBOM(mergedSBOM, dedupResult.CanonicalPackages, updatedRelationships)

	outputFormat := config.OutputFormat
	if outputFormat == "" {
		outputFormat = format
	}

	processingTime := time.Since(startTime)
	dedupResult.Statistics.ProcessingTime = processingTime

	return &MergeResult{
		MergedSBOM:          finalSBOM,
		DeduplicationResult: dedupResult,
		OutputFormat:        outputFormat,
		Statistics: MergeStatistics{
			InputSBOMs:           len(inputFiles),
			TotalInputPackages:   len(allPackages),
			OutputPackages:       len(dedupResult.CanonicalPackages),
			DeduplicatedPackages: dedupResult.Statistics.DeduplicatedCount,
			InputRelationships:   len(allRelationships),
			OutputRelationships:  len(updatedRelationships),
			ProcessingTime:       processingTime,
		},
	}, nil
}

// validateInputs validates all input files and configuration.
func validateInputs(config MergeConfig, inputFiles []string) error {
	if len(inputFiles) < 2 {
		return fmt.Errorf("at least 2 input files required, got %d", len(inputFiles))
	}

	// Validate all input files exist and are readable
	for _, file := range inputFiles {
		if err := validate.ValidateFilePath(file, "input SBOM", true); err != nil {
			return fmt.Errorf("input file validation failed: %w", err)
		}
	}

	return nil
}

// loadSBOMs loads all SBOM files in parallel (mixed formats allowed).
// Thread-safety: Syft's LoadSBOM is safe for concurrent use as it only performs
// independent file I/O operations without shared mutable state.
func loadSBOMs(inputFiles []string) ([]*sbom.SBOM, string, error) {
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > len(inputFiles) {
		numWorkers = len(inputFiles)
	}

	sboms := make([]*sbom.SBOM, len(inputFiles))
	formats := make([]string, len(inputFiles))

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(numWorkers)

	for i := range inputFiles {
		i := i
		g.Go(func() error {
			s, format, err := processor.LoadSBOM(inputFiles[i])
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", inputFiles[i], err)
			}
			sboms[i] = s
			formats[i] = format
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, "", err
	}

	// Find first non-empty format for output format detection
	var detectedFormat string
	for _, f := range formats {
		if f != "" {
			detectedFormat = f
			break
		}
	}

	if detectedFormat == "" {
		return nil, "", fmt.Errorf("no valid SBOM format detected from input files")
	}

	slog.Info("All SBOM files loaded successfully", "count", len(sboms))
	return sboms, detectedFormat, nil
}

// sourceToPackage converts an SBOM's Source (metadata.component) to a package.
// This ensures the subject of each input SBOM is preserved in the merged output.
func sourceToPackage(src source.Description) *pkg.Package {
	if src.Name == "" {
		return nil
	}

	p := pkg.Package{
		Name:    src.Name,
		Version: src.Version,
		Type:    pkg.UnknownPkg,
	}
	p.SetID()

	slog.Debug("Converted source to package", "name", src.Name, "version", src.Version)
	return &p
}

// extractPackageFromEndpoint extracts a package from a relationship endpoint.
func extractPackageFromEndpoint(endpoint artifact.Identifiable) (pkg.Package, bool) {
	if endpoint == nil {
		return pkg.Package{}, false
	}
	if p, ok := endpoint.(pkg.Package); ok {
		return p, true
	}
	if p, ok := endpoint.(*pkg.Package); ok && p != nil {
		return *p, true
	}
	return pkg.Package{}, false
}

// mergeSBOMs combines SBOM metadata and extracts all packages and relationships.
// Each input SBOM's Source (metadata.component) is converted to a package to preserve
// the subject of each SBOM in the merged output. Contains relationships are created
// from the source package to all packages in that SBOM.
func mergeSBOMs(sboms []*sbom.SBOM) (*sbom.SBOM, []pkg.Package, []artifact.Relationship) {
	if len(sboms) == 0 {
		return &sbom.SBOM{}, nil, nil
	}

	// Use first SBOM as base for descriptor
	merged := &sbom.SBOM{
		Descriptor: sboms[0].Descriptor,
	}

	// Estimate capacity for pre-allocation
	var totalPkgs, totalRels int
	for _, s := range sboms {
		totalPkgs += s.Artifacts.Packages.PackageCount()
		totalRels += len(s.Relationships)
	}
	totalPkgs += len(sboms) // Account for source packages

	// Track seen package IDs to avoid duplicates from relationship endpoints
	seenIDs := make(map[artifact.ID]struct{}, totalPkgs)
	allPackages := make([]pkg.Package, 0, totalPkgs)
	allRelationships := make([]artifact.Relationship, 0, totalRels)

	// Collect all packages and relationships, including Source as a package
	for _, s := range sboms {
		sbomPackages := s.Artifacts.Packages.Sorted()
		for _, p := range sbomPackages {
			seenIDs[p.ID()] = struct{}{}
			allPackages = append(allPackages, p)
		}
		allRelationships = append(allRelationships, s.Relationships...)

		// Convert Source (metadata.component) to a package and create contains relationships
		if srcPkg := sourceToPackage(s.Source); srcPkg != nil {
			seenIDs[srcPkg.ID()] = struct{}{}
			allPackages = append(allPackages, *srcPkg)

			// Create "contains" relationships from source to each package in this SBOM
			for _, p := range sbomPackages {
				allRelationships = append(allRelationships, artifact.Relationship{
					From: *srcPkg,
					To:   p,
					Type: artifact.ContainsRelationship,
				})
			}
		}
	}

	// Extract packages from relationship endpoints (only if not already seen)
	for _, rel := range allRelationships {
		if p, ok := extractPackageFromEndpoint(rel.From); ok {
			if _, seen := seenIDs[p.ID()]; !seen {
				seenIDs[p.ID()] = struct{}{}
				allPackages = append(allPackages, p)
			}
		}
		if p, ok := extractPackageFromEndpoint(rel.To); ok {
			if _, seen := seenIDs[p.ID()]; !seen {
				seenIDs[p.ID()] = struct{}{}
				allPackages = append(allPackages, p)
			}
		}
	}

	slog.Debug("SBOM merge phase completed",
		"total_packages", len(allPackages),
		"total_relationships", len(allRelationships))

	return merged, allPackages, allRelationships
}

// createFinalSBOM creates the final SBOM with deduplicated packages and relationships.
func createFinalSBOM(base *sbom.SBOM, canonicalPackages map[string]*pkg.Package, relationships []artifact.Relationship) *sbom.SBOM {
	// Create new package collection
	collection := pkg.NewCollection()
	for _, p := range canonicalPackages {
		if p != nil {
			collection.Add(*p)
		}
	}

	// Build map from ID to package pointer in collection
	idToCollectionPkg := make(map[artifact.ID]pkg.Package, len(canonicalPackages))
	for p := range collection.Enumerate() {
		idToCollectionPkg[p.ID()] = p
	}

	// Update relationship endpoints to point to collection packages
	validRelationships := make([]artifact.Relationship, 0, len(relationships))
	for _, rel := range relationships {
		// Nil checks before calling ID()
		if rel.From == nil || rel.To == nil {
			continue
		}

		newRel := rel

		// Update From endpoint
		if fromPkg, ok := idToCollectionPkg[rel.From.ID()]; ok {
			newRel.From = fromPkg
		} else {
			continue // Skip if From not in collection
		}

		// Update To endpoint
		if toPkg, ok := idToCollectionPkg[rel.To.ID()]; ok {
			newRel.To = toPkg
		} else {
			continue // Skip if To not in collection
		}

		validRelationships = append(validRelationships, newRel)
	}

	return &sbom.SBOM{
		Descriptor: base.Descriptor,
		Artifacts: sbom.Artifacts{
			Packages: collection,
		},
		Relationships: validRelationships,
	}
}
