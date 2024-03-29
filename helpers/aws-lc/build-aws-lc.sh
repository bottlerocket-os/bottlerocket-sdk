#!/usr/bin/env bash
# shellcheck disable=SC2034

set -eux -o pipefail
shopt -qs failglob

for opt in "$@"; do
   optarg="$(expr "${opt}" : '[^=]*=\(.*\)')"
   case "${opt}" in
      --arch=*) ARCH="${optarg}" ;;
      --go-dir=*) GODIR="${optarg}" ;;
   esac
done

ARCH="${ARCH:?}"
GODIR="${GODIR:?}"
TARGET="${ARCH}-bottlerocket-linux-gnu"

# Some of the AWS-LC sources are built with `-O0`. This is not compatible with
# `-Wp,-D_FORTIFY_SOURCE=2`, which needs at least `-O2`. Add `-DGOBORING` to
# avoid weak symbols.
CFLAGS="${CFLAGS} -Wp,-U_FORTIFY_SOURCE -DGOBORING"

cd "${HOME}/aws-lc/build"
cmake \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_TOOLCHAIN_FILE="${TARGET}.toolchain.cmake" \
  -GNinja \
  -DFIPS=1 \
  ..
ninja

go run parse-functions.go \
  "${GODIR}/src/crypto/internal/boring/goboringcrypto.h" .

GOARCH_aarch64="arm64"
GOARCH_x86_64="amd64"
GOARCH_ARCH="GOARCH_${ARCH}"
GOARCH="${!GOARCH_ARCH}"

"${TARGET}-gcc" -c -o umod.o umod-"${GOARCH}".?

"${TARGET}-objcopy" \
  --globalize-symbol=BORINGSSL_bcm_power_on_self_test \
  crypto/libcrypto.a libcrypto.a

"${TARGET}-ld" \
  -r -nostdlib --whole-archive \
  -o goboringcrypto.o libcrypto.a umod.o

"${TARGET}-objcopy" \
  --redefine-syms=renames.txt \
  goboringcrypto.o

"${TARGET}-objcopy" \
  --keep-global-symbols=globals.txt --strip-unneeded \
  goboringcrypto.o "goboringcrypto_linux_${GOARCH}.syso"
