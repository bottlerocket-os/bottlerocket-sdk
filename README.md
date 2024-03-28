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

The [Bottlerocket SDK](https://gallery.ecr.aws/bottlerocket/bottlerocket-sdk) is available through Amazon ECR Public.

### Development

The SDK can be built on either an **x86_64** or an **aarch64** host.
```shell
make
```

See the [BUILDING](https://github.com/bottlerocket-os/bottlerocket-sdk/blob/develop/BUILDING.md) guide for more details.
