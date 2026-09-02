FROM public.ecr.aws/docker/library/fedora:43 AS base

# Everything we need to build our SDK and packages.
RUN \
  dnf config-manager setopt fedora-cisco-openh264.enabled=0 && \
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
    openssl-devel-engine \
    p11-kit-devel \
    perl-ExtUtils-MakeMaker \
    perl-FindBin \
    perl-IPC-Cmd \
    perl-open \
    python \
    systemd-ukify \
    rsync \
    wget \
    which \
  && \
  useradd builder
COPY ./sdk-fetch /usr/local/bin

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# We expect our C cross-compiler to be used on other distros for building kernel
# modules, so we build it with an older glibc for compatibility.
FROM public.ecr.aws/docker/library/ubuntu:16.04 AS compat
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

FROM compat AS toolchain
USER builder

# Configure Git for any subsequent use.
RUN \
  git config --global user.name "Builder" && \
  git config --global user.email "builder@localhost"

ARG UPSTREAM_SOURCE_FALLBACK
ENV BRVER="2025.05.1"
ENV KVER="6.1.147"

WORKDIR /home/builder
COPY ./hashes/buildroot ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf buildroot-${BRVER}.tar.xz && \
  rm buildroot-${BRVER}.tar.xz && \
  mv buildroot-${BRVER} buildroot && \
  mkdir musl-compat-headers && \
  mv queue.h musl-compat-headers

