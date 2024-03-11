FROM public.ecr.aws/docker/library/fedora:39 as base

# Everything we need to build our SDK and packages.
RUN \
  dnf makecache && \
  dnf -y update && \
  dnf -y install --setopt=install_weak_deps=False \
    bc \
    bison \
    cmake \
    cpio \
    curl \
    dnf-plugins-core \
    dwarves \
    elfutils-devel \
    flex \
    g++ \
    gcc \
    git \
    gperf \
    hostname \
    intltool \
    jq \
    json-c-devel \
    kmod \
    libcurl-devel \
    libtool \
    meson \
    openssl \
    openssl-devel \
    p11-kit-devel \
    perl-ExtUtils-MakeMaker \
    perl-FindBin \
    perl-IPC-Cmd \
    perl-open \
    python \
    rsync \
    wget \
    which \
  && \
  dnf config-manager --set-disabled \
    fedora-cisco-openh264 \
  && \
  useradd builder
COPY ./sdk-fetch /usr/local/bin

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# We expect our C cross-compiler to be used on other distros for building kernel
# modules, so we build it with an older glibc for compatibility.
FROM public.ecr.aws/docker/library/ubuntu:16.04 as compat
RUN \
  apt-get update && \
  apt-get -y dist-upgrade && \
  apt-get -y install \
    autoconf \
    automake \
    bc \
    build-essential \
    cpio \
    curl \
    file \
    git \
    libexpat1-dev \
    libtool \
    libz-dev \
    pkgconf \
    python3 \
    unzip \
    wget \
  && \
  useradd -m -u 1000 builder
COPY ./sdk-fetch /usr/local/bin

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM compat as toolchain
USER builder

# Configure Git for any subsequent use.
RUN \
  git config --global user.name "Builder" && \
  git config --global user.email "builder@localhost"

ARG UPSTREAM_SOURCE_FALLBACK
ENV BRVER="2022.11.1"
ENV KVER="5.10.162"

WORKDIR /home/builder
COPY ./hashes/buildroot ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf buildroot-${BRVER}.tar.xz && \
  rm buildroot-${BRVER}.tar.xz && \
  mv buildroot-${BRVER} buildroot && \
  mv queue.h queue.h?rev=1.70

