package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft/cpe"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bottlerocket-os/bottlerocket-sdk/sbomtool/go/internal/reproducible"
)

// mergeRuns repeats each merge enough times for randomized map order to surface.
const mergeRuns = 25

type reproPackage struct {
	name     string
	version  string
	cpes     []string
	licenses []string
}

// writeReproInput writes an SBOM with the given packages in the given format ("spdx" or
// "cyclonedx") and returns its path.
func writeReproInput(t *testing.T, dir, name, format string, packages []reproPackage) string {
	t.Helper()

	var pkgs []pkg.Package
	for _, rp := range packages {
		p := pkg.Package{Name: rp.name, Version: rp.version, Type: pkg.RpmPkg}
		for _, c := range rp.cpes {
			p.CPEs = append(p.CPEs, cpe.Must(c, cpe.DeclaredSource))
		}
		for _, l := range rp.licenses {
			p.Licenses.Add(pkg.NewLicense(l))
		}
		p.SetID()
		pkgs = append(pkgs, p)
	}
	s := sbom.SBOM{
		Artifacts:  sbom.Artifacts{Packages: pkg.NewCollection(pkgs...)},
		Source:     source.Description{Name: name, Version: "1.0"},
		Descriptor: sbom.Descriptor{Name: "sbomtool-test", Version: "0.0.0"},
	}

	var buf bytes.Buffer
	switch format {
	case "spdx":
		require.NoError(t, reproducible.EncodeSPDX(&buf, s, "2.3", false))
	case "cyclonedx":
		require.NoError(t, reproducible.EncodeCycloneDX(&buf, s, "1.6", false))
	default:
		t.Fatalf("unknown format %q", format)
	}
	path := filepath.Join(dir, name+"-"+format+".json")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))
	return path
}

func runMergeCommand(t *testing.T, output string, inputs ...string) []byte {
	t.Helper()
	rootCmd := createRootCommand()
	rootCmd.SetArgs(append([]string{"--log-level", "error", "merge", "--output", output}, inputs...))
	require.NoError(t, rootCmd.Execute())
	out, err := os.ReadFile(output)
	require.NoError(t, err)
	return out
}

