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
CFLAGS="-O2 -g -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection -fno-omit-frame-pointer"
LDFLAGS="-Wl,-z,relro -Wl,-z,now"

cd "${HOME}/musl"
./configure \
  CFLAGS="${CFLAGS}" \
  LDFLAGS="${LDFLAGS}" \
  --target="${TARGET}" \
  --disable-gcc-wrapper \
  --enable-static \
  --prefix="${SYSROOT}/usr" \
  --libdir="${SYSROOT}/usr/lib"

make -j"$(nproc)"

OUTDIR="${HOME}/musl/output"
make install DESTDIR="${OUTDIR}"
install -p -m 0644 -Dt "${OUTDIR}/${SYSROOT}/usr/share/licenses/musl" COPYRIGHT
