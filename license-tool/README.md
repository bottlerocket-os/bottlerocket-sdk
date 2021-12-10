# Bottlerocket's license-tool helper
There are cases where multiple licenses could apply to a package in the main bottlerocket repo, depending on who will distribute the software.
For such packages, the users should have the ability to choose what license they want to use for a package with multiple distribution licenses, without hardcoding this information in the RPM recipes.

The **license-tool** helper can be used to select the licenses that will apply to a set of packages.
The tool provides commands to retrieve the licenses' SPDX id and file paths, which are used in the RPM spec recipes.

The Bottlerocket build system uses this helper to let users select the license that best applies to them, for some of the third-party packages distributed in the base image.

## Use in the Bottlerocket build system
The Bottlerocket build system calls `bottlerocket-license-tool fetch` to create a local copy of the licenses files, before the packages builds begin.
In the spec file of a package with multiple distribution licenses, `bottlerocket-license-tool` is called with the `spdx-id` and `path` commands to fetch the license's SDPX id and file path.
For example, a RPM spec recipe could have:

```rpm-spec
%global spdx_id %(bottlerocket-license-tool spdx-id package-name)
%global license_file %(bottlerocket-license-tool path package-name -p ./licenses)

# ...

%package my-package
License: %{spdx_id}

# ...

%files my-package
%license %{license_file}
```

## Licenses file example

```toml
[my-package]
spdx-id = "SPDX-ID AND SPDX-ID-2 AND SPDX-ID-3" # Package with multiple licenses
licenses = [
  # This file is copied from a file system, and will be saved as `path`
  { license-url = "file:///path/to/spdx-id-license.txt", path = "spdx-id-license.txt" },
  # This file is fetched from a https endpoint, and will be saved as `path`
  { license-url = "https://localhost/spdx-id-license-v2.txt", path = "spdx-id-license-2.txt" }
  # This file is expected to be in the directory specified in `--prefix`
  { path = "spdx-id-license-3.txt" }
]
```