// TestMergeIsReproducible merges SBOMs whose packages carry several overlapping CPEs and
// licenses, many times over, and requires byte-identical output every time.
func TestMergeIsReproducible(t *testing.T) {
	t.Setenv(reproducible.SourceDateEpochEnv, "1792012736")

	// Overlapping CPE lists force a union; "zeta" has no CPEs and matches by name+version+type.
	first := []reproPackage{
		{name: "apiclient", version: "1.2.3", cpes: []string{
			"cpe:2.3:a:bottlerocket:apiclient:1.2.3:*:*:*:*:*:*:*",
			"cpe:2.3:a:bottlerocket_apiclient:apiclient:1.2.3:*:*:*:*:*:*:*",
			"cpe:2.3:a:apiclient:apiclient:1.2.3:*:*:*:*:*:*:*",
		}, licenses: []string{"Apache-2.0", "MIT"}},
		{name: "glibc", version: "2.40", cpes: []string{
			"cpe:2.3:a:gnu:glibc:2.40:*:*:*:*:*:*:*",
			"cpe:2.3:a:bottlerocket:glibc:2.40:*:*:*:*:*:*:*",
		}, licenses: []string{"LGPL-2.1-or-later"}},
		{name: "zeta", version: "0.1", licenses: []string{"BSD-3-Clause"}},
	}
	second := []reproPackage{
		{name: "apiclient", version: "1.2.3", cpes: []string{
			"cpe:2.3:a:apiclient_project:apiclient:1.2.3:*:*:*:*:*:*:*",
			"cpe:2.3:a:bottlerocket_apiclient:apiclient:1.2.3:*:*:*:*:*:*:*",
			"cpe:2.3:a:amazon:apiclient:1.2.3:*:*:*:*:*:*:*",
			"cpe:2.3:a:aws:apiclient:1.2.3:*:*:*:*:*:*:*",
		}, licenses: []string{"MIT", "Apache-2.0"}},
		{name: "glibc", version: "2.40", cpes: []string{
			"cpe:2.3:a:glibc:glibc:2.40:*:*:*:*:*:*:*",
			"cpe:2.3:a:gnu:glibc:2.40:*:*:*:*:*:*:*",
			"cpe:2.3:a:gnu_project:glibc:2.40:*:*:*:*:*:*:*",
		}, licenses: []string{"GPL-2.0-or-later"}},
		{name: "zeta", version: "0.1", licenses: []string{"BSD-2-Clause"}},
	}
	third := []reproPackage{
		{name: "glibc", version: "2.40", cpes: []string{
			"cpe:2.3:a:the_gnu_project:glibc:2.40:*:*:*:*:*:*:*",
			"cpe:2.3:a:bottlerocket:glibc:2.40:*:*:*:*:*:*:*",
			"cpe:2.3:a:gnu_c_library:glibc:2.40:*:*:*:*:*:*:*",
		}},
	}

	for _, format := range []string{"spdx", "cyclonedx"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			inputs := []string{
				writeReproInput(t, dir, "first", format, first),
				writeReproInput(t, dir, "second", format, second),
				writeReproInput(t, dir, "third", format, third),
			}

			want := runMergeCommand(t, filepath.Join(dir, "merged-0.json"), inputs...)
			for i := 1; i < mergeRuns; i++ {
				got := runMergeCommand(t, filepath.Join(dir, "merged.json"), inputs...)
				require.Equal(t, string(want), string(got), "merge run %d differs from run 0", i)
			}

			// Union keeps first-appearance order; CPEs[0] comes from the first input.
			wantCPEs := map[string][]string{
				"apiclient": {
					"cpe:2.3:a:bottlerocket:apiclient:1.2.3:*:*:*:*:*:*:*",
					"cpe:2.3:a:bottlerocket_apiclient:apiclient:1.2.3:*:*:*:*:*:*:*",
					"cpe:2.3:a:apiclient:apiclient:1.2.3:*:*:*:*:*:*:*",
					"cpe:2.3:a:apiclient_project:apiclient:1.2.3:*:*:*:*:*:*:*",
					"cpe:2.3:a:amazon:apiclient:1.2.3:*:*:*:*:*:*:*",
					"cpe:2.3:a:aws:apiclient:1.2.3:*:*:*:*:*:*:*",
				},
			}
			gotCPEs := mergedCPEs(t, want)
			assert.Equal(t, wantCPEs["apiclient"], gotCPEs["apiclient"])
			assert.Equal(t, "cpe:2.3:a:gnu:glibc:2.40:*:*:*:*:*:*:*", gotCPEs["glibc"][0])
			assert.Len(t, gotCPEs["glibc"], 6)
		})
	}
}

// mergedCPEs extracts each package's CPEs, in document order, from SPDX or CycloneDX JSON.
func mergedCPEs(t *testing.T, doc []byte) map[string][]string {
	t.Helper()
	var parsed struct {
		Packages []struct {
			Name         string `json:"name"`
			ExternalRefs []struct {
				Type    string `json:"referenceType"`
				Locator string `json:"referenceLocator"`
			} `json:"externalRefs"`
		} `json:"packages"`
		Components []struct {
			Name       string `json:"name"`
			CPE        string `json:"cpe"`
			Properties []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"properties"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(doc, &parsed))

	result := map[string][]string{}
	for _, p := range parsed.Packages {
		for _, ref := range p.ExternalRefs {
			if ref.Type == "cpe23Type" {
				result[p.Name] = append(result[p.Name], ref.Locator)
			}
		}
	}
	for _, c := range parsed.Components {
		if c.CPE != "" {
			result[c.Name] = append(result[c.Name], c.CPE)
		}
		for _, prop := range c.Properties {
			if prop.Name == "syft:cpe23" {
				result[c.Name] = append(result[c.Name], prop.Value)
			}
		}
	}
	return result
}
