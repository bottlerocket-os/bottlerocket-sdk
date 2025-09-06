// Package merge provides functionality for merging multiple SBOM files.
//
// The merge functionality is not yet implemented.
package merge

import (
	"errors"
)

// ErrNotImplemented is returned when attempting to use unimplemented functionality.
var ErrNotImplemented = errors.New("merge functionality is not yet implemented")

// Merge combines multiple SBOM files into a single comprehensive SBOM.
// This function is a placeholder for future implementation and always returns ErrNotImplemented.
func Merge(level int, files []string) (bool, error) {
	return false, ErrNotImplemented
}