WORKDIR /home/builder/buildroot
COPY ./patches/buildroot/* ./
COPY ./configs/buildroot/* ./configs/
COPY ./helpers/buildroot/* ./
RUN \
  git init . && \
  git apply --whitespace=nowarn *.patch

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain AS toolchain-gnu-x86_64
ENV ARCH="x86_64"
RUN ./build-gnu-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain AS toolchain-gnu-aarch64
ENV ARCH="aarch64"
RUN ./build-gnu-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain AS toolchain-musl-x86_64
ENV ARCH="x86_64"
RUN ./build-musl-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain AS toolchain-musl-aarch64
ENV ARCH="aarch64"
RUN ./build-musl-toolchain.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Add our cross-compilers to the base SDK layer.
FROM base AS sdk
USER root

ARG UPSTREAM_SOURCE_FALLBACK
ENV KVER="6.1.147"

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
FROM sdk AS sdk-gnu
USER builder

WORKDIR /home/builder
COPY ./hashes/glibc ./hashes
COPY ./helpers/glibc/* ./

ENV GLIBCVER="2.40"
ENV KVER="6.1.147"
RUN \
  sdk-fetch hashes && \
  tar xf glibc-${GLIBCVER}.tar.xz && \
  rm glibc-${GLIBCVER}.tar.xz && \
  mv glibc-${GLIBCVER} glibc && \
  cd glibc && \
  mkdir build

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-gnu AS sdk-gnu-x86_64
ENV ARCH="x86_64"
RUN ./build-glibc.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-gnu AS sdk-gnu-aarch64
ENV ARCH="aarch64"
RUN ./build-glibc.sh --arch="${ARCH}" --kernel-version="${KVER}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk AS sdk-musl
USER builder

WORKDIR /home/builder
COPY ./hashes/musl ./hashes
COPY ./patches/musl ./patches
COPY ./helpers/musl/* ./

ENV MUSLVER="1.2.5"
RUN \
  sdk-fetch hashes && \
  tar xf musl-${MUSLVER}.tar.gz && \
  rm musl-${MUSLVER}.tar.gz && \
  mv musl-${MUSLVER} musl && \
  cd musl && \
  git init . && \
  git apply --whitespace=nowarn ../patches/*.patch

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-musl AS sdk-musl-x86_64
ENV ARCH="x86_64"
RUN ./build-musl.sh --arch="${ARCH}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-musl AS sdk-musl-aarch64
ENV ARCH="aarch64"
RUN ./build-musl.sh --arch="${ARCH}"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM scratch AS sdk-libc-gnu

ENV GNU_TARGET_x86_64="x86_64-bottlerocket-linux-gnu"
ENV GNU_TARGET_aarch64="aarch64-bottlerocket-linux-gnu"

COPY --chown=0:0 --from=sdk-gnu-x86_64 \
  /home/builder/glibc/output/${GNU_TARGET_x86_64}/sys-root/ \
  /${GNU_TARGET_x86_64}/sys-root/

COPY --chown=0:0 --from=sdk-gnu-aarch64 \
  /home/builder/glibc/output/${GNU_TARGET_aarch64}/sys-root/ \
  /${GNU_TARGET_aarch64}/sys-root/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM scratch AS sdk-libc-musl
ENV MUSL_TARGET_x86_64="x86_64-bottlerocket-linux-musl"
ENV MUSL_TARGET_aarch64="aarch64-bottlerocket-linux-musl"

COPY --chown=0:0 --from=sdk-musl-x86_64 \
  /home/builder/musl/output/${MUSL_TARGET_x86_64}/sys-root/ \
  /${MUSL_TARGET_x86_64}/sys-root/

COPY --chown=0:0 --from=sdk-musl-aarch64 \
  /home/builder/musl/output/${MUSL_TARGET_aarch64}/sys-root/ \
  /${MUSL_TARGET_aarch64}/sys-root/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk AS sdk-libc

COPY --from=sdk-libc-gnu / /
COPY --from=sdk-libc-musl / /

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libc AS sdk-rust

USER root
RUN \
  mkdir -p /usr/libexec/{rust,llvm} && \
  chown -R builder:builder /usr/libexec/{rust,llvm}

ARG HOST_ARCH
ENV VENDOR="bottlerocket"
ENV RUSTVER="1.97.1"

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
  dir=build/cache/$(awk -F= '/^compiler_date/{print $2}' src/stage0); \
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
# These vendor targets are effectively the same as the "unknown" targets.
COPY ./configs/rust/targets ./targets

# In addition to our vendor-specific targets, we also need to build for the host
# platform, since that is no longer done implicitly.
COPY ./configs/rust/config.toml.in ./
RUN \
  sed -e "s,@HOST_TRIPLE@,${HOST_ARCH}-unknown-linux-gnu,g" config.toml.in > config.toml && \
  RUSTUP_DIST_SERVER=example:// RUST_TARGET_PATH=${PWD}/targets python3 ./x.py install

# Copy target configs into the installed Rust environment.
RUN \
  for arch in x86_64 aarch64 ; do \
    for libc in gnu musl ; do \
      cp \
        targets/${arch}-bottlerocket-linux-${libc}.json \
        /usr/libexec/rust/lib/rustlib/${arch}-bottlerocket-linux-${libc}/target.json ; \
    done ; \
  done

# Copy out the LLVM toolchain that was built along with Rust.
RUN \
  rm -rf "build/${HOST_ARCH}-unknown-linux-gnu/llvm/build" && \
  rsync -aq "build/${HOST_ARCH}-unknown-linux-gnu/llvm/" /usr/libexec/llvm/

RUN \
  install -p -m 0644 -Dt licenses COPYRIGHT LICENSE-*

# Set appropriate environment for using this Rust compiler to build tools
ENV PATH="/usr/libexec/rust/bin:$PATH" LD_LIBRARY_PATH="/usr/libexec/rust/lib"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM scratch AS sdk-llvm

# Copy the LLVM install directly to `/usr`, so clang can auto-discover the
# GCC target toolchains.
COPY --from=sdk-rust /usr/libexec/llvm/ /usr/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk AS sdk-grub

USER root
ARG HOST_ARCH
ENV GRUB_VER="2.06-61.amzn2023.0.9"

RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/grub && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/grub

USER builder
WORKDIR /home/builder
COPY ./hashes/grub /home/builder/hashes
COPY ./patches/grub /home/builder/patches

# This rather elaborate way of unpacking the sources and applying the
# patches mimics the way GRUB is built in Bottlerocket, which in turn
# mimics the way that Amazon Linux and Fedora build it.
RUN \
  sdk-fetch /home/builder/hashes && \
  rpm2cpio "grub2-${GRUB_VER}.src.rpm" \
  | cpio -iu \
     "./grub-${GRUB_VER%%-*}.tar.xz" \
      ./bootstrap ./bootstrap.conf ./gitignore \
      "./gnulib-*.tar.gz" "./*.patch" && \
  rm "grub2-${GRUB_VER}.src.rpm" && \
  mkdir "grub-${GRUB_VER}" && \
  cd "grub-${GRUB_VER}" && \
  tar --strip-components=1 -xof "../grub-${GRUB_VER%%-*}.tar.xz" && \
  rm "../grub-${GRUB_VER%%-*}.tar.xz" && \
  mv ../bootstrap{,.conf} . && \
  mv ../gitignore .gitignore && \
  tar -xof ../gnulib-*.tar.gz && \
  rm ../gnulib-*.tar.gz && \
  mv gnulib-* gnulib && \
  mv unicode/COPYING COPYING.unicode && \
  rm -f configure && \
  git init && \
  git config user.name 'Builder' && \
  git config user.email 'builder@localhost' && \
  git add . && \
  git commit -a -q -m "base" && \
  git am --whitespace=nowarn ../*.patch ../patches/*.patch && \
  rm ../*.patch && \
  rm -r build-aux m4 && \
  ./bootstrap

WORKDIR /home/builder/grub-${GRUB_VER}
RUN \
  cp -p COPYING COPYING.unicode /usr/share/licenses/grub

# We only need the grub-bios-setup tool for the host. However, we can only get
# it by specifying the "i386" target, which the host toolchain may not support.
# Work around this by using our cross-compiling toolchain for x86_64 to build
# any target binaries.
ENV TARGET="x86_64-bottlerocket-linux-gnu"
ENV TARGET_CPP="${TARGET}-gcc -E"
ENV TARGET_CC="${TARGET}-gcc"
ENV TARGET_NM="${TARGET}-nm"
ENV TARGET_OBJCOPY="${TARGET}-objcopy"
ENV TARGET_STRIP="${TARGET}-strip"

RUN \
  ./configure \
    --host="${HOST_ARCH}-redhat-linux" \
    --target="i386" \
    --with-platform="pc" \
    --with-utils=host \
    --disable-grub-mkfont \
    --disable-rpm-sort \
    --disable-werror \
    --enable-efiemu=no \
    --enable-device-mapper=no \
    --enable-libzfs=no && \
  make -j"$(nproc)" && \
  cp -p grub-bios-setup /usr/libexec/tools/grub-bios-setup

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk AS sdk-bootconfig

USER root

ENV KVER="6.1.147"

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

FROM sdk AS sdk-ca-certificates

USER root

ENV CABUNDLEVER="2026-08-13"

RUN \
  mkdir -p /usr/share/bottlerocket/ca-certificates && \
  chown -R builder:builder /usr/share/bottlerocket/ca-certificates

USER builder
WORKDIR /home/builder
COPY ./hashes/ca-certificates ./hashes

RUN \
  sdk-fetch hashes && \
  install -p -m 0644 cacert-${CABUNDLEVER}.pem /usr/share/bottlerocket/ca-certificates/ca-bundle.crt

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libc AS sdk-go-prep

# Set up the environment for building.
ENV GOOS="linux"
ENV CGO_ENABLED=1
ENV CFLAGS="-O2 -g1 -pipe -Wformat -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection -fno-omit-frame-pointer"
ENV CXXFLAGS="${CFLAGS}"
ENV LDFLAGS="-Wl,-z,relro -Wl,-z,now"
ENV CGO_CFLAGS="${CFLAGS}"
ENV CGO_CXXFLAGS="${CXXFLAGS}"
ENV CGO_LDFLAGS="${LDFLAGS}"
ENV GOAMD64="v2"
ENV GOARM64="v8.0,crypto"

ENV GO111MODULE="auto"

USER root
RUN dnf -y install golang

ENV GO125VER="1.25.14"
ENV GO126VER="1.26.8"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-prep AS sdk-go-1.25-prep

ENV GOMAJOR="1.25"

USER builder

WORKDIR /home/builder/sdk-go

COPY ./hashes/go-${GOMAJOR} /home/builder/hashes-go
COPY ./helpers/go/prep-go.sh ./
COPY ./patches/go-${GOMAJOR} /home/builder/patches-go

RUN ./prep-go.sh --go-version=${GO125VER}

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-prep AS sdk-go-1.26-prep

ENV GOMAJOR="1.26"

USER builder

WORKDIR /home/builder/sdk-go

COPY ./hashes/go-${GOMAJOR} /home/builder/hashes-go
COPY ./helpers/go/prep-go.sh ./
COPY ./patches/go-${GOMAJOR} /home/builder/patches-go

RUN ./prep-go.sh --go-version=${GO126VER}

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-1.25-prep AS sdk-go-1.25

COPY ./helpers/go/build-go.sh ./

# Build Go - finally!
RUN ./build-go.sh --go-version=${GO125VER}

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-1.26-prep AS sdk-go-1.26

COPY ./helpers/go/build-go.sh ./

# Build Go - finally!
RUN ./build-go.sh --go-version=${GO126VER}

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-rust AS sdk-cargo
USER builder

# Cache crates.io index here to avoid repeated downloads if a build fails.
RUN cargo install lazy_static ||:

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-rust AS rust-sources

# Copy the sources without clarify.toml or deny.toml, so that validation failures
# don't require a full rebuild from source every time those files are modified.
COPY license-scan /license-scan

USER root
RUN rm /license-scan/{clarify,deny}.toml

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo AS sdk-license-scan

ENV SPDXVER="3.28.0"

USER builder
WORKDIR /home/builder/license-scan
COPY ./hashes/license-scan ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf license-list-data-${SPDXVER}.tar.gz license-list-data-${SPDXVER}/json/details && \
  rm license-list-data-${SPDXVER}.tar.gz && \
  mv license-list-data-${SPDXVER} /home/builder/license-list-data

COPY --from=rust-sources /license-scan /home/builder/license-scan
RUN cargo build --release --locked

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cargo AS sdk-cargo-deny

ENV DENYVER="0.18.1"

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

FROM sdk-cargo AS sdk-rust-tools

# Bring it all back together and run license-scan and cargo-deny on everything.

COPY --from=sdk-cargo-deny \
  /home/builder/cargo-deny \
  /home/builder/cargo-deny

COPY --from=sdk-license-scan \
  /home/builder/license-scan \
  /home/builder/license-scan

COPY --chown=0:0 --from=sdk-cargo-deny \
  /home/builder/cargo-deny/target/release/cargo-deny \
  /usr/libexec/tools/

COPY --chown=0:0 --from=sdk-license-scan \
  /home/builder/license-scan/target/release/bottlerocket-license-scan \
  /usr/libexec/tools/

COPY --chown=0:0 --from=sdk-license-scan \
  /home/builder/license-list-data/json/details \
  /usr/libexec/tools/spdx-data

COPY --chown=1000:1000 --from=sdk-cargo-deny \
  /home/builder/cargo-deny/LICENSE-* \
  /usr/share/licenses/cargo-deny/

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

FROM sdk-go-1.25 AS sdk-govc

USER root
RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/govmomi && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/govmomi

ENV GOVMOMIVER="0.55.1"
ENV GOVMOMISHORTCOMMIT="1e372f1"
ENV GOVMOMIDATE="2026-07-03T14:30:07Z"

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
  go -C govc mod vendor && \
  /usr/libexec/tools/bottlerocket-license-scan \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/govmomi/vendor \
    go-vendor ./govc/vendor

RUN \
  export CGO_ENABLED=0 ; \
  export BUILD_VERSION_PKG="github.com/vmware/govmomi/cli/flags" ; \
  go -C govc build -mod=vendor -o /usr/libexec/tools/govc -ldflags " \
    -s -w \
    -X ${BUILD_VERSION_PKG}.BuildVersion=${GOVMOMIVER} \
    -X ${BUILD_VERSION_PKG}.BuildCommit=${GOVMOMISHORTCOMMIT} \
    -X ${BUILD_VERSION_PKG}.BuildDate=${GOVMOMIDATE} \
    " github.com/vmware/govmomi/govc

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go-1.25 AS sdk-sbomtool

USER root
RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/sbomtool && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/sbomtool

USER builder
WORKDIR /home/builder/sbomtool

COPY ./sbomtool /home/builder/sbomtool

COPY --chown=0:0 --from=sdk-rust-tools /usr/libexec/tools/ /usr/libexec/tools/
RUN \
  cp -p LICENSE-* /usr/share/licenses/sbomtool/ && \
  # Use go work vendor instead of go mod vendor
  /home/builder/sdk-go/bin/go work vendor && \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/sbomtool/vendor \
    go-vendor ./vendor

# Build the sbomtool using the correct Go binary path with workspace mode
RUN \
  /home/builder/sdk-go/bin/go build -o sbomtool ./cmd/sbomtool

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk AS sdk-cpp

ENV AWS_SDK_CPP_VER="1.11.398"

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
    -DBUILD_ONLY="kms;acm-pca" \
    -DENABLE_TESTING=OFF \
    -DCMAKE_INSTALL_PREFIX=/home/builder/aws-sdk-cpp \
    -DBUILD_SHARED_LIBS=OFF && \
  make && \
  make install

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-cpp AS sdk-aws-kms-pkcs11

ENV AWS_KMS_PKCS11_VER="0.0.11"

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

FROM sdk AS sdk-nasm

ENV NASM_VER="2.16.03"

USER builder
WORKDIR /home/builder/nasm
COPY ./hashes/nasm ./hashes
RUN \
  sdk-fetch hashes && \
  tar --strip-components=1 -xf nasm-${NASM_VER}.tar.xz && \
  rm nasm-${NASM_VER}.tar.xz

RUN \
  ./configure && \
  make all -j"$(nproc)"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk AS sdk-plus

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
    erofs-utils \
    flatbuffers-compiler \
    gdisk \
    glibc \
    glib2-devel \
    gnupg-pkcs11-scd \
    gnutls-utils \
    groff \
    kpartx \
    less \
    libbpf-devel \
    libcap-devel \
    libseccomp-devel \
    libkcapi-hmaccalc \
    lz4 \
    mtools \
    nss-tools \
    openssl-pkcs11 \
    pesign \
    policycoreutils \
    protobuf-compiler \
    protobuf-devel \
    python3-devel \
    python3-pyelftools \
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

FROM sdk AS toolchain-archive

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

FROM sdk AS sdk-macros

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
# find-debuginfo adds the directory where it's located as the first entry in $PATH.
# Set up a directory for each architecture containing the unprefixed versions of
# the binutils programs, along with a link to find-debuginfo, so that it will work
# as expected when extracting and stripping debuginfo from binaries built for the
# target architecture.
FROM sdk AS sdk-find-debuginfo-symlinks
RUN \
  for arch in x86_64 aarch64 ; do \
    triple="${arch}-bottlerocket-linux-gnu" ; \
    debuginfo_bindir="/usr/${triple}/debuginfo/bin" ; \
    mkdir -p "${debuginfo_bindir}" ; \
    ln -sr /usr/bin/find-debuginfo "${debuginfo_bindir}/find-debuginfo" ; \
    for b in nm objcopy objdump readelf strip ; do \
      ln -sr "/usr/bin/${triple}-${b}" "${debuginfo_bindir}/${b}" ; \
    done ; \
  done

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=
#
# Collect all SDK builds
FROM scratch AS sdk-final
USER root

WORKDIR /
# "sdk-plus" has our C/C++ toolchain and kernel headers for both targets, and
# any other host programs we want available for OS builds.
COPY --from=sdk-plus / /

# "toolchain-archive" has the toolchains for both targets bundled together in
# a format that's convenient for extracting later.
COPY --from=toolchain-archive /toolchain /toolchain

# "sdk-libc-musl" has the MUSL C library and headers.
COPY --from=sdk-libc-musl / /

# "sdk-libc-gnu" has the GNU C library and headers.
COPY --from=sdk-libc-gnu / /

# "sdk-rust" has our Rust toolchains with the required targets.
COPY --chown=0:0 --from=sdk-rust /usr/libexec/rust/ /usr/libexec/rust/
COPY --chown=0:0 --from=sdk-rust \
  /home/builder/rust/licenses/ \
  /usr/share/licenses/rust/

# "sdk-llvm" has our LLVM toolchain.
COPY --chown=0:0 --from=sdk-llvm / /

# "sdk-go" has the Go toolchain and standard library builds.
COPY --chown=0:0 --from=sdk-go-1.25 /home/builder/sdk-go/bin /usr/libexec/go-1.25/bin/
COPY --chown=0:0 --from=sdk-go-1.25 /home/builder/sdk-go/lib /usr/libexec/go-1.25/lib/
COPY --chown=0:0 --from=sdk-go-1.25 /home/builder/sdk-go/pkg /usr/libexec/go-1.25/pkg/
COPY --chown=0:0 --from=sdk-go-1.25 /home/builder/sdk-go/src /usr/libexec/go-1.25/src/
COPY --chown=0:0 --from=sdk-go-1.25 /home/builder/sdk-go/go.env /usr/libexec/go-1.25/go.env
COPY --chown=0:0 --from=sdk-go-1.25 \
  /home/builder/sdk-go/licenses/ \
  /usr/share/licenses/go-1.25/

COPY --chown=0:0 --from=sdk-go-1.26 /home/builder/sdk-go/bin /usr/libexec/go-1.26/bin/
COPY --chown=0:0 --from=sdk-go-1.26 /home/builder/sdk-go/lib /usr/libexec/go-1.26/lib/
COPY --chown=0:0 --from=sdk-go-1.26 /home/builder/sdk-go/pkg /usr/libexec/go-1.26/pkg/
COPY --chown=0:0 --from=sdk-go-1.26 /home/builder/sdk-go/src /usr/libexec/go-1.26/src/
COPY --chown=0:0 --from=sdk-go-1.26 /home/builder/sdk-go/go.env /usr/libexec/go-1.26/go.env
COPY --chown=0:0 --from=sdk-go-1.26 \
  /home/builder/sdk-go/licenses/ \
  /usr/share/licenses/go-1.26/

# "sdk-rust-tools" has our attribution generation and license scan tools.
COPY --chown=0:0 --from=sdk-rust-tools /usr/libexec/tools/ /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-rust-tools /usr/share/licenses/bottlerocket-license-scan/ /usr/share/licenses/bottlerocket-license-scan/
COPY --chown=0:0 --from=sdk-rust-tools /usr/share/licenses/cargo-deny/ /usr/share/licenses/cargo-deny/

# "sdk-sbomtool" has our SBOM generation tool
COPY --chown=0:0 --from=sdk-sbomtool /home/builder/sbomtool/sbomtool /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-sbomtool /usr/share/licenses/sbomtool/ /usr/share/licenses/sbomtool/

# "sdk-govc" has the VMware govc tool and licenses.
COPY --chown=0:0 --from=sdk-govc /usr/libexec/tools/govc /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-govc /usr/share/licenses/govmomi/ /usr/share/licenses/govmomi/

# "sdk-bootconfig" has the bootconfig tool
COPY --chown=0:0 --from=sdk-bootconfig /usr/libexec/tools/bootconfig /usr/libexec/tools/bootconfig
COPY --chown=0:0 --from=sdk-bootconfig /usr/share/licenses/bootconfig /usr/share/licenses/bootconfig

# "sdk-grub" has the grub-bios-setup tool
COPY --chown=0:0 --from=sdk-grub /usr/libexec/tools/grub-bios-setup /usr/libexec/tools/grub-bios-setup
COPY --chown=0:0 --from=sdk-grub /usr/share/licenses/grub /usr/share/licenses/grub

# "sdk-ca-certificates" has CA certificates extracted from Mozilla
COPY --chown=0:0 --from=sdk-ca-certificates \
  /usr/share/bottlerocket/ca-certificates \
  /usr/share/bottlerocket/ca-certificates

COPY --chown=0:0 --chmod=0644 \
  ./configs/ca-certificates/attribution.txt \
  /usr/share/licenses/ca-certificates/

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

# "sdk-nasm" has the NASM assembler.
COPY --chown=0:0 --from=sdk-nasm \
  /home/builder/nasm/nasm \
  /usr/bin/nasm

COPY --chown=0:0 --from=sdk-nasm \
  /home/builder/nasm/LICENSE \
  /usr/share/licenses/nasm/

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

# Instead of a symlink to libexec, we must select by go version.
COPY ./wrappers/go/go /usr/bin/go
COPY ./wrappers/go/go-latest /usr/bin/go-latest
COPY ./wrappers/go/gofmt /usr/bin/gofmt

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

# Forcibly undefine the auto_set_build_flags macros, since it no longer works to
# undefine it when rpmbuild is invoked (as of RPM 4.20).
RUN \
  sed -i '/%_auto_set_build_flags 1/d' \
    /usr/lib/rpm/redhat/macros

# Reset permissions for `builder`.
RUN \
  mkdir -p /home/builder && \
  chown builder:builder -R /home/builder

USER builder
RUN rpmdev-setuptree

# Create an empty "certdb" for signing.
WORKDIR /home/builder
RUN \
  mkdir .netscape && \
  certutil -N --empty-password

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Collect all builds for the SDK and squashes them into a final, single layer
FROM scratch AS sdk-golden

COPY --from=sdk-final / /

# The `builder` user is setup in the "final" layer and is used in place of the
# default `root` user
USER builder
WORKDIR /home/builder

# Set the default Go major version.
ENV GO_MAJOR="1.25"

# In NSS 3.101, lib::pkix was enabled as the default X.509 validator.
# This causes signature checking of secureboot artifacts to fail during build.
# Temporarily revert to the previous verifier.
ENV NSS_DISABLE_PKIX_VERIFY=1

CMD ["/bin/bash"]