WORKDIR /home/builder/buildroot
COPY ./patches/buildroot/* ./
COPY ./configs/buildroot/* ./configs/
COPY ./helpers/buildroot/* ./
RUN \
  git init . && \
  git apply --whitespace=nowarn *.patch

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain as toolchain-gnu-x86_64
ENV ARCH="x86_64"
RUN ./build-gnu-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain as toolchain-gnu-aarch64
ENV ARCH="aarch64"
RUN ./build-gnu-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain as toolchain-musl-x86_64
ENV ARCH="x86_64"
RUN ./build-musl-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain as toolchain-musl-aarch64
ENV ARCH="aarch64"
RUN ./build-musl-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Add our cross-compilers to the base SDK layer.
FROM base as sdk
USER root

ARG UPSTREAM_SOURCE_FALLBACK
ENV KVER="5.10.162"

WORKDIR /

COPY --chown=0:0 --from=toolchain-gnu-x86_64 \
  /home/builder/buildroot/output/x86_64-gnu/toolchain/ /
COPY --chown=0:0 --from=toolchain-gnu-x86_64 \
  /home/builder/buildroot/output/x86_64-gnu/build/linux-headers-${KVER}/usr/include/ \
  /x86_64-bottlerocket-linux-gnu/sys-root/usr/include/
COPY --chown=0:0 --from=toolchain-gnu-x86_64 \
  /home/builder/buildroot/output/x86_64-gnu/build/licenses/ \
  /x86_64-bottlerocket-linux-gnu/sys-root/usr/share/licenses/

COPY --chown=0:0 --from=toolchain-gnu-aarch64 \
  /home/builder/buildroot/output/aarch64-gnu/toolchain/ /
COPY --chown=0:0 --from=toolchain-gnu-aarch64 \
  /home/builder/buildroot/output/aarch64-gnu/build/linux-headers-${KVER}/usr/include/ \
  /aarch64-bottlerocket-linux-gnu/sys-root/usr/include/
COPY --chown=0:0 --from=toolchain-gnu-aarch64 \
  /home/builder/buildroot/output/aarch64-gnu/build/licenses/ \
  /aarch64-bottlerocket-linux-gnu/sys-root/usr/share/licenses/

COPY --chown=0:0 --from=toolchain-musl-x86_64 \
  /home/builder/buildroot/output/x86_64-musl/toolchain/ /
COPY --chown=0:0 --from=toolchain-musl-x86_64 \
  /home/builder/buildroot/output/x86_64-musl/build/linux-headers-${KVER}/usr/include/ \
  /x86_64-bottlerocket-linux-musl/sys-root/usr/include/
COPY --chown=0:0 --from=toolchain-musl-x86_64 \
  /home/builder/buildroot/output/x86_64-musl/build/licenses/ \
  /x86_64-bottlerocket-linux-musl/sys-root/usr/share/licenses/

COPY --chown=0:0 --from=toolchain-musl-aarch64 \
  /home/builder/buildroot/output/aarch64-musl/toolchain/ /
COPY --chown=0:0 --from=toolchain-musl-aarch64 \
  /home/builder/buildroot/output/aarch64-musl/build/linux-headers-${KVER}/usr/include/ \
  /aarch64-bottlerocket-linux-musl/sys-root/usr/include/
COPY --chown=0:0 --from=toolchain-musl-aarch64 \
  /home/builder/buildroot/output/aarch64-musl/build/licenses/ \
  /aarch64-bottlerocket-linux-musl/sys-root/usr/share/licenses/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Build C libraries so we can build our rust and golang toolchains.
FROM sdk as sdk-gnu
USER builder

WORKDIR /home/builder
COPY ./hashes/glibc ./hashes
COPY ./helpers/glibc/* ./

ENV GLIBCVER="2.37"
ENV KVER="5.10.162"
RUN \
  sdk-fetch hashes && \
  tar xf glibc-${GLIBCVER}.tar.xz && \
  rm glibc-${GLIBCVER}.tar.xz && \
  mv glibc-${GLIBCVER} glibc && \
  cd glibc && \
  mkdir build

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-gnu as sdk-gnu-x86_64
ENV ARCH="x86_64"
RUN ./build-glibc.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-gnu as sdk-gnu-aarch64
ENV ARCH="aarch64"
RUN ./build-glibc.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-musl
USER builder

WORKDIR /home/builder
COPY ./hashes/musl ./hashes
COPY ./helpers/musl/* ./

ENV MUSLVER="1.2.3"
RUN \
  sdk-fetch hashes && \
  tar xf musl-${MUSLVER}.tar.gz && \
  rm musl-${MUSLVER}.tar.gz && \
  mv musl-${MUSLVER} musl

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-musl as sdk-musl-x86_64
ENV ARCH="x86_64"
RUN ./build-musl.sh --arch="${ARCH}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-musl as sdk-musl-aarch64
ENV ARCH="aarch64"
RUN ./build-musl.sh --arch="${ARCH}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Rust's musl targets depend on libunwind.
FROM sdk as sdk-libunwind
USER builder

WORKDIR /home/builder
COPY ./hashes/libunwind ./hashes
COPY ./helpers/libunwind/* ./

ENV LLVMVER="14.0.6"
RUN \
  sdk-fetch hashes && \
  tar xf llvm-${LLVMVER}.src.tar.xz && \
  rm llvm-${LLVMVER}.src.tar.xz && \
  mv llvm-${LLVMVER}.src llvm && \
  tar xf libcxx-${LLVMVER}.src.tar.xz && \
  rm libcxx-${LLVMVER}.src.tar.xz && \
  mv libcxx-${LLVMVER}.src libcxx && \
  tar xf libunwind-${LLVMVER}.src.tar.xz && \
  rm libunwind-${LLVMVER}.src.tar.xz && \
  mv libunwind-${LLVMVER}.src libunwind && \
  mkdir libunwind/build

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libunwind as sdk-libunwind-x86_64

ENV ARCH="x86_64"
ENV MUSL_TARGET="${ARCH}-bottlerocket-linux-musl"

COPY --chown=0:0 --from=sdk-musl-x86_64 \
  /home/builder/musl/output/${MUSL_TARGET}/sys-root/ \
  /${MUSL_TARGET}/sys-root/

RUN ./build-libunwind.sh --arch="${ARCH}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libunwind as sdk-libunwind-aarch64

ENV ARCH="aarch64"
ENV MUSL_TARGET="${ARCH}-bottlerocket-linux-musl"

COPY --chown=0:0 --from=sdk-musl-aarch64 \
  /home/builder/musl/output/${MUSL_TARGET}/sys-root/ \
  /${MUSL_TARGET}/sys-root/

RUN ./build-libunwind.sh --arch="${ARCH}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM scratch as sdk-libc-gnu

ENV GNU_TARGET_x86_64="x86_64-bottlerocket-linux-gnu"
ENV GNU_TARGET_aarch64="aarch64-bottlerocket-linux-gnu"

COPY --chown=0:0 --from=sdk-gnu-x86_64 \
  /home/builder/glibc/output/${GNU_TARGET_x86_64}/sys-root/ \
  /${GNU_TARGET_x86_64}/sys-root/

COPY --chown=0:0 --from=sdk-gnu-aarch64 \
  /home/builder/glibc/output/${GNU_TARGET_aarch64}/sys-root/ \
  /${GNU_TARGET_aarch64}/sys-root/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM scratch as sdk-libc-musl
ENV MUSL_TARGET_x86_64="x86_64-bottlerocket-linux-musl"
ENV MUSL_TARGET_aarch64="aarch64-bottlerocket-linux-musl"

COPY --chown=0:0 --from=sdk-musl-x86_64 \
  /home/builder/musl/output/${MUSL_TARGET_x86_64}/sys-root/ \
  /${MUSL_TARGET_x86_64}/sys-root/

COPY --chown=0:0 --from=sdk-libunwind-x86_64 \
  /home/builder/libunwind/output/${MUSL_TARGET_x86_64}/sys-root/ \
  /${MUSL_TARGET_x86_64}/sys-root/

COPY --chown=0:0 --from=sdk-musl-aarch64 \
  /home/builder/musl/output/${MUSL_TARGET_aarch64}/sys-root/ \
  /${MUSL_TARGET_aarch64}/sys-root/

COPY --chown=0:0 --from=sdk-libunwind-aarch64 \
  /home/builder/libunwind/output/${MUSL_TARGET_aarch64}/sys-root/ \
  /${MUSL_TARGET_aarch64}/sys-root/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-libc

COPY --from=sdk-libc-gnu / /
COPY --from=sdk-libc-musl / /

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libc as sdk-rust

USER root
RUN \
  mkdir -p /usr/libexec/rust && \
  chown -R builder:builder /usr/libexec/rust

ARG HOST_ARCH
ENV VENDOR="bottlerocket"
ENV RUSTVER="1.77.0"

USER builder
WORKDIR /home/builder
COPY ./hashes/rust ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf rustc-${RUSTVER}-src.tar.xz && \
  rm rustc-${RUSTVER}-src.tar.xz && \
  mv rustc-${RUSTVER}-src rust

WORKDIR /home/builder/rust
RUN \
  dir=build/cache/$(jq -r '.compiler.date' src/stage0.json); \
  mkdir -p $dir && mv ../*.xz $dir

# For any architecture, we rely on two or more of Rust's native targets:
#
# 1) the host platform
#    (x86_64-unknown-linux-gnu for a Fedora x86_64 host)
# 2) the target platform for dynamically linked builds
#    (x86_64-unknown-linux-gnu for a Bottlerocket x86_64 target)
# 3) the target platform for statically linked builds
#    (x86_64-unknown-linux-musl for a Bottlerocket x86_64 target)
#
# We need to override the C compiler used for linking the targets in #2 and #3,
# to ensure that the libraries in our sysroot are used instead of the host's
# libraries.
#
# If the target in #1 is the same as #2 or #3, then we're in trouble. This can
# happen with build scripts, which may require us to build for the host before
# we can build for the target. In this scenario, we have to pick from two bad
# options: link host programs with the target's libraries, which may fail to
# run if the host's libraries are too old; or link target programs with the
# host's libraries, which may fail to run if the host's libraries are too new.
#
# To resolve this, we create vendor-specific targets based on the native ones.
# That allows us to leave the settings for the host platform alone, while also
# ensuring that the target platform always uses the libraries from our sysroot.
# These vendor targets are effectively the same as the "unknown" targets, so we
# just need to copy them, change the "vendor" field, and refer to them in the
# module so `rustc` knows they exist.

RUN \
  for arch in x86_64 aarch64 ; do \
    for libc in gnu musl ; do \
      cp compiler/rustc_target/src/spec/targets/${arch}_{unknown,${VENDOR}}_linux_${libc}.rs && \
      sed -i -e '/let mut base = base::linux_'${libc}'::opts();/a base.vendor = "'${VENDOR}'".into();' \
        compiler/rustc_target/src/spec/targets/${arch}_${VENDOR}_linux_${libc}.rs && \
      sed -i -e '/ \.\.base::linux_'${libc}'::opts()/i vendor: "'${VENDOR}'".into(),' \
        compiler/rustc_target/src/spec/targets/${arch}_${VENDOR}_linux_${libc}.rs && \
      sed -i -e '/("'${arch}-unknown-linux-${libc}'", .*),/a("'${arch}-${VENDOR}-linux-${libc}'", '${arch}_${VENDOR}_linux_${libc}'),' \
        compiler/rustc_target/src/spec/mod.rs ; \
    done ; \
  done && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/mod.rs && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/targets/x86_64_${VENDOR}_linux_gnu.rs && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/targets/x86_64_${VENDOR}_linux_musl.rs && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/targets/aarch64_${VENDOR}_linux_gnu.rs && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/targets/aarch64_${VENDOR}_linux_musl.rs

# In addition to our vendor-specific targets, we also need to build for the host
# platform, since that is no longer done implicitly.
COPY ./configs/rust/* ./
RUN \
  sed -e "s,@HOST_TRIPLE@,${HOST_ARCH}-unknown-linux-gnu,g" config.toml.in > config.toml && \
  RUSTUP_DIST_SERVER=example:// python3 ./x.py install

RUN \
  install -p -m 0644 -Dt licenses COPYRIGHT LICENSE-*

# Set appropriate environment for using this Rust compiler to build tools
ENV PATH="/usr/libexec/rust/bin:$PATH" LD_LIBRARY_PATH="/usr/libexec/rust/lib"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-bootconfig

USER root

ENV KVER="5.10.162"

RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/bootconfig && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/bootconfig

USER builder
WORKDIR /home/builder
COPY ./hashes/kernel /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar -xf linux-${KVER}.tar.xz && rm linux-${KVER}.tar.xz

WORKDIR /home/builder/linux-${KVER}
RUN \
  cp -p COPYING LICENSES/preferred/GPL-2.0 /usr/share/licenses/bootconfig
RUN \
  make -C tools/bootconfig && \
  cp tools/bootconfig/bootconfig /usr/libexec/tools/bootconfig

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libc as sdk-go-prep

ENV GOVER="1.22.1"
ENV AWS_LC_FIPS_VER="2.0.9"

USER root
RUN dnf -y install golang

USER builder
WORKDIR /home/builder/sdk-go
COPY ./hashes/go /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar --strip-components=1 -xf go${GOVER}.src.tar.gz && \
  rm go${GOVER}.src.tar.gz

# Patch Go sources so that they work with AWS-LC as the crypto implementation.
# Note that this will break use of `GOEXPERIMENT=boringcrypto` when using the
# default syso files that ship with Go, since the functions and data structures
# will no longer match. We build the replacement AWS-LC syso files below.
COPY patches/go/* ./
RUN \
  git init && \
  git apply --whitespace=nowarn *.patch

# We need to build AWS-LC before we can build Go.
WORKDIR /home/builder/aws-lc
COPY ./hashes/aws-lc /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar --strip-components=1 -xf AWS-LC-FIPS-${AWS_LC_FIPS_VER}.tar.gz && \
  rm AWS-LC-FIPS-${AWS_LC_FIPS_VER}.tar.gz

# Patch AWS-LC sources to avoid weak symbols for memory management functions
# when GOBORING is defined.
COPY patches/aws-lc/* ./
RUN \
  git init && \
  git apply --whitespace=nowarn *.patch

# Set up the environment for building.
ENV GOOS="linux"
ENV CGO_ENABLED=1
ENV CFLAGS="-O2 -g -pipe -Wformat -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection"
ENV CXXFLAGS="${CFLAGS}"
ENV LDFLAGS="-Wl,-z,relro -Wl,-z,now"
ENV CGO_CFLAGS="${CFLAGS}"
ENV CGO_CXXFLAGS="${CXXFLAGS}"
ENV CGO_LDFLAGS="${LDFLAGS}"

WORKDIR /home/builder/aws-lc/build
COPY ./configs/aws-lc/* .
COPY ./helpers/aws-lc/* .

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-prep as sdk-go-aws-lc-x86_64
ENV ARCH="x86_64"
RUN ./build-aws-lc.sh --arch="${ARCH}" --go-dir="${HOME}/sdk-go"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-prep as sdk-go-aws-lc-aarch64
ENV ARCH="aarch64"
RUN ./build-aws-lc.sh --arch="${ARCH}" --go-dir="${HOME}/sdk-go"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-prep as sdk-go

COPY --from=sdk-go-aws-lc-x86_64 \
  /home/builder/aws-lc/build/goboringcrypto_linux_amd64.syso \
  /home/builder/sdk-go/src/crypto/internal/boring/syso/goboringcrypto_linux_amd64.syso

COPY --from=sdk-go-aws-lc-aarch64 \
  /home/builder/aws-lc/build/goboringcrypto_linux_arm64.syso \
  /home/builder/sdk-go/src/crypto/internal/boring/syso/goboringcrypto_linux_arm64.syso

# Build Go - finally!
ENV GOROOT_FINAL="/usr/libexec/go"
WORKDIR /home/builder/sdk-go/src
RUN ./all.bash

# Install the Go standard library and toolchain.
WORKDIR /home/builder/sdk-go
ENV PATH="/home/builder/sdk-go/bin:${PATH}" GO111MODULE="auto"
RUN \
  go install -buildmode=pie std cmd && \
  install -p -m 0644 -Dt licenses LICENSE PATENTS

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-rust as sdk-cargo
USER builder

# Cache crates.io index here to avoid repeated downloads if a build fails.
RUN cargo install lazy_static ||:

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-rust as rust-sources

# Copy the sources without clarify.toml or deny.toml, so that validation failures
# don't require a full rebuild from source every time those files are modified.
COPY license-scan /license-scan
COPY license-tool /license-tool

USER root
RUN rm /license-{scan,tool}/{clarify,deny}.toml

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo as sdk-license-scan

ENV SPDXVER="3.19"

USER builder
WORKDIR /home/builder/license-scan
COPY ./hashes/license-scan ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf license-list-data-${SPDXVER}.tar.gz license-list-data-${SPDXVER}/json/details && \
  rm license-list-data-${SPDXVER}.tar.gz && \
  mv license-list-data-${SPDXVER} license-list-data

COPY --from=rust-sources /license-scan /home/builder/license-scan
RUN cargo build --release --locked

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo as sdk-license-tool

USER builder
WORKDIR /home/builder/license-tool
COPY --from=rust-sources license-tool .
RUN cargo build --release --locked

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo as sdk-cargo-deny

ENV DENYVER="0.14.20"

USER builder
WORKDIR /home/builder
COPY ./hashes/cargo-deny ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf cargo-deny-${DENYVER}.tar.gz && \
  rm cargo-deny-${DENYVER}.tar.gz && \
  mv cargo-deny-${DENYVER} cargo-deny

WORKDIR /home/builder/cargo-deny
RUN cargo build --release --locked

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo as sdk-cargo-make

ENV MAKEVER="0.36.8"

USER builder
WORKDIR /home/builder
COPY ./hashes/cargo-make ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf cargo-make-${MAKEVER}.tar.gz && \
  rm cargo-make-${MAKEVER}.tar.gz && \
  mv cargo-make-${MAKEVER} cargo-make

WORKDIR /home/builder/cargo-make
RUN cargo build --release --locked

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo as sdk-rust-tools

# Bring it all back together and run license-scan and cargo-deny on everything.

COPY --from=sdk-cargo-deny \
  /home/builder/cargo-deny \
  /home/builder/cargo-deny

COPY --from=sdk-cargo-make \
  /home/builder/cargo-make \
  /home/builder/cargo-make

COPY --from=sdk-license-tool \
  /home/builder/license-tool \
  /home/builder/license-tool

COPY --from=sdk-license-scan \
  /home/builder/license-scan \
  /home/builder/license-scan

COPY --chown=0:0 --from=sdk-cargo-deny \
  /home/builder/cargo-deny/target/release/cargo-deny \
  /usr/libexec/tools/

COPY --chown=0:0 --from=sdk-cargo-make \
  /home/builder/cargo-make/target/release/cargo-make \
  /usr/libexec/tools/

COPY --chown=0:0 --from=sdk-license-tool \
  /home/builder/license-tool/target/release/bottlerocket-license-tool \
  /usr/libexec/tools/

COPY --chown=0:0 --from=sdk-license-scan \
  /home/builder/license-scan/target/release/bottlerocket-license-scan \
  /usr/libexec/tools/

COPY --chown=0:0 --from=sdk-license-scan \
  /home/builder/license-scan/license-list-data/json/details \
  /usr/libexec/tools/spdx-data

COPY --chown=1000:1000 --from=sdk-cargo-deny \
  /home/builder/cargo-deny/LICENSE-* \
  /usr/share/licenses/cargo-deny/

COPY --chown=1000:1000 --from=sdk-cargo-make \
  /home/builder/cargo-make/LICENSE \
  /usr/share/licenses/cargo-make/

COPY --chown=1000:1000 \
  COPYRIGHT LICENSE-APACHE LICENSE-MIT \
  /usr/share/licenses/bottlerocket-license-tool/

COPY --chown=1000:1000 \
  COPYRIGHT LICENSE-APACHE LICENSE-MIT \
  /usr/share/licenses/bottlerocket-license-scan/

WORKDIR /home/builder/cargo-deny
COPY ./configs/cargo-deny/clarify.toml .
RUN \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/cargo-deny/vendor \
    cargo --locked Cargo.toml

COPY ./configs/cargo-deny/deny.toml .
RUN \
  /usr/libexec/tools/cargo-deny \
    --all-features check --disable-fetch licenses bans sources

WORKDIR /home/builder/cargo-make
COPY ./configs/cargo-make/clarify.toml .
RUN \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/cargo-make/vendor \
    cargo --locked Cargo.toml

COPY ./configs/cargo-make/deny.toml .
RUN \
  /usr/libexec/tools/cargo-deny \
    --all-features check --disable-fetch licenses bans sources

WORKDIR /home/builder/license-tool
COPY license-tool/clarify.toml .
RUN \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/bottlerocket-license-tool/vendor \
    cargo --locked Cargo.toml

COPY license-tool/deny.toml .
RUN \
  /usr/libexec/tools/cargo-deny \
    --all-features check --disable-fetch licenses bans sources

WORKDIR /home/builder/license-scan
COPY license-scan/clarify.toml .
RUN \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/bottlerocket-license-scan/vendor \
    cargo --locked Cargo.toml

COPY license-scan/deny.toml .
RUN \
  /usr/libexec/tools/cargo-deny \
    --all-features check --disable-fetch licenses bans sources

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go as sdk-govc

USER root
RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/govmomi && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/govmomi

ENV GOVMOMIVER="0.30.2"
ENV GOVMOMISHORTCOMMIT="9078b0b"
ENV GOVMOMIDATE="2023-02-01T04:38:23Z"

USER builder
WORKDIR /home/builder/go/src/github.com/vmware/govmomi
COPY ./hashes/govmomi /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar --strip-components=1 -xf govmomi-${GOVMOMIVER}.tar.gz && \
  rm govmomi-${GOVMOMIVER}.tar.gz

COPY --chown=0:0 --from=sdk-rust-tools /usr/libexec/tools/ /usr/libexec/tools/
RUN \
  cp -p LICENSE.txt /usr/share/licenses/govmomi && \
  go mod vendor && \
  /usr/libexec/tools/bottlerocket-license-scan \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/govmomi/vendor \
    go-vendor ./vendor

RUN \
  export CGO_ENABLED=0 ; \
  export BUILD_VERSION_PKG="github.com/vmware/govmomi/govc/flags" ; \
  go build -mod=vendor -o /usr/libexec/tools/govc -ldflags " \
    -s -w \
    -X ${BUILD_VERSION_PKG}.BuildVersion=${GOVMOMIVER} \
    -X ${BUILD_VERSION_PKG}.BuildCommit=${GOVMOMISHORTCOMMIT} \
    -X ${BUILD_VERSION_PKG}.BuildDate=${GOVMOMIDATE} \
    " github.com/vmware/govmomi/govc

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go as sdk-docker

USER root
RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/docker && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/docker

ENV DOCKERVER="20.10.21"
ENV DOCKERCOMMIT="baeda1f82a10204ec5708d5fbba130ad76cfee49"
ENV DOCKERIMPORT="github.com/docker/cli"
ENV MOBYBIRTHDAY="2017-04-18T14:29:00.000000000+00:00"

USER builder
WORKDIR /home/builder/go/src/${DOCKERIMPORT}
COPY ./hashes/docker /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar --strip-components=1 -xf cli-${DOCKERVER}.tar.gz && \
  rm cli-${DOCKERVER}.tar.gz

COPY --chown=0:0 --from=sdk-rust-tools /usr/libexec/tools/ /usr/libexec/tools/
COPY ./configs/docker/clarify.toml .
RUN \
  cp -p LICENSE NOTICE /usr/share/licenses/docker && \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/docker/vendor \
    go-vendor ./vendor

RUN \
  export CGO_ENABLED=0 ; \
  go build -o /usr/libexec/tools/docker -ldflags " \
    -s -w \
    -X github.com/docker/cli/cli/version.Version=${DOCKERVER} \
    -X github.com/docker/cli/cli/version.GitCommit=${DOCKERCOMMIT} \
    -X github.com/docker/cli/cli/version.BuildTime=${MOBYBIRTHDAY} \
    -X \"github.com/docker/cli/cli/version.PlatformName=Docker Engine - Community\" \
    " ${DOCKERIMPORT}/cmd/docker

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-cpp

ENV AWS_SDK_CPP_VER="1.11.207"

USER builder
WORKDIR /home/builder/aws-sdk-cpp-src
COPY ./hashes/aws-sdk-cpp /home/builder/aws-sdk-cpp-src/hashes

# Upstream source fallback is explicitly disabled here as the SHA512 hash
# verification fails due to a difference in the upstream names and the SDK's.
RUN \
  UPSTREAM_SOURCE_FALLBACK=false sdk-fetch hashes && \
  tar --strip-components=1 -xf aws-sdk-cpp-${AWS_SDK_CPP_VER}.tar.gz && \
  rm aws-sdk-cpp-${AWS_SDK_CPP_VER}.tar.gz && \
  install -p -m 0644 -D -t \
    licenses/aws-sdk-cpp-${AWS_SDK_CPP_VER} \
    LICENSE {LICENSE,NOTICE}.txt && \
  tar -C crt/aws-crt-cpp --strip-components=1 -xf aws-crt-cpp.tar.gz && \
  rm aws-crt-cpp.tar.gz && \
  install -p -m 0644 -D -t \
    licenses/aws-sdk-cpp-${AWS_SDK_CPP_VER}/crt \
    crt/aws-crt-cpp/{LICENSE,NOTICE}

RUN \
  for tar in *.tar.gz ; do \
    dir="${tar%%.*}" && \
    tar -C crt/aws-crt-cpp/crt/${dir} --strip-components=1 -xf ${tar} && \
    licenses="$(\
      cd crt/aws-crt-cpp && \
      find crt/${dir} -type f \
        \( -iname '*LICENSE*' -o -iname '*NOTICE*' \) \
        ! -iname '*.cpp' ! -iname '*.h' ! -iname '*.json' \
        ! -iname '*.go' ! -iname '*.yml' ! -path '*tests*' )" && \
    for license in ${licenses} ; do \
      licensedir="licenses/aws-sdk-cpp-${AWS_SDK_CPP_VER}/${license%/*}" && \
      mkdir -p "${licensedir}" && \
      install -p -m 0644 "crt/aws-crt-cpp/${license}" "${licensedir}" ; \
    done ; \
  done && \
  rm *.tar.gz

WORKDIR /home/builder/aws-sdk-cpp-src/build
RUN \
  cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_ONLY=kms \
    -DENABLE_TESTING=OFF \
    -DCMAKE_INSTALL_PREFIX=/home/builder/aws-sdk-cpp \
    -DBUILD_SHARED_LIBS=OFF && \
  make && \
  make install

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cpp as sdk-aws-kms-pkcs11

ENV AWS_KMS_PKCS11_VER="0.0.9"

USER builder
WORKDIR /home/builder/aws-kms-pkcs11
COPY ./hashes/aws-kms-pkcs11 ./hashes
RUN \
  sdk-fetch hashes && \
  tar --strip-components=1 -xf aws-kms-pkcs11-${AWS_KMS_PKCS11_VER}.tar.gz && \
  rm aws-kms-pkcs11-${AWS_KMS_PKCS11_VER}.tar.gz

ENV AWS_SDK_PATH="/home/builder/aws-sdk-cpp"
RUN make

USER root
RUN make install

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-e2fsprogs

ENV E2FSPROGS_VER="1.46.6"

USER builder
WORKDIR /home/builder
COPY ./hashes/e2fsprogs /home/builder/hashes
RUN \
  sdk-fetch hashes && \
  tar --strip-components=1 -xf e2fsprogs-${E2FSPROGS_VER}.tar.xz && \
  rm e2fsprogs-${E2FSPROGS_VER}.tar.xz

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-plus

# Install any host tools that we don't need to build the software above, but
# that we want in the final SDK. This happens in a separate stage so we don't
# have to rebuild Rust every time we add new packages.
USER root
RUN \
  dnf -y install --setopt=install_weak_deps=False \
    ccache \
    createrepo_c \
    dosfstools \
    e2fsprogs \
    efitools \
    erofs-utils \
    gdisk \
    glibc \
    glib2-devel \
    gnupg-pkcs11-scd \
    gnutls-utils \
    groff \
    kpartx \
    less \
    libcap-devel \
    lz4 \
    mtools \
    nss-tools \
    openssl-pkcs11 \
    pesign \
    policycoreutils \
    protobuf-compiler \
    protobuf-devel \
    python3-jinja2 \
    python3-virt-firmware \
    qemu-img \
    rpcgen \
    rpmdevtools \
    sbsigntools \
    secilc \
    ShellCheck \
    squashfs-tools \
    unzip \
    veritysetup \
    xfsprogs \
  && \
  dnf -y remove awscli && \
  dnf clean all

ARG HOST_ARCH
ENV AWSCLI_VER="2.14.6"

USER builder
WORKDIR /home/builder/awscli
COPY ./hashes/awscli /home/builder/awscli/hashes
RUN \
  sdk-fetch hashes && \
  unzip awscli-exe-linux-${HOST_ARCH}-${AWSCLI_VER}.zip && \
  rm awscli-exe-linux-*-${AWSCLI_VER}.zip

USER root

RUN \
  ./aws/install && \
  install -p -m 0644 -D -t \
    /usr/share/licenses/awscli-${AWSCLI_VER} \
    aws/THIRD_PARTY_LICENSES && \
  rm -rf /home/builder

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as toolchain-archive

ENV MUSL_TARGET_x86_64="x86_64-bottlerocket-linux-musl"
ENV MUSL_TARGET_aarch64="aarch64-bottlerocket-linux-musl"

COPY --from=toolchain-musl-x86_64 \
  /home/builder/buildroot/output/x86_64-musl/build/toolchain-x86_64.txt \
  /tmp/toolchain-x86_64.txt

COPY --from=toolchain-musl-x86_64 \
  /home/builder/buildroot/output/x86_64-musl/build/toolchain-licenses-x86_64.txt \
  /tmp/toolchain-licenses-x86_64.txt

COPY --from=toolchain-musl-aarch64 \
  /home/builder/buildroot/output/aarch64-musl/build/toolchain-aarch64.txt \
  /tmp/toolchain-aarch64.txt

COPY --from=toolchain-musl-aarch64 \
  /home/builder/buildroot/output/aarch64-musl/build/toolchain-licenses-aarch64.txt \
  /tmp/toolchain-licenses-aarch64.txt

WORKDIR /tmp

RUN \
  tar cvf toolchain.tar \
    --transform "s,^,toolchain/," \
    -C / \
    -T toolchain-x86_64.txt && \
  tar rvf toolchain.tar \
    --transform "s,^,toolchain/licenses/," \
    -C "/${MUSL_TARGET_x86_64}/sys-root/usr/share/licenses" \
    -T toolchain-licenses-x86_64.txt && \
  tar rvf toolchain.tar \
    --transform "s,^,toolchain/," \
    -C / \
    -T toolchain-aarch64.txt && \
  tar rvf toolchain.tar \
    --transform "s,^,toolchain/licenses/," \
    -C "/${MUSL_TARGET_aarch64}/sys-root/usr/share/licenses" \
    -T toolchain-licenses-aarch64.txt && \
  tar xvf toolchain.tar -C /

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=
#
# Generate macros for the target.

FROM sdk as sdk-macros

COPY macros/* /tmp/

WORKDIR /tmp
RUN \
  for arch in x86_64 aarch64 ; do \
    platform_dir="/usr/lib/rpm/platform/${arch}-bottlerocket" ; \
    mkdir -p "${platform_dir}" ; \
    cat ${arch} shared rust cargo > "${platform_dir}/macros" ; \
  done && \
  vendor_dir="/usr/lib/rpm/bottlerocket" && \
  mkdir -p "${vendor_dir}" && \
  cp -a check-fips "${vendor_dir}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=
#
# Create symlinks that can be added to $PATH to override programs invoked by
# find-debuginfo.sh, which does not expect to add a prefix.
FROM sdk as sdk-find-debuginfo-symlinks
RUN \
  for arch in x86_64 aarch64 ; do \
    triple="${arch}-bottlerocket-linux-gnu" ; \
    debuginfo_bindir="/usr/${triple}/debuginfo/bin" ; \
    mkdir -p "${debuginfo_bindir}" ; \
    for b in nm objcopy objdump strip ; do \
      ln -sr "/usr/bin/${triple}-${b}" "${debuginfo_bindir}/${b}" ; \
    done ; \
  done

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=
#
# Collect all SDK builds
FROM scratch as sdk-final
USER root

WORKDIR /
# "sdk-plus" has our C/C++ toolchain and kernel headers for both targets, and
# any other host programs we want available for OS builds.
COPY --from=sdk-plus / /

# "toolchain-archive" has the toolchains for both targets bundled together in
# a format that's convenient for extracting later.
COPY --from=toolchain-archive /toolchain /toolchain

# "sdk-libc-musl" has the musl C library and headers. We omit "sdk-libc-gnu"
# because we expect to build glibc again for the target OS, while we will use
# the musl artifacts directly to generate static binaries such as migrations.
COPY --from=sdk-libc-musl / /

# "sdk-rust" has our Rust toolchain with the required targets.
COPY --chown=0:0 --from=sdk-rust /usr/libexec/rust/ /usr/libexec/rust/
COPY --chown=0:0 --from=sdk-rust \
  /home/builder/rust/licenses/ \
  /usr/share/licenses/rust/

# "sdk-go" has the Go toolchain and standard library builds.
COPY --chown=0:0 --from=sdk-go /home/builder/sdk-go/bin /usr/libexec/go/bin/
COPY --chown=0:0 --from=sdk-go /home/builder/sdk-go/lib /usr/libexec/go/lib/
COPY --chown=0:0 --from=sdk-go /home/builder/sdk-go/pkg /usr/libexec/go/pkg/
COPY --chown=0:0 --from=sdk-go /home/builder/sdk-go/src /usr/libexec/go/src/
COPY --chown=0:0 --from=sdk-go /home/builder/sdk-go/go.env /usr/libexec/go/go.env
COPY --chown=0:0 --from=sdk-go \
  /home/builder/sdk-go/licenses/ \
  /usr/share/licenses/go/
COPY --chown=0:0 --from=sdk-go \
  /home/builder/aws-lc/LICENSE \
  /usr/share/licenses/aws-lc/LICENSE

# "sdk-rust-tools" has our attribution generation and license scan tools.
COPY --chown=0:0 --from=sdk-rust-tools /usr/libexec/tools/ /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-rust-tools /usr/share/licenses/bottlerocket-license-scan/ /usr/share/licenses/bottlerocket-license-scan/
COPY --chown=0:0 --from=sdk-rust-tools /usr/share/licenses/bottlerocket-license-tool/ /usr/share/licenses/bottlerocket-license-tool/
COPY --chown=0:0 --from=sdk-rust-tools /usr/share/licenses/cargo-deny/ /usr/share/licenses/cargo-deny/
COPY --chown=0:0 --from=sdk-rust-tools /usr/share/licenses/cargo-make/ /usr/share/licenses/cargo-make/

# "sdk-govc" has the VMware govc tool and licenses.
COPY --chown=0:0 --from=sdk-govc /usr/libexec/tools/govc /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-govc /usr/share/licenses/govmomi/ /usr/share/licenses/govmomi/

# "sdk-docker" has the Docker CLI and licenses.
COPY --chown=0:0 --from=sdk-docker /usr/libexec/tools/docker /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-docker /usr/share/licenses/docker/ /usr/share/licenses/docker/

# "sdk-bootconfig" has the bootconfig tool
COPY --chown=0:0 --from=sdk-bootconfig /usr/libexec/tools/bootconfig /usr/libexec/tools/bootconfig
COPY --chown=0:0 --from=sdk-bootconfig /usr/share/licenses/bootconfig /usr/share/licenses/bootconfig

# "sdk-aws-kms-pkcs11" has the PKCS#11 provider for an AWS KMS backend
COPY --chown=0:0 --from=sdk-aws-kms-pkcs11 \
  /usr/lib64/pkcs11/aws_kms_pkcs11.so \
  /usr/lib64/pkcs11/

COPY --chown=0:0 --from=sdk-aws-kms-pkcs11 \
  /home/builder/aws-kms-pkcs11/LICENSE \
  /usr/share/licenses/aws-kms-pkcs11/

# Also include the licenses from the AWS SDK for C++, since those are
# statically linked into the provider.
COPY --chown=0:0 --from=sdk-cpp \
  /home/builder/aws-sdk-cpp-src/licenses/ \
  /usr/share/licenses/aws-kms-pkcs11/vendor/

# Configure p11-kit to use the provider.
COPY --chown=0:0 \
  ./configs/aws-kms-pkcs11/aws-kms-pkcs11.module \
  /etc/pkcs11/modules/

# Configure gpg to use the provider.
COPY --chown=0:0 \
  ./configs/gnupg/gpg-agent.conf \
  /etc/gnupg/gpg-agent.conf

COPY --chown=0:0 \
  ./configs/gnupg/gnupg-pkcs11-scd.conf \
  /etc/gnupg-pkcs11-scd.conf

# "sdk-e2fsprogs" has the dir2fs tool
COPY --chown=0:0 --from=sdk-e2fsprogs \
  /home/builder/contrib/dir2fs \
  /usr/local/bin/dir2fs

COPY --chown=0:0 --from=sdk-e2fsprogs \
  /home/builder/NOTICE \
  /usr/share/licenses/dir2fs/

# "sdk-macros" has the rpm macros
COPY --chown=0:0 --from=sdk-macros \
  /usr/lib/rpm/platform/x86_64-bottlerocket/ \
  /usr/lib/rpm/platform/x86_64-bottlerocket/

COPY --chown=0:0 --from=sdk-macros \
  /usr/lib/rpm/platform/aarch64-bottlerocket/ \
  /usr/lib/rpm/platform/aarch64-bottlerocket/

COPY --chown=0:0 --from=sdk-macros \
  /usr/lib/rpm/bottlerocket/check-fips \
  /usr/lib/rpm/bottlerocket/check-fips

COPY --chown=0:0 --from=sdk-find-debuginfo-symlinks \
  /usr/x86_64-bottlerocket-linux-gnu/debuginfo/bin/ \
  /usr/x86_64-bottlerocket-linux-gnu/debuginfo/bin/

COPY --chown=0:0 --from=sdk-find-debuginfo-symlinks \
  /usr/aarch64-bottlerocket-linux-gnu/debuginfo/bin/ \
  /usr/aarch64-bottlerocket-linux-gnu/debuginfo/bin/

# Add Rust programs and libraries to the path.
# Also add symlinks to help out with sysroot discovery.
RUN \
  for b in /usr/libexec/rust/bin/* ; do \
    ln -s ../libexec/rust/bin/${b##*/} /usr/bin/${b##*/} ; \
  done && \
  echo '/usr/libexec/rust/lib' > /etc/ld.so.conf.d/rust.conf && \
  ldconfig && \
  for d in /usr/lib64 /usr/lib ; do \
    ln -s ../libexec/rust/lib/rustlib ${d}/rustlib ; \
  done

# Add Go programs to $PATH and sync timestamps to avoid rebuilds.
RUN \
  ln -s ../libexec/go/bin/go /usr/bin/go && \
  ln -s ../libexec/go/bin/gofmt /usr/bin/gofmt && \
  find /usr/libexec/go -type f -exec touch -r /usr/libexec/go/bin/go {} \+

# Strip and add tools to the path.
RUN \
  for b in /usr/libexec/tools/* ; do \
    strip -g $b ; \
    ln -s ../libexec/tools/${b##*/} /usr/bin/${b##*/} ; \
  done

# Make the licenses in the sys-roots easier to find.
RUN \
  ln -sr /x86_64-bottlerocket-linux-gnu/sys-root/usr/share/licenses /usr/share/licenses/bottlerocket-sdk-gnu-x86_64 && \
  ln -sr /x86_64-bottlerocket-linux-musl/sys-root/usr/share/licenses /usr/share/licenses/bottlerocket-sdk-musl-x86_64 && \
  ln -sr /aarch64-bottlerocket-linux-gnu/sys-root/usr/share/licenses /usr/share/licenses/bottlerocket-sdk-gnu-aarch64 && \
  ln -sr /aarch64-bottlerocket-linux-musl/sys-root/usr/share/licenses /usr/share/licenses/bottlerocket-sdk-musl-aarch64

# Configure the Docker CLI.
COPY \
  ./configs/docker/docker-cli.json \
  /home/builder/.docker/config.json

# Reset permissions for `builder`.
RUN chown builder:builder -R /home/builder

USER builder
RUN rpmdev-setuptree

# Create an empty "certdb" for signing.
WORKDIR /home/builder
RUN \
  mkdir .netscape && \
  certutil -N --empty-password

# Disable cargo make update checks for invocations within the SDK.
RUN \
  echo "export CARGO_MAKE_DISABLE_UPDATE_CHECK=1" >> .bashrc

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Collect all builds for the SDK and squashes them into a final, single layer
FROM scratch as sdk-golden

COPY --from=sdk-final / /

# The `builder` user is setup in the "final" layer and is used in place of the
# default `root` user
USER builder
WORKDIR /home/builder

CMD ["/bin/bash"]
