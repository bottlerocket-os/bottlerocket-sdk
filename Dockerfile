FROM fedora:33 as base

# Everything we need to build our SDK and packages.
RUN \
  dnf makecache && \
  dnf -y update && \
  dnf -y groupinstall "C Development Tools and Libraries" && \
  dnf -y install \
    rpmdevtools dnf-plugins-core createrepo_c \
    cmake git meson perl-ExtUtils-MakeMaker python which \
    bc hostname intltool gperf kmod rsync wget openssl \
    dwarves elfutils-devel libcap-devel openssl-devel \
    createrepo_c e2fsprogs gdisk grub2-tools.$(uname -m) \
    kpartx lz4 veritysetup dosfstools mtools squashfs-tools \
    perl-FindBin perl-open policycoreutils secilc qemu-img \
    glib2-devel rpcgen && \
  dnf clean all && \
  useradd builder
COPY ./sdk-fetch /usr/local/bin

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# We expect our C cross-compiler to be used on other distros for building kernel
# modules, so we build it with an older glibc for compatibility.
FROM ubuntu:16.04 as compat
RUN \
  apt-get update && \
  apt-get -y dist-upgrade && \
  apt-get -y install \
    autoconf automake bc build-essential cpio curl file git \
    libexpat1-dev libtool libz-dev pkgconf python3 unzip wget && \
  useradd -m -u 1000 builder
COPY ./sdk-fetch /usr/local/bin

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM compat as toolchain
USER builder

# Configure Git for any subsequent use.
RUN \
  git config --global user.name "Builder" && \
  git config --global user.email "builder@localhost"

ARG BRVER="2021.02.3"
ARG KVER="5.4.125"

WORKDIR /home/builder
COPY ./hashes/buildroot ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf buildroot-${BRVER}.tar.gz && \
  rm buildroot-${BRVER}.tar.gz && \
  mv buildroot-${BRVER} buildroot && \
  mv queue.h queue.h?rev=1.70

WORKDIR /home/builder/buildroot
COPY ./patches/buildroot/* ./
COPY ./configs/buildroot/* ./configs/
RUN \
  git init . && \
  git apply --whitespace=nowarn *.patch

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain as toolchain-gnu
ARG ARCH
ARG KVER="5.4.125"
RUN \
  make O=output/${ARCH}-gnu defconfig BR2_DEFCONFIG=configs/sdk_${ARCH}_gnu_defconfig && \
  make O=output/${ARCH}-gnu toolchain && \
  find output/${ARCH}-gnu/build/linux-headers-${KVER}/usr/include -name '.*' -delete

WORKDIR /home/builder/buildroot/output/${ARCH}-gnu/build
SHELL ["/bin/bash", "-c"]
RUN \
  install -p -m 0644 -Dt licenses/binutils host-binutils-*/COPYING{,3}{,.LIB} && \
  install -p -m 0644 -Dt licenses/bison host-bison-*/COPYING && \
  install -p -m 0644 -Dt licenses/gawk host-gawk-*/COPYING && \
  install -p -m 0644 -Dt licenses/gcc host-gcc-final-*/{COPYING,COPYING.LIB,COPYING.RUNTIME,COPYING3,COPYING3.LIB} && \
  install -p -m 0644 -Dt licenses/gmp host-gmp-*/COPYING{,v2,v3,.LESSERv3} && \
  install -p -m 0644 -Dt licenses/isl host-isl-*/LICENSE && \
  install -p -m 0644 -Dt licenses/linux linux-headers-*/{COPYING,LICENSES/preferred/GPL-2.0,LICENSES/exceptions/Linux-syscall-note} && \
  install -p -m 0644 -Dt licenses/m4 host-m4-*/COPYING && \
  install -p -m 0644 -Dt licenses/mpc host-mpc-*/COPYING.LESSER && \
  install -p -m 0644 -Dt licenses/mpfr host-mpfr-*/COPYING{,.LESSER}

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM toolchain as toolchain-musl
ARG ARCH
ARG KVER="5.4.125"
RUN \
  make O=output/${ARCH}-musl defconfig BR2_DEFCONFIG=configs/sdk_${ARCH}_musl_defconfig && \
  make O=output/${ARCH}-musl toolchain && \
  find output/${ARCH}-musl/build/linux-headers-${KVER}/usr/include -name '.*' -delete

