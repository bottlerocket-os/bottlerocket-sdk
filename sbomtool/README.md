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

All input files must be in the same format (SPDX or CycloneDX).

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

## Output

The tool generates SBOM files in the specified output directory:
- `{name}-spdx.json`: SPDX format SBOM
- `{name}-cyclonedx.json`: CycloneDX format SBOM

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
