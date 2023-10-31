# Bottlerocket's sbom-tool helper

Bottlerocket includes a Software Bill of Material (SBOM) for each variant built.
The **sbom-tool** helper is used as part of the build process to generate SBOMs for each package included in the variant.
The output is an SBOM for the source and dependencies used by each package, which is later combined to provide the full SBOM for the OS.

## Commands

The `sbom-tool` helper has two subcommands for use in creating an overall variant SBOM for Bottlerocket.

| Command | Description |
|---------|-------------|
| `sbom-tool generate` | Used to generate the package SBOM for each package being built in the build process. |
| `sbom-tool combine` | Used at the end of the build when the variant image is being generated to combine individual package SBOMs into a unified overall SBOM for the variant. |

## Use in the Bottlerocket build system

The Bottlerocket build system calls `bottlerocket-sbom-tool generate` to create an SBOM from the source code of each package.
SBOM generation is an option part of the build to allow development builds to be created without adding unnecessary overhead.
To support this ability, two RPM macros are provided that will make these steps no-ops if an SBOM is not needed.

| Macro | Description |
|-------|-------------|
| `%sbom_generate` | If SBOM creation is enabled, calls `bottlerocket-sbom-tool generate` to generate the SBOM |
| `%sbom` | If SBOM creation is enabled, places the previously generated SBOM in a well-known location |

The `%sbom_generate` macro is added to the spec file after the source archives have been extracted:

```rpm-spec
%prep
%setup -n %{gorepo}-%{gover} -q

%build
%set_cross_go_flags

# Generate SBOM
%sbom_generate ${TBD}

go build ...
```

The generated file is then later included as part of the package files, using the `%sbom` macro:

```rpm-spec
%files
%license LICENSE
%sbom ${TBD}
```
