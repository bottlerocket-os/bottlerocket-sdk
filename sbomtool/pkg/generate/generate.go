package generate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/format/cyclonedxjson"
	"github.com/anchore/syft/syft/format/spdxjson"
	"github.com/anchore/syft/syft/sbom"
)

// closeFile safely closes a file handle and logs any close errors without returning them.
// It logs close errors to stderr but doesn't return them to avoid
// overriding more important errors from the main operation.
func closeFile(f *os.File, operation string) {
	if closeErr := f.Close(); closeErr != nil {
		slog.Error("Failed to close file", "operation", operation, "error", closeErr)
	}
}

// createSpdxSbom generates an SPDX 2.3 format SBOM file from the provided SBOM data.
func createSpdxSbom(sbom sbom.SBOM, name string, outDir string) error {
	cfg := spdxjson.DefaultEncoderConfig()
	cfg.Pretty = false
	cfg.Version = "2.3"

	encoder, err := spdxjson.NewFormatEncoderWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create SPDX encoder: %w", err)
	}

	outputPath := filepath.Join(outDir, name+"-spdx.json")
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create SPDX output file: %w", err)
	}
	defer closeFile(f, "SPDX")

	if err = encoder.Encode(f, sbom); err != nil {
		return fmt.Errorf("failed to encode SPDX SBOM: %w", err)
	}

	return nil
}

// createCyclonedxSbom generates a CycloneDX 1.6 format SBOM file from the provided SBOM data.
func createCyclonedxSbom(sbom sbom.SBOM, name string, outDir string) error {
	cfg := cyclonedxjson.DefaultEncoderConfig()
	cfg.Pretty = false
	cfg.Version = "1.6"

	encoder, err := cyclonedxjson.NewFormatEncoderWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create CycloneDX encoder: %w", err)
	}

	outputPath := filepath.Join(outDir, name+"-cyclonedx.json")
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create CycloneDX output file: %w", err)
	}
	defer closeFile(f, "CycloneDX")

	if err = encoder.Encode(f, sbom); err != nil {
		return fmt.Errorf("failed to encode CycloneDX SBOM: %w", err)
	}

	return nil
}

// Generate creates SBOM files for the specified build directory.
func Generate(name string, spdx bool, cyclonedx bool, buildDir string, outDir string) (bool, error) {
	slog.Debug("Running generate with parameters",
		"name", name,
		"spdx", spdx,
		"cyclonedx", cyclonedx,
		"build_dir", buildDir,
		"out_dir", outDir)

	if !cyclonedx && !spdx {
		slog.Warn("No SBOM format selected")
		return false, nil
	}

	slog.Debug("Creating output directory", "path", outDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Syft generates the SBOM once with all the data it will need, and is then
	// able to output either SPDX or CycloneDX with the resulting data.
	// This single-analysis approach is more efficient than running separate scans
	// for each output format and ensures consistency between different SBOM formats.
	slog.Info("Starting SBOM analysis", "build_dir", buildDir)
	gsc := syft.DefaultGetSourceConfig()
	src, err := syft.GetSource(context.Background(), buildDir, gsc)
	if err != nil {
		return false, fmt.Errorf("failed to get source: %w", err)
	}

	slog.Debug("Creating SBOM data structure")
	sc := syft.DefaultCreateSBOMConfig()
	sbomData, err := sc.Create(context.Background(), src)
	if err != nil {
		return false, fmt.Errorf("failed to create SBOM: %w", err)
	}

	if spdx {
		slog.Info("Generating SPDX SBOM", "name", name)
		if err := createSpdxSbom(*sbomData, name, outDir); err != nil {
			return false, err
		}
		slog.Info("SPDX SBOM generated successfully")
	}

	if cyclonedx {
		slog.Info("Generating CycloneDX SBOM", "name", name)
		if err := createCyclonedxSbom(*sbomData, name, outDir); err != nil {
			return false, err
		}
		slog.Info("CycloneDX SBOM generated successfully")
	}

	slog.Info("All requested SBOM formats generated successfully")
	return true, nil
}
