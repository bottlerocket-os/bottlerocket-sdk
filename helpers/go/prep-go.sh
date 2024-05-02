#!/usr/bin/env bash

set -eux -o pipefail
shopt -qs failglob

for opt in "$@"; do
   optarg="$(expr "${opt}" : '[^=]*=\(.*\)')"
   case "${opt}" in
      --go-version=*) GOVER="${optarg}" ;;
   esac
done

GOVER="${GOVER:?}"

cd "${HOME}/sdk-go"
sdk-fetch "${HOME}/hashes-go"
tar --strip-components=1 -xf go${GOVER}.src.tar.gz
rm go${GOVER}.src.tar.gz

# Patch Go sources so that they work with AWS-LC as the crypto implementation.
# Note that this will break use of `GOEXPERIMENT=boringcrypto` when using the
# default syso files that ship with Go, since the functions and data structures
# will no longer match. We build the replacement AWS-LC syso files below.
git init
git apply --whitespace=nowarn "${HOME}"/patches-go/*.patch

# We need to build AWS-LC before we can build Go.
mkdir -p "${HOME}/aws-lc"
cd "${HOME}/aws-lc"
sdk-fetch "${HOME}/hashes-aws-lc"
tar --strip-components=1 -xf AWS-LC-FIPS-${AWS_LC_FIPS_VER}.tar.gz
rm AWS-LC-FIPS-${AWS_LC_FIPS_VER}.tar.gz

# Patch AWS-LC sources to avoid weak symbols for memory management functions
# when GOBORING is defined.
git init
git apply --whitespace=nowarn "${HOME}"/patches-aws-lc/*.patch
