package buildroot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func createTestBuildroot(b *testing.B, fileCount int) string {
	b.Helper()
	dir, err := os.MkdirTemp("", "scanner-bench")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < fileCount; i++ {
		subdir := filepath.Join(dir, "usr", "lib")
		if err := os.MkdirAll(subdir, 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subdir, fmt.Sprintf("file_%d.txt", i)), []byte("x"), 0644); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}

func BenchmarkScanDirectory_Small(b *testing.B) {
	dir := createTestBuildroot(b, 100)
	defer os.RemoveAll(dir)
	scanner := NewScanner(10, nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = scanner.ScanDirectory(dir)
	}
}

func BenchmarkScanDirectory_Medium(b *testing.B) {
	dir := createTestBuildroot(b, 1000)
	defer os.RemoveAll(dir)
	scanner := NewScanner(10, nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = scanner.ScanDirectory(dir)
	}
}

func BenchmarkGetNormalizedPaths(b *testing.B) {
	dir := createTestBuildroot(b, 500)
	defer os.RemoveAll(dir)
	scanner := NewScanner(10, nil)
	result, _ := scanner.ScanDirectory(dir)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = result.GetNormalizedPaths()
	}
}
