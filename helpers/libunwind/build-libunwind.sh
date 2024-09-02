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
export CFLAGS="-O2 -g -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -Wp,-D_GLIBCXX_ASSERTIONS -fexceptions -fstack-clash-protection -fno-omit-frame-pointer"
export LDFLAGS="-Wl,-z,relro -Wl,-z,now"

cd "${HOME}/libunwind/build"
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
  ..
make unwind

OUTDIR="${HOME}/libunwind/output/${SYSROOT}"
make install-unwind DESTDIR="${OUTDIR}"
install -p -m 0644 -Dt "${OUTDIR}/usr/share/licenses/libunwind" ../LICENSE.TXT
