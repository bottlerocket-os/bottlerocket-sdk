#!/usr/bin/env bash

set -eux -o pipefail
shopt -qs failglob

for opt in "$@"; do
   optarg="$(expr "${opt}" : '[^=]*=\(.*\)')"
   case "${opt}" in
      --arch=*) ARCH="${optarg}" ;;
   esac
done

ARCH="${ARCH:?}"

TARGET="${ARCH}-bottlerocket-linux-musl"
SYSROOT="/${TARGET}/sys-root"
CFLAGS="-O2 -g1 -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection -fno-omit-frame-pointer -mno-omit-leaf-frame-pointer"
LDFLAGS="-Wl,-z,relro -Wl,-z,now"

case "${ARCH}" in
  x86_64)
    ARCH_CFLAGS="-march=x86-64-v2 -mtune=generic -fcf-protection=full"
    ARCH_CONFIG=""
    ;;
  aarch64)
    ARCH_CFLAGS="-march=armv8-a+crypto+crc"
    ARCH_CONFIG=""
    ;;
esac

CFLAGS="${CFLAGS} ${ARCH_CFLAGS}"

cd "${HOME}/musl"
./configure \
  CFLAGS="${CFLAGS}" \
  LDFLAGS="${LDFLAGS}" \
  --target="${TARGET}" \
  --disable-gcc-wrapper \
  --enable-static \
  --prefix="${SYSROOT}/usr" \
  --libdir="${SYSROOT}/usr/lib" \
  ${ARCH_CONFIG}

make -j"$(nproc)"

OUTDIR="${HOME}/musl/output"
make install DESTDIR="${OUTDIR}"
install -p -m 0644 -Dt "${OUTDIR}/${SYSROOT}/usr/share/licenses/musl" COPYRIGHT
