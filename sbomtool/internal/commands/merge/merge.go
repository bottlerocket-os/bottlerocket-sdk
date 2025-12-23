// Package merge provides functionality for merging multiple SBOM files.
//
// The merge command combines multiple SBOM files of the same format into a single
// comprehensive SBOM with proper deduplication and relationship preservation.
package merge

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"

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

// loadSBOMs loads all SBOM files and validates format consistency.
func loadSBOMs(inputFiles []string) ([]*sbom.SBOM, string, error) {
	var sboms []*sbom.SBOM
	var commonFormat string

	for i, file := range inputFiles {
		slog.Debug("Loading SBOM file", "file", file, "index", i+1)

		s, format, err := processor.LoadSBOM(file)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load SBOM from %s: %w", file, err)
		}

		if i == 0 {
			commonFormat = format
		} else if format != commonFormat {
			return nil, "", fmt.Errorf("format mismatch: expected %s, got %s in file %s", commonFormat, format, file)
		}

		sboms = append(sboms, s)
	}

	slog.Info("All SBOM files loaded successfully", "count", len(sboms), "format", commonFormat)
	return sboms, commonFormat, nil
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

	var allPackages []pkg.Package
	var allRelationships []artifact.Relationship

	// Collect all packages and relationships, including Source as a package
	for _, s := range sboms {
		sbomPackages := s.Artifacts.Packages.Sorted()
		allPackages = append(allPackages, sbomPackages...)
		allRelationships = append(allRelationships, s.Relationships...)

		// Convert Source (metadata.component) to a package and create contains relationships
		if srcPkg := sourceToPackage(s.Source); srcPkg != nil {
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
		collection.Add(*p)
	}

	return &sbom.SBOM{
		Descriptor: base.Descriptor,
		Artifacts: sbom.Artifacts{
			Packages: collection,
		},
		Relationships: relationships,
	}
}
