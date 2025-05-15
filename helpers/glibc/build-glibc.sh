#!/usr/bin/env bash

set -eux -o pipefail
shopt -qs failglob

for opt in "$@"; do
   optarg="$(expr "${opt}" : '[^=]*=\(.*\)')"
   case "${opt}" in
      --arch=*) ARCH="${optarg}" ;;
      --kernel-version=*) KVER="${optarg}" ;;
   esac
done

ARCH="${ARCH:?}"
KVER="${KVER:?}"

TARGET="${ARCH}-bottlerocket-linux-gnu"
SYSROOT="/${TARGET}/sys-root"
BUILDFLAGS="-O2 -g -Wp,-D_GLIBCXX_ASSERTIONS -fstack-clash-protection -fno-omit-frame-pointer -mno-omit-leaf-frame-pointer"

case "${ARCH}" in
  x86_64)
    ARCH_CFLAGS=""
    ARCH_CONFIG=""
    ;;
  aarch64)
    ARCH_CFLAGS=""
    ARCH_CONFIG=""
    ;;
esac

cd "${HOME}/glibc/build"
CC="${TARGET}-gcc ${ARCH_CFLAGS}" CXX="${TARGET}-g++ ${ARCH_CFLAGS}" \
CFLAGS="${BUILDFLAGS}" CPPFLAGS="" CXXFLAGS="${BUILDFLAGS}" \
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
  --without-selinux \
  ${ARCH_CONFIG}

make -j"$(nproc)" -O -r
make install_root="${HOME}/glibc/output" install
