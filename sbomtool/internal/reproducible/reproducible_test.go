package reproducible

import (
	"bytes"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

func testSBOM(names ...string) sbom.SBOM {
	var pkgs []pkg.Package
	for _, n := range names {
		p := pkg.Package{
			Name:    n,
			Version: "1.0",
			Type:    pkg.RpmPkg,
			CPEs:    []cpe.CPE{cpe.Must("cpe:2.3:a:bottlerocket:"+n+":1.0:*:*:*:*:*:*:*", cpe.DeclaredSource)},
		}
		p.SetID()
		pkgs = append(pkgs, p)
	}
	return sbom.SBOM{
		Artifacts: sbom.Artifacts{Packages: pkg.NewCollection(pkgs...)},
		Source: source.Description{
			Name:     "variant-rpms",
			Metadata: source.DirectoryMetadata{Path: "rpms"},
		},
		Descriptor: sbom.Descriptor{Name: "syft", Version: "test"},
	}
}

func TestTimestamp(t *testing.T) {
	t.Run("uses SOURCE_DATE_EPOCH in UTC", func(t *testing.T) {
		t.Setenv(SourceDateEpochEnv, "1792012736")
		ts, err := Timestamp()
		require.NoError(t, err)
		assert.Equal(t, "2026-10-14T21:18:56Z", ts.Format(time.RFC3339))
	})

	t.Run("falls back to the current time", func(t *testing.T) {
		t.Setenv(SourceDateEpochEnv, "")
		fixed := time.Date(2030, 1, 2, 3, 4, 5, 999, time.FixedZone("X", 3600))
		now = func() time.Time { return fixed }
		t.Cleanup(func() { now = time.Now })

		ts, err := Timestamp()
		require.NoError(t, err)
		assert.Equal(t, "2030-01-02T02:04:05Z", ts.Format(time.RFC3339))
	})

	for _, bad := range []string{"abc", "-1", "1.5", "1792012736 "} {
		t.Run("rejects "+bad, func(t *testing.T) {
			t.Setenv(SourceDateEpochEnv, bad)
			_, err := Timestamp()
			assert.ErrorContains(t, err, SourceDateEpochEnv)
		})
	}
}

func TestEncodeSPDX(t *testing.T) {
	t.Setenv(SourceDateEpochEnv, "1792012736")

	encode := func(s sbom.SBOM, version string) ([]byte, map[string]any) {
		var buf bytes.Buffer
		require.NoError(t, EncodeSPDX(&buf, s, version, false))
		var doc map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
		return buf.Bytes(), doc
	}

	for _, version := range []string{"2.2", "2.3"} {
		t.Run(version, func(t *testing.T) {
			a, docA := encode(testSBOM("a", "b"), version)
			b, _ := encode(testSBOM("a", "b"), version)
			_, docC := encode(testSBOM("a", "c"), version)

			assert.Equal(t, a, b, "same input must encode to the same bytes")
			assert.Equal(t, "SPDX-"+version, docA["spdxVersion"])
			assert.Equal(t, "2026-10-14T21:18:56Z", docA["creationInfo"].(map[string]any)["created"])
			assert.Regexp(t, regexp.MustCompile(`^https://anchore\.com/syft/dir/variant-rpms-`+uuidPattern+`$`), docA["documentNamespace"])
			assert.NotEqual(t, docA["documentNamespace"], docC["documentNamespace"], "different content must get a different namespace")
		})
	}

	t.Run("namespace depends on the timestamp", func(t *testing.T) {
		_, docA := encode(testSBOM("a"), "2.3")
		t.Setenv(SourceDateEpochEnv, "1792012737")
		_, docB := encode(testSBOM("a"), "2.3")
		assert.NotEqual(t, docA["documentNamespace"], docB["documentNamespace"])
	})
}

func TestEncodeCycloneDX(t *testing.T) {
	t.Setenv(SourceDateEpochEnv, "1792012736")

	encode := func(s sbom.SBOM) ([]byte, map[string]any) {
		var buf bytes.Buffer
		require.NoError(t, EncodeCycloneDX(&buf, s, "1.6", false))
		var doc map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
		return buf.Bytes(), doc
	}

	a, docA := encode(testSBOM("a", "b"))
	b, _ := encode(testSBOM("a", "b"))
	_, docC := encode(testSBOM("a", "c"))

	assert.Equal(t, a, b, "same input must encode to the same bytes")
	assert.Equal(t, "1.6", docA["specVersion"])
	assert.Equal(t, "2026-10-14T21:18:56Z", docA["metadata"].(map[string]any)["timestamp"])
	assert.Regexp(t, regexp.MustCompile(`^urn:uuid:`+uuidPattern+`$`), docA["serialNumber"])
	assert.NotEqual(t, docA["serialNumber"], docC["serialNumber"])

	t.Run("rejects unknown versions", func(t *testing.T) {
		assert.Error(t, EncodeCycloneDX(&bytes.Buffer{}, testSBOM("a"), "9.9", false))
	})
}

func TestNamespacePrefix(t *testing.T) {
	assert.Equal(t, "https://anchore.com/syft/dir/name-", namespacePrefix("https://anchore.com/syft/dir/name-1804ac57-1aa3-4446-9d9f-52dc3037ced0"))
	assert.Equal(t, "https://anchore.com/syft/dir/", namespacePrefix("https://anchore.com/syft/dir/1804ac57-1aa3-4446-9d9f-52dc3037ced0"))
	assert.Equal(t, "https://example.com/doc/", namespacePrefix("https://example.com/doc"))
	assert.Equal(t, "https://anchore.com/syft/unknown-source-type/", namespacePrefix(""))
}
