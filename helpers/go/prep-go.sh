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

# Apply Go patches.
git init
git apply --whitespace=nowarn "${HOME}"/patches-go/*.patch
