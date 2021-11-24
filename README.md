# Bottlerocket SDK

This is the SDK for [Bottlerocket](https://github.com/bottlerocket-os/bottlerocket).

It provides the base layer used by package and variant builds.

## Contents

The SDK includes:
* Development tools from the host Linux distribution
* C and C++ cross-compilers for the target Linux distribution
* Kernel headers
* Toolchains for Rust and Go
* Software license scanner

### Availability

The SDK is available through Amazon ECR Public Gallery:

- [bottlerocket-sdk-aarch64](https://gallery.ecr.aws/bottlerocket/bottlerocket-sdk-aarch64)
- [bottlerocket-sdk-x86_64](https://gallery.ecr.aws/bottlerocket/bottlerocket-sdk-x86_64)
- [bottlerocket-toolchain-aarch64](https://gallery.ecr.aws/bottlerocket/bottlerocket-toolchain-aarch64)
- [bottlerocket-toolchain-x86_64](https://gallery.ecr.aws/bottlerocket/bottlerocket-toolchain-x86_64)

### Development

The SDK can be built for either **x86_64** or **aarch64**.
```shell
make ARCH="x86_64"
make ARCH="aarch64"
```

It supports either architecture for a build host in both cases.

See the [BUILDING](https://github.com/bottlerocket-os/bottlerocket-sdk/blob/develop/BUILDING.md) guide for more details.