WORKDIR /home/builder/buildroot/output/${ARCH}-musl/build
SHELL ["/bin/bash", "-c"]
RUN \
  install -p -m 0644 -Dt licenses/binutils host-binutils-*/COPYING{,3}{,.LIB} && \
  install -p -m 0644 -Dt licenses/gcc host-gcc-final-*/{COPYING,COPYING.LIB,COPYING.RUNTIME,COPYING3,COPYING3.LIB} && \
  install -p -m 0644 -Dt licenses/gmp host-gmp-*/COPYING{,v2,v3,.LESSERv3} && \
  install -p -m 0644 -Dt licenses/isl host-isl-*/LICENSE && \
  install -p -m 0644 -Dt licenses/linux linux-headers-*/{COPYING,LICENSES/preferred/GPL-2.0,LICENSES/exceptions/Linux-syscall-note} && \
  install -p -m 0644 -Dt licenses/m4 host-m4-*/COPYING && \
  install -p -m 0644 -Dt licenses/mpc host-mpc-*/COPYING.LESSER && \
  install -p -m 0644 -Dt licenses/mpfr host-mpfr-*/COPYING{,.LESSER}

# For kernel module development, we only need one toolchain, and it doesn't
# matter which one we pick since the kernel doesn't use the C library. Record
# the files we need so they can be archived later.
WORKDIR /home/builder/buildroot/output/${ARCH}-musl/toolchain
RUN find . -type f -printf '%P\n' > ../build/toolchain.txt

WORKDIR /home/builder/buildroot/output/${ARCH}-musl/build
RUN find licenses -type f -printf '%P\n' > toolchain-licenses.txt

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Add our cross-compilers to the base SDK layer.
FROM base as sdk
USER root

ARG ARCH
ARG KVER="5.4.125"

WORKDIR /

COPY --from=toolchain-gnu \
  /home/builder/buildroot/output/${ARCH}-gnu/toolchain/ /
COPY --from=toolchain-gnu \
  /home/builder/buildroot/output/${ARCH}-gnu/build/linux-headers-${KVER}/usr/include/ \
  /${ARCH}-bottlerocket-linux-gnu/sys-root/usr/include/
COPY --from=toolchain-gnu \
  /home/builder/buildroot/output/${ARCH}-gnu/build/licenses/ \
  /${ARCH}-bottlerocket-linux-gnu/sys-root/usr/share/licenses/

COPY --from=toolchain-musl \
  /home/builder/buildroot/output/${ARCH}-musl/toolchain/ /
COPY --from=toolchain-musl \
  /home/builder/buildroot/output/${ARCH}-musl/build/linux-headers-${KVER}/usr/include/ \
  /${ARCH}-bottlerocket-linux-musl/sys-root/usr/include/
COPY --from=toolchain-musl \
  /home/builder/buildroot/output/${ARCH}-musl/build/licenses/ \
  /${ARCH}-bottlerocket-linux-musl/sys-root/usr/share/licenses/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Build C libraries so we can build our rust and golang toolchains.
FROM sdk as sdk-gnu
USER builder

ARG GLIBCVER="2.33"

WORKDIR /home/builder
COPY ./hashes/glibc ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf glibc-${GLIBCVER}.tar.xz && \
  rm glibc-${GLIBCVER}.tar.xz && \
  mv glibc-${GLIBCVER} glibc && \
  cd glibc && \
  mkdir build

ARG ARCH
ARG TARGET="${ARCH}-bottlerocket-linux-gnu"
ARG SYSROOT="/${TARGET}/sys-root"
ARG CFLAGS="-O2 -g -Wp,-D_GLIBCXX_ASSERTIONS -fstack-clash-protection"
ARG CXXFLAGS="${CFLAGS}"
ARG CPPFLAGS=""
ARG KVER="5.4"

