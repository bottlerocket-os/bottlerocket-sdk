package generate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCloseFile verifies that file handles are properly closed and errors are logged.
//
// Given: A temporary file is created and opened
// When: closeFile is called on the file handle
// Then: The file should be closed and subsequent operations should fail
func TestCloseFile(t *testing.T) {
	tempFile, err := os.CreateTemp("", "close-test")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	closeFile(tempFile, "test")

	_, err = tempFile.Write([]byte("test"))
	assert.Error(t, err, "Expected error writing to closed file")
}

// TestGenerateNoFormats verifies that Generate handles the case where no output formats are selected.
//
// Given: Generate is called with both spdx and cyclonedx flags set to false
// When: The function executes
// Then: It should return false with no error, indicating no work was performed
func TestGenerateNoFormats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sbom-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	result, err := Generate("test", false, false, "/test/build", tempDir)

	assert.False(t, result, "Expected result to be false when no formats are selected")
	assert.NoError(t, err, "Expected no error when no formats are selected")
}
