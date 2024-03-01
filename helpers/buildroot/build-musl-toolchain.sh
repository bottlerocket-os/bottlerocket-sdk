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

cd "${HOME}/buildroot"
make "O=output/${ARCH}-musl" defconfig BR2_DEFCONFIG="configs/sdk_${ARCH}_musl_defconfig"
make "O=output/${ARCH}-musl" toolchain
find "output/${ARCH}-musl/build/linux-headers-${KVER}/usr/include" -name '.*' -delete

cd "${HOME}/buildroot/output/${ARCH}-musl/build"
install -p -m 0644 -Dt licenses/binutils host-binutils-*/COPYING{,3}{,.LIB}
install -p -m 0644 -Dt licenses/gcc host-gcc-final-*/{COPYING,COPYING.LIB,COPYING.RUNTIME,COPYING3,COPYING3.LIB}
install -p -m 0644 -Dt licenses/gmp host-gmp-*/COPYING{,v2,v3,.LESSERv3}
install -p -m 0644 -Dt licenses/isl host-isl-*/LICENSE
install -p -m 0644 -Dt licenses/linux linux-headers-*/{COPYING,LICENSES/preferred/GPL-2.0,LICENSES/exceptions/Linux-syscall-note}
install -p -m 0644 -Dt licenses/m4 host-m4-*/COPYING
install -p -m 0644 -Dt licenses/mpc host-mpc-*/COPYING.LESSER
install -p -m 0644 -Dt licenses/mpfr host-mpfr-*/COPYING{,.LESSER}

# Record the toolchain's files so they can be archived later for subsequent
# use in kernel module development.
cd "${HOME}/buildroot/output/${ARCH}-musl/toolchain"
find . -type f -printf '%P\n' > "../build/toolchain-${ARCH}.txt"

cd "${HOME}/buildroot/output/${ARCH}-musl/build"
find licenses -type f -printf '%P\n' > "toolchain-licenses-${ARCH}.txt"