WORKDIR /home/builder/glibc/build
RUN \
  ../configure \
    --prefix="${SYSROOT}/usr" \
    --sysconfdir="/etc" \
    --localstatedir="/var" \
    --target="${TARGET}" \
    --host="${TARGET}" \
    --with-headers="/${SYSROOT}/usr/include" \
    --enable-bind-now \
    --enable-kernel="${KVER}" \
    --enable-shared \
    --enable-stack-protector=strong \
    --disable-crypt \
    --disable-multi-arch \
    --disable-profile \
    --disable-systemtap \
    --disable-timezone-tools \
    --disable-tunables \
    --without-cvs \
    --without-gd \
    --without-selinux && \
  make -j$(nproc) -O -r

USER root
WORKDIR /home/builder/glibc/build
RUN make install

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-musl
USER builder

ARG MUSLVER="1.2.2"

WORKDIR /home/builder
COPY ./hashes/musl ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf musl-${MUSLVER}.tar.gz && \
  rm musl-${MUSLVER}.tar.gz && \
  mv musl-${MUSLVER} musl

ARG ARCH
ARG TARGET="${ARCH}-bottlerocket-linux-musl"
ARG SYSROOT="/${TARGET}/sys-root"
ARG CFLAGS="-O2 -g -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection"
ARG LDFLAGS="-Wl,-z,relro -Wl,-z,now"

WORKDIR /home/builder/musl
RUN \
  ./configure \
    CFLAGS="${CFLAGS}" \
    LDFLAGS="${LDFLAGS}" \
    --target="${TARGET}" \
    --disable-gcc-wrapper \
    --enable-static \
    --prefix="${SYSROOT}/usr" \
    --libdir="${SYSROOT}/usr/lib" && \
   make -j$(nproc)

USER root
WORKDIR /home/builder/musl
RUN make install
RUN \
  install -p -m 0644 -Dt ${SYSROOT}/usr/share/licenses/musl COPYRIGHT

ARG LLVMVER="12.0.0"

USER builder
WORKDIR /home/builder

# Rust's musl targets depend on libunwind.
COPY ./hashes/libunwind ./hashes
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

WORKDIR /home/builder/libunwind/build
RUN \
  cmake \
    -DLLVM_PATH=../../llvm \
    -DLIBUNWIND_ENABLE_SHARED=1 \
    -DLIBUNWIND_ENABLE_STATIC=1 \
    -DCMAKE_INSTALL_PREFIX="/usr" \
    -DCMAKE_C_COMPILER="${TARGET}-gcc" \
    -DCMAKE_C_COMPILER_TARGET="${TARGET}" \
    -DCMAKE_CXX_COMPILER="${TARGET}-g++" \
    -DCMAKE_CXX_COMPILER_TARGET="${TARGET}" \
    -DCMAKE_AR="/usr/bin/${TARGET}-ar" \
    -DCMAKE_RANLIB="/usr/bin/${TARGET}-ranlib" \
    .. && \
  make unwind

USER root
WORKDIR /home/builder/libunwind/build
RUN make install-unwind DESTDIR="${SYSROOT}"
RUN \
  install -p -m 0644 -Dt ${SYSROOT}/usr/share/licenses/libunwind ../LICENSE.TXT

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as sdk-libc

ARG ARCH
ARG GNU_TARGET="${ARCH}-bottlerocket-linux-gnu"
ARG GNU_SYSROOT="/${GNU_TARGET}/sys-root"
ARG MUSL_TARGET="${ARCH}-bottlerocket-linux-musl"
ARG MUSL_SYSROOT="/${MUSL_TARGET}/sys-root"

COPY --from=sdk-gnu ${GNU_SYSROOT}/ ${GNU_SYSROOT}/
COPY --from=sdk-musl ${MUSL_SYSROOT}/ ${MUSL_SYSROOT}/

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libc as sdk-rust

USER root
RUN \
  mkdir -p /usr/libexec/rust && \
  chown -R builder:builder /usr/libexec/rust

