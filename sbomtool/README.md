# sbomtool

A Software Bill of Materials (SBOM) generation tool for the Bottlerocket SDK.

## Overview

`sbomtool` is a command-line utility that generates standardized Software Bill of Materials (SBOM) files for software packages.
 It analyzes a build directory to identify all components and dependencies, then produces SBOM files in industry-standard formats.

## Features

- Generate SBOM files in multiple formats:
  - SPDX 2.3 (JSON)
  - CycloneDX 1.6 (JSON)
- Merge multiple SBOM files with intelligent deduplication
- Filter SBOM files based on buildroot contents

## Installation

The `sbomtool` is included in the Bottlerocket SDK. If you're using the SDK, the tool is already available.

## Usage

```
sbomtool [global options] command [command options]
```

### Global Options

- `--help`: Show help message
- `--log-level string`: Set log level (debug, info, warn, error) (default "info")

### Commands

#### Generate

Create SBOM files for a specified directory:

```
sbomtool generate [options]
```

Options:
- `--name string`: Name of the target package
- `--build-dir string`: Target directory of the package to analyze
- `--out-dir string`: Output directory for the SBOM files
- `--spdx`: Generate an SPDX SBOM
- `--cyclonedx`: Generate a CycloneDX SBOM

#### Merge

Merge multiple SBOM files into a single comprehensive SBOM:

```
sbomtool merge [options] file1 file2 [file3...]
```

Options:
- `--output string`: Output file path for merged SBOM (required)
- `--level int`: Merge level (reserved for future use) (default 0)

The merge command combines multiple SBOM files while:
- Deduplicating packages using CPE-based matching
- Preserving all dependency relationships
- Maintaining SBOM format integrity
- Providing comprehensive merge statistics

Input files can be mixed formats (SPDX and CycloneDX). The output format is determined by the first input file.

#### Filter

Remove components from an SBOM that are not present in a given buildroot directory:

```
sbomtool filter [options]
```

Options:
- `--input-sbom string`: Path to the SBOM file to filter (required)
- `--filter-by-buildroot string`: Buildroot directory whose contents determine which components are kept (required)
- `--output string`: Output file path for the filtered SBOM (required)

The filter command keeps only the components whose files are present in the buildroot, then applies the same CPE-based deduplication used by `merge`. The output preserves the format of the input SBOM.

### Examples

Generate an SPDX SBOM:
```
sbomtool generate --name mypackage --build-dir ./build --out-dir ./sbom --spdx
```

Generate a CycloneDX SBOM with debug logging:
```
sbomtool --log-level debug generate --name mypackage --build-dir ./build --out-dir ./sbom --cyclonedx
```

Generate both SPDX and CycloneDX SBOMs:
```
sbomtool generate --name mypackage --build-dir ./build --out-dir ./sbom --spdx --cyclonedx
```

Merge multiple SPDX SBOMs:
```
sbomtool merge --output merged.json app1-spdx.json app2-spdx.json lib1-spdx.json
```

Merge with debug logging:
```
sbomtool --log-level debug merge --output final.json app1.json app2.json app3.json
```

Filter an SBOM down to the components present in a buildroot:
```
sbomtool filter --input-sbom mypackage-spdx.json --filter-by-buildroot ./buildroot --output mypackage-filtered.json
```

## Output

The tool generates SBOM files in the specified output directory:
- `{name}-spdx.json`: SPDX format SBOM
- `{name}-cyclonedx.json`: CycloneDX format SBOM

## Reading a generated SBOM

Both output files are JSON, so you can inspect them directly with `jq` or hand them to any SPDX- or CycloneDX-aware tool.
Because `sbomtool` builds on [Syft](https://github.com/anchore/syft), the files are standard SPDX 2.3 and CycloneDX 1.6 documents and work with Syft, [Grype](https://github.com/anchore/grype), and other supply-chain tooling.

In an SPDX document, packages live under `.packages[]`; each entry carries a `name`, a `versionInfo`, license fields, and `externalRefs` (CPE and purl identifiers):

```
# List every package and its version
jq -r '.packages[] | "\(.name) \(.versionInfo)"' mypackage-spdx.json

# Show the CPE/purl external references for each package
jq '.packages[] | {name, refs: [.externalRefs[]?.referenceLocator]}' mypackage-spdx.json
```

In a CycloneDX document, the same data lives under `.components[]` as `name`, `version`, `purl`, and `licenses`:

```
jq -r '.components[] | "\(.name) \(.version)"' mypackage-cyclonedx.json
```

To check the inventory against known vulnerabilities, point a scanner at either file:

```
grype sbom:mypackage-spdx.json
```

### The whole-image SBOM

The per-package SBOMs produced by `generate` are merged (with `merge`) when a Bottlerocket image is assembled, producing a single image-wide SBOM in the running OS at `/usr/share/bottlerocket/spdx-sbom.json` and `/usr/share/bottlerocket/cyclonedx-sbom.json`.
These files can be read on a running node the same way, with `jq` or an SBOM-aware scanner.

## Deduplication Behavior

The merge command uses deduplication to combine packages from multiple SBOMs:

### CPE-Based Deduplication
- **Primary Strategy**: Uses CPE as the canonical identifier
- **Fallback Strategy**: Uses name + version + type for packages without CPE
- **Metadata Merging**: Combines licenses, files, and other metadata from duplicate packages
- **Relationship Preservation**: Updates all dependency relationships to reference canonical packages

### Deduplication Process
1. **Package Identity**: Generates canonical keys using CPE or fallback strategy
2. **Conflict Resolution**: First occurrence with CPE becomes canonical
3. **Metadata Consolidation**: Merges all metadata from duplicate packages
4. **Relationship Updates**: Updates all relationships to use canonical package IDs

## Implementation Details

`sbomtool` uses the [Anchore Syft](https://github.com/anchore/syft) library for SBOM generation, which provides comprehensive package detection across various ecosystems.

## License

This project is licensed under both:
- Apache License, Version 2.0
- MIT License

## Contributing

Contributions to improve `sbomtool` are welcome. Please see [CONTRIBUTING.md](../CONTRIBUTING.md) for details on how to contribute to this project. Ensure your code follows the Go style guidelines and includes appropriate tests.
