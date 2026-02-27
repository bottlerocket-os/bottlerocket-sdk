// Package processor provides Syft-based SBOM processing capabilities optimized for Bottlerocket builds.
// It configures Syft's catalogers for comprehensive package detection including Go and Rust binary analysis.

package processor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/anchore/syft/syft/format"
	"github.com/anchore/syft/syft/format/cyclonedxjson"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"golang.org/x/sync/errgroup"
)

// LoadSBOM loads an existing SBOM using Syft's format detection.
// It returns the SBOM, format identifier, and any error encountered.
func LoadSBOM(path string) (*sbom.SBOM, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open SBOM file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Warn("Failed to close SBOM file", "error", closeErr)
		}
	}()

	s, formatID, _, err := format.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode SBOM: %w", err)
	}

	slog.Debug("SBOM loaded successfully",
		"format", formatID,
		"packages", s.Artifacts.Packages.PackageCount(),
		"relationships", len(s.Relationships))

	return s, string(formatID), nil
}

// SaveSBOM saves an SBOM using Syft's format encoders.
// It supports all formats that Syft can encode.
func SaveSBOM(s *sbom.SBOM, path, formatName string) error {
	var encoder sbom.FormatEncoder
	var err error

	if formatName == "cyclonedx-json" {
		encoder, err = cyclonedxjson.NewFormatEncoderWithConfig(cyclonedxjson.EncoderConfig{
			Version: "1.6",
			Pretty:  true,
		})
		if err != nil {
			return fmt.Errorf("failed to create CycloneDX encoder: %w", err)
		}
	} else {
		for _, enc := range format.Encoders() {
			if string(enc.ID()) == formatName {
				encoder = enc
				break
			}
		}
	}

	if encoder == nil {
		return fmt.Errorf("unsupported output format: %s", formatName)
	}

	bytes, err := format.Encode(*s, encoder)
	if err != nil {
		return fmt.Errorf("failed to encode SBOM: %w", err)
	}

	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write SBOM to file: %w", err)
	}

	return nil
}

// countPackagesByType counts packages of a specific type for logging purposes.
func countPackagesByType(catalog *pkg.Collection, pkgType pkg.Type) int {
	count := 0
	for _, pkg := range catalog.Sorted() {
		if pkg.Type == pkgType {
			count++
		}
	}
	return count
}

// SaveSBOMBothFormats saves the SBOM in both SPDX and CycloneDX formats in parallel.
func SaveSBOMBothFormats(ctx context.Context, s *sbom.SBOM, basePath string) error {
	base := strings.TrimSuffix(basePath, ".json")

	g, ctx := errgroup.WithContext(ctx)

	formats := []struct {
		name string
		id   string
	}{
		{"spdx", "spdx-json"},
		{"cyclonedx", "cyclonedx-json"},
	}

	for _, f := range formats {
		name, formatID := f.name, f.id
		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := fmt.Sprintf("%s-%s.json", base, name)
			if err := SaveSBOM(s, path, formatID); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return nil
		})
	}

	return g.Wait()
}
