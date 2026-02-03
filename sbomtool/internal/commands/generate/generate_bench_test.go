package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/sbom"
)

func createTestSBOM(b *testing.B, dir string) sbom.SBOM {
	b.Helper()
	gsc := syft.DefaultGetSourceConfig()
	src, err := syft.GetSource(context.Background(), dir, gsc)
	if err != nil {
		b.Fatal(err)
	}
	sc := syft.DefaultCreateSBOMConfig()
	s, err := sc.Create(context.Background(), src)
	if err != nil {
		b.Fatal(err)
	}
	return *s
}

func setupBenchDir(b *testing.B, fileCount int) (string, string) {
	b.Helper()
	buildDir, err := os.MkdirTemp("", "gen-bench-build")
	if err != nil {
		b.Fatal(err)
	}
	outDir, err := os.MkdirTemp("", "gen-bench-out")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < fileCount; i++ {
		if err := os.WriteFile(filepath.Join(buildDir, fmt.Sprintf("file%d", i)), []byte("x"), 0644); err != nil {
			b.Fatal(err)
		}
	}
	return buildDir, outDir
}

func BenchmarkCreateSpdxSbom_Small(b *testing.B) {
	buildDir, outDir := setupBenchDir(b, 10)
	defer os.RemoveAll(buildDir)
	defer os.RemoveAll(outDir)
	s := createTestSBOM(b, buildDir)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = createSpdxSbom(s, "bench", outDir)
	}
}

func BenchmarkCreateCyclonedxSbom_Small(b *testing.B) {
	buildDir, outDir := setupBenchDir(b, 10)
	defer os.RemoveAll(buildDir)
	defer os.RemoveAll(outDir)
	s := createTestSBOM(b, buildDir)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = createCyclonedxSbom(s, "bench", outDir)
	}
}
