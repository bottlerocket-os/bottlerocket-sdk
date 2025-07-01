# sbomtool

A Software Bill of Materials (SBOM) generation tool for the Bottlerocket SDK.

## Overview

`sbomtool` is a command-line utility that generates standardized Software Bill of Materials (SBOM) files for software packages.
 It analyzes a build directory to identify all components and dependencies, then produces SBOM files in industry-standard formats.

## Features

- Generate SBOM files in multiple formats:
  - SPDX 2.3 (JSON)
  - CycloneDX 1.6 (JSON)
- Future support for merging multiple SBOM files

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

#### Merge (Future Feature)

Merge multiple SBOM files:

```
sbomtool merge [options] file1 file2 [file3...]
```

Options:
- `--level int`: Merge level (default 0)

Note: This feature is not yet implemented.

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

## Output

The tool generates SBOM files in the specified output directory:
- `{name}-spdx.json`: SPDX format SBOM
- `{name}-cyclonedx.json`: CycloneDX format SBOM

## Implementation Details

`sbomtool` uses the [Anchore Syft](https://github.com/anchore/syft) library for SBOM generation, which provides comprehensive package detection across various ecosystems.

## License

This project is licensed under both:
- Apache License, Version 2.0
- MIT License

## Contributing

Contributions to improve `sbomtool` are welcome. Please see [CONTRIBUTING.md](../CONTRIBUTING.md) for details on how to contribute to this project. Ensure your code follows the Go style guidelines and includes appropriate tests.
