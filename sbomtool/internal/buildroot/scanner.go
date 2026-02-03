// Package buildroot provides utilities for scanning and analyzing buildroot directory structures.
package buildroot

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// Scanner provides comprehensive buildroot directory scanning functionality.
type Scanner struct {
	maxDepth        int
	excludePatterns []string
}

// ScanResult contains the complete results of a buildroot directory scan.
// Note: mu is only initialized by ScanDirectory(); zero-value ScanResults have nil mu.
type ScanResult struct {
	AllFiles        []string
	RegularFiles    []string
	SymbolicLinks   []string
	Directories     []string
	NormalizedPaths map[string]string
	TotalSize       int64
	ScanDuration    time.Duration
	mu              *sync.RWMutex
}

// fileWork represents a file to be processed by workers.
type fileWork struct {
	path  string
	entry os.DirEntry
}

// NewScanner creates a new buildroot scanner with the specified configuration.
func NewScanner(maxDepth int, excludePatterns []string) *Scanner {
	return &Scanner{
		maxDepth:        maxDepth,
		excludePatterns: excludePatterns,
	}
}

// ScanDirectory performs comprehensive recursive scanning of the buildroot directory.
func (s *Scanner) ScanDirectory(buildrootPath string) (*ScanResult, error) {
	startTime := time.Now()

	if _, err := os.Stat(buildrootPath); err != nil {
		return nil, fmt.Errorf("buildroot path not accessible: %w", err)
	}

	result := &ScanResult{
		AllFiles:        make([]string, 0),
		RegularFiles:    make([]string, 0),
		SymbolicLinks:   make([]string, 0),
		Directories:     make([]string, 0),
		NormalizedPaths: make(map[string]string),
		mu:              &sync.RWMutex{},
	}

	// Start worker pool for parallel file processing
	numWorkers := runtime.GOMAXPROCS(0)
	// Buffer 100 items per worker to reduce contention while limiting memory usage
	workCh := make(chan fileWork, numWorkers*100)
	var wg sync.WaitGroup
	var workerErr error
	var errMu sync.Mutex
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workCh {
				if err := s.processFile(work.path, work.entry, result); err != nil {
					errMu.Lock()
					if workerErr == nil {
						workerErr = err
					} else {
						slog.Debug("Additional worker error", "path", work.path, "error", err)
					}
					errMu.Unlock()
				}
			}
		}()
	}

	// Walk directories sequentially, send files to workers
	walkErr := s.walkDirectories(buildrootPath, 0, result, workCh)
	close(workCh)
	wg.Wait()

	if walkErr != nil {
		return nil, fmt.Errorf("failed to scan buildroot: %w", walkErr)
	}
	if workerErr != nil {
		return nil, fmt.Errorf("failed to scan buildroot: %w", workerErr)
	}

	// Sort for deterministic output (parallel workers append in arbitrary order)
	slices.Sort(result.AllFiles)
	slices.Sort(result.RegularFiles)
	slices.Sort(result.SymbolicLinks)
	slices.Sort(result.Directories)

	// Build normalized paths (safe after workers complete)
	for _, filePath := range result.AllFiles {
		normalized, err := BuildrootToInstalled(filePath, buildrootPath)
		if err != nil {
			slog.Warn("Failed to normalize path", "path", filePath, "error", err)
			continue
		}
		result.NormalizedPaths[filePath] = normalized
	}

	result.ScanDuration = time.Since(startTime)

	slog.Info("Buildroot scan completed",
		"path", buildrootPath,
		"total_files", len(result.AllFiles),
		"regular_files", len(result.RegularFiles),
		"symbolic_links", len(result.SymbolicLinks),
		"directories", len(result.Directories),
		"normalized_paths", len(result.NormalizedPaths),
		"total_size_mb", result.TotalSize/(1024*1024),
		"duration_ms", result.ScanDuration.Milliseconds())

	return result, nil
}

// walkDirectories walks directories sequentially and sends files to workCh.
func (s *Scanner) walkDirectories(currentPath string, depth int, result *ScanResult, workCh chan<- fileWork) error {
	if depth > s.maxDepth {
		return nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("failed to read buildroot directory %s: %w", currentPath, err)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(currentPath, entry.Name())

		exclude, err := s.shouldExclude(fullPath)
		if err != nil {
			return err
		}
		if exclude {
			continue
		}

		if entry.IsDir() {
			// Process directory info inline (cheap)
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("failed to get dir info for %s: %w", fullPath, err)
			}
			size := info.Size()
			result.mu.Lock()
			result.AllFiles = append(result.AllFiles, fullPath)
			result.Directories = append(result.Directories, fullPath)
			result.TotalSize += size
			result.mu.Unlock()

			// Recurse into subdirectory
			if err := s.walkDirectories(fullPath, depth+1, result, workCh); err != nil {
				return err
			}
		} else {
			// Send file to worker pool for parallel processing
			workCh <- fileWork{path: fullPath, entry: entry}
		}
	}

	return nil
}

// processFile processes a single file (called by workers).
func (s *Scanner) processFile(fullPath string, entry os.DirEntry, result *ScanResult) error {
	info, err := os.Lstat(fullPath)
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", fullPath, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		size := info.Size()
		result.mu.Lock()
		result.AllFiles = append(result.AllFiles, fullPath)
		result.SymbolicLinks = append(result.SymbolicLinks, fullPath)
		result.TotalSize += size
		result.mu.Unlock()
		return nil
	}

	size := info.Size()
	result.mu.Lock()
	result.AllFiles = append(result.AllFiles, fullPath)
	result.TotalSize += size
	if info.Mode().IsRegular() {
		result.RegularFiles = append(result.RegularFiles, fullPath)
	}
	result.mu.Unlock()

	return nil
}

// GetNormalizedPaths extracts normalized paths from scan results.
func (sr *ScanResult) GetNormalizedPaths() []string {
	if sr.mu != nil {
		sr.mu.RLock()
		defer sr.mu.RUnlock()
	}
	var normalizedPaths []string
	for _, normalized := range sr.NormalizedPaths {
		normalizedPaths = append(normalizedPaths, normalized)
	}
	return normalizedPaths
}

// shouldExclude checks if a path should be excluded from scanning.
func (s *Scanner) shouldExclude(path string) (bool, error) {
	for _, pattern := range s.excludePatterns {
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return false, fmt.Errorf("invalid exclusion pattern %q: %w", pattern, err)
		}
		if matched {
			return true, nil
		}

		matched, err = filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return false, fmt.Errorf("invalid exclusion pattern %q: %w", pattern, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// BuildrootToInstalled converts a buildroot path to an installed path.
func BuildrootToInstalled(buildrootFile, buildrootPath string) (string, error) {
	cleanFile := filepath.Clean(buildrootFile)
	cleanBuildroot := filepath.Clean(buildrootPath)
	rel, err := filepath.Rel(cleanBuildroot, cleanFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside buildroot: %s", buildrootFile)
	}

	installedPath := filepath.Clean("/" + rel)

	return installedPath, nil
}