ARG ARCH
ARG HOST_ARCH
ARG VENDOR="bottlerocket"
ARG RUSTVER="1.53.0"

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
  dir=build/cache/$(awk '/^date:/ { print $2 }' src/stage0.txt); \
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
  for libc in gnu musl ; do \
    cp compiler/rustc_target/src/spec/${ARCH}_{unknown,${VENDOR}}_linux_${libc}.rs && \
    sed -i -e '/let mut base = super::linux_'${libc}'_base::opts();/a base.vendor = "'${VENDOR}'".to_string();' \
      compiler/rustc_target/src/spec/${ARCH}_${VENDOR}_linux_${libc}.rs && \
    sed -i -e '/("'${ARCH}-unknown-linux-${libc}'", .*),/a("'${ARCH}-${VENDOR}-linux-${libc}'", '${ARCH}_${VENDOR}_linux_${libc}'),' \
      compiler/rustc_target/src/spec/mod.rs ; \
  done && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/mod.rs && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/${ARCH}_${VENDOR}_linux_gnu.rs && \
  grep -Fq ${VENDOR} compiler/rustc_target/src/spec/${ARCH}_${VENDOR}_linux_musl.rs

# In addition to our vendor-specific targets, we also need to build for the host
# platform, since that is no longer done implicitly.
COPY ./configs/rust/* ./
RUN \
  sed -e "s,@HOST_TRIPLE@,${HOST_ARCH}-unknown-linux-gnu,g" config-${ARCH}.toml.in > config.toml && \
  RUSTUP_DIST_SERVER=example:// python3 ./x.py install

RUN \
  install -p -m 0644 -Dt licenses COPYRIGHT LICENSE-*

# Set appropriate environment for using this Rust compiler to build tools
ENV PATH="/usr/libexec/rust/bin:$PATH" LD_LIBRARY_PATH="/usr/libexec/rust/lib"

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-libc as sdk-go

ARG ARCH
ARG TARGET="${ARCH}-bottlerocket-linux-gnu"
ARG GOVER="1.16.5"

USER root
RUN dnf -y install golang

USER builder
WORKDIR /home/builder/sdk-go
COPY ./hashes/go /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar --strip-components=1 -xf go${GOVER}.src.tar.gz && \
  rm go${GOVER}.src.tar.gz

ARG GOROOT_FINAL="/usr/libexec/go"
ARG GOOS="linux"
ARG CGO_ENABLED=1
ARG GOARCH_aarch64="arm64"
ARG GOARCH_x86_64="amd64"
ARG GOARCH_ARCH="GOARCH_${ARCH}"
ARG CFLAGS="-O2 -g -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection"
ARG CXXFLAGS="${CFLAGS}"
ARG LDFLAGS="-Wl,-z,relro -Wl,-z,now"
ARG CGO_CFLAGS="${CFLAGS}"
ARG CGO_CXXFLAGS="${CXXFLAGS}"
ARG CGO_LDFLAGS="${LDFLAGS}"

WORKDIR /home/builder/sdk-go/src
RUN ./make.bash --no-clean

# Build the standard library with and without PIE. Target binaries
# should use PIE, but any host binaries generated during the build
# might not.
WORKDIR /home/builder/sdk-go
ENV GOOS="${GOOS}" \
  GOROOT="/home/builder/sdk-go" \
  GOPATH="/home/builder/gopath" \
  PATH="/home/builder/sdk-go/bin:${PATH}"
RUN \
  export GOARCH="${!GOARCH_ARCH}" ; \
  export CC="${TARGET}-gcc" ; \
  export CC_FOR_TARGET="${TARGET}-gcc" ; \
  export CC_FOR_${GOOS}_${GOARCH}="${TARGET}-gcc" ; \
  export CXX="${TARGET}-g++" ; \
  export CXX_FOR_TARGET="${TARGET}-g++" ; \
  export CXX_FOR_${GOOS}_${GOARCH}="${TARGET}-g++" ; \
  export GOFLAGS="-mod=vendor" ; \
  go install std && \
  go install -buildmode=pie std

RUN \
  install -p -m 0644 -Dt licenses LICENSE PATENTS

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-rust as sdk-license-scan

USER root
RUN \
  mkdir -p /usr/libexec/tools /home/builder/license-scan /usr/share/licenses/bottlerocket-license-scan && \
  chown -R builder:builder /usr/libexec/tools /home/builder/license-scan /usr/share/licenses/bottlerocket-license-scan

ARG SPDXVER="3.13"

USER builder
WORKDIR /home/builder/license-scan
COPY ./hashes/license-scan ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf license-list-data-${SPDXVER}.tar.gz license-list-data-${SPDXVER}/json/details && \
  rm license-list-data-${SPDXVER}.tar.gz && \
  mv license-list-data-${SPDXVER} license-list-data
COPY license-scan /home/builder/license-scan
RUN cargo build --release --locked
RUN install -p -m 0755 target/release/bottlerocket-license-scan /usr/libexec/tools/
RUN cp -r license-list-data/json/details /usr/libexec/tools/spdx-data
COPY COPYRIGHT LICENSE-APACHE LICENSE-MIT /usr/share/licenses/bottlerocket-license-scan/
# quine - scan the license tool itself for licenses
RUN \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/bottlerocket-license-scan/vendor \
    cargo --locked Cargo.toml

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-license-scan as sdk-cargo-deny

USER root
RUN \
  mkdir -p /usr/share/licenses/cargo-deny && \
  chown -R builder:builder /usr/share/licenses/cargo-deny

ARG DENYVER="0.6.2"

USER builder
WORKDIR /home/builder
COPY ./hashes/cargo-deny ./hashes
RUN \
  sdk-fetch hashes && \
  tar xf cargo-deny-${DENYVER}.tar.gz && \
  rm cargo-deny-${DENYVER}.tar.gz && \
  mv cargo-deny-${DENYVER} cargo-deny

WORKDIR /home/builder/cargo-deny
COPY LICENSE-APACHE LICENSE-MIT /usr/share/licenses/cargo-deny/
COPY ./configs/cargo-deny/clarify.toml .
RUN \
  cargo build --release --locked && \
  install -p -m 0755 target/release/cargo-deny /usr/libexec/tools/ && \
  /usr/libexec/tools/bottlerocket-license-scan \
    --clarify clarify.toml \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/cargo-deny/vendor \
    cargo --locked Cargo.toml

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk-go as sdk-govc

USER root
RUN \
  mkdir -p /usr/libexec/tools /usr/share/licenses/govmomi && \
  chown -R builder:builder /usr/libexec/tools /usr/share/licenses/govmomi

ARG GOVMOMIVER="0.26.0"
ARG GOVMOMISHORTCOMMIT="34586b6"
ARG GOVMOMIDATE="2021-06-03T19:03:25Z"

USER builder
WORKDIR ${GOPATH}/src/github.com/vmware/govmomi
COPY ./hashes/govmomi /home/builder/hashes
RUN \
  sdk-fetch /home/builder/hashes && \
  tar --strip-components=1 -xf govmomi-${GOVMOMIVER}.tar.gz && \
  rm govmomi-${GOVMOMIVER}.tar.gz

COPY --chown=0:0 --from=sdk-license-scan /usr/libexec/tools/ /usr/libexec/tools/
RUN \
  cp -p LICENSE.txt /usr/share/licenses/govmomi && \
  go mod vendor && \
  /usr/libexec/tools/bottlerocket-license-scan \
    --spdx-data /usr/libexec/tools/spdx-data \
    --out-dir /usr/share/licenses/govmomi \
    go-vendor ./vendor

RUN \
  export CGO_ENABLED=0 ; \
  export BUILD_VERSION_PKG="github.com/vmware/govmomi/govc/flags" ; \
  go build -mod=vendor -o /usr/libexec/tools/govc -ldflags " \
    -X ${BUILD_VERSION_PKG}.BuildVersion=${GOVMOMIVER} \
    -X ${BUILD_VERSION_PKG}.BuildCommit=${GOVMOMISHORTCOMMIT} \
    -X ${BUILD_VERSION_PKG}.BuildDate=${GOVMOMIDATE} \
    " github.com/vmware/govmomi/govc

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

FROM sdk as toolchain-archive

ARG ARCH
ARG MUSL_TARGET="${ARCH}-bottlerocket-linux-musl"
ARG MUSL_SYSROOT="/${MUSL_TARGET}/sys-root"

COPY --from=toolchain-musl \
  /home/builder/buildroot/output/${ARCH}-musl/build/toolchain.txt \
  /tmp/toolchain.txt

COPY --from=toolchain-musl \
  /home/builder/buildroot/output/${ARCH}-musl/build/toolchain-licenses.txt \
  /tmp/toolchain-licenses.txt

WORKDIR /tmp

RUN \
  tar cvf toolchain.tar --transform "s,^,toolchain/," \
    -C / -T toolchain.txt && \
  tar rvf toolchain.tar --transform "s,^,toolchain/licenses/," \
    -C /${MUSL_SYSROOT}/usr/share/licenses -T toolchain-licenses.txt && \
  tar xvf toolchain.tar -C /

FROM scratch as toolchain-final
COPY --from=toolchain-archive /toolchain /toolchain

# =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=   =^..^=

# Collect all builds in a single layer.
FROM scratch as sdk-final
USER root

ARG ARCH
ARG GNU_TARGET="${ARCH}-bottlerocket-linux-gnu"
ARG GNU_SYSROOT="/${GNU_TARGET}/sys-root"
ARG MUSL_TARGET="${ARCH}-bottlerocket-linux-musl"
ARG MUSL_SYSROOT="/${MUSL_TARGET}/sys-root"

WORKDIR /
# "sdk" has our C/C++ toolchain and kernel headers for both targets.
COPY --from=sdk / /

# "sdk-musl" has a superset of the above, and includes C library and headers.
# We omit "sdk-gnu" because we expect to build glibc again for the target OS,
# while we will use the musl artifacts directly to generate static binaries
# such as migrations.
COPY --chown=0:0 --from=sdk-musl ${MUSL_SYSROOT}/ ${MUSL_SYSROOT}/

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
COPY --chown=0:0 --from=sdk-go \
  /home/builder/sdk-go/licenses/ \
  /usr/share/licenses/go/

# "sdk-license-scan" has our attribution generation tool.
COPY --chown=0:0 --from=sdk-license-scan /usr/libexec/tools/ /usr/libexec/tools/
# quine - include the licenses for the license scan tool itself
COPY --chown=0:0 --from=sdk-license-scan /usr/share/licenses/bottlerocket-license-scan/ /usr/share/licenses/bottlerocket-license-scan/

# "sdk-cargo-deny" has the cargo deny command and licenses.
COPY --chown=0:0 --from=sdk-cargo-deny /usr/libexec/tools/ /usr/libexec/tools/
COPY --chown=0:0 --from=sdk-cargo-deny /usr/share/licenses/cargo-deny/ /usr/share/licenses/cargo-deny/

# "sdk-govc" has the VMware govc tool and licenses.
COPY --chown=0:0 --from=sdk-govc /usr/libexec/tools/govc /usr/libexec/tools/govc
COPY --chown=0:0 --from=sdk-govc /usr/share/licenses/govmomi /usr/share/licenses/govmomi

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

# Add target binutils to $PATH to override programs used to extract debuginfo.
RUN \
  ln -s ../../${GNU_TARGET}/bin/nm /usr/local/bin/nm && \
  ln -s ../../${GNU_TARGET}/bin/objcopy /usr/local/bin/objcopy && \
  ln -s ../../${GNU_TARGET}/bin/objdump /usr/local/bin/objdump && \
  ln -s ../../${GNU_TARGET}/bin/strip /usr/local/bin/strip

# Strip and add tools to the path.
RUN \
  for b in /usr/libexec/tools/* ; do \
    strip -g $b ; \
    ln -s ../libexec/tools/${b##*/} /usr/bin/${b##*/} ; \
  done

# Make the licenses in the sys-roots easier to find.
RUN \
  ln -sr /${ARCH}-bottlerocket-linux-gnu/sys-root/usr/share/licenses /usr/share/licenses/bottlerocket-sdk-gnu && \
  ln -sr /${ARCH}-bottlerocket-linux-musl/sys-root/usr/share/licenses /usr/share/licenses/bottlerocket-sdk-musl

# Reset permissions for `builder`.
RUN chown builder:builder -R /home/builder

USER builder
RUN rpmdev-setuptree

CMD ["/bin/bash"]
