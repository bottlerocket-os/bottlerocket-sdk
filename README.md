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

### Development

The SDK can be built for either x86_64 or aarch64.
```
make ARCH=x86_64
make ARCH=aarch64
```

Currently it assumes an x86_64 build host for both cases.
