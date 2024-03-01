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
make O="output/${ARCH}-gnu" defconfig BR2_DEFCONFIG="configs/sdk_${ARCH}_gnu_defconfig"
make O="output/${ARCH}-gnu" toolchain
find "output/${ARCH}-gnu/build/linux-headers-${KVER}/usr/include" -name '.*' -delete

cd "${HOME}/buildroot/output/${ARCH}-gnu/build"
install -p -m 0644 -Dt licenses/binutils host-binutils-*/COPYING{,3}{,.LIB}
install -p -m 0644 -Dt licenses/bison host-bison-*/COPYING
install -p -m 0644 -Dt licenses/gawk host-gawk-*/COPYING
install -p -m 0644 -Dt licenses/gcc host-gcc-final-*/{COPYING,COPYING.LIB,COPYING.RUNTIME,COPYING3,COPYING3.LIB}
install -p -m 0644 -Dt licenses/gmp host-gmp-*/COPYING{,v2,v3,.LESSERv3}
install -p -m 0644 -Dt licenses/isl host-isl-*/LICENSE
install -p -m 0644 -Dt licenses/linux linux-headers-*/{COPYING,LICENSES/preferred/GPL-2.0,LICENSES/exceptions/Linux-syscall-note}
install -p -m 0644 -Dt licenses/m4 host-m4-*/COPYING
install -p -m 0644 -Dt licenses/mpc host-mpc-*/COPYING.LESSER
install -p -m 0644 -Dt licenses/mpfr host-mpfr-*/COPYING{,.LESSER}
