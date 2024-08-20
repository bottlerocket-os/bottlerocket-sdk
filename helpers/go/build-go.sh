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

cd "${HOME}/sdk-go/src"
./all.bash

# Install the Go standard library and toolchain.
cd "${HOME}/sdk-go"
# shellcheck disable=SC2034
PATH="${HOME}/sdk-go/bin:${PATH}" GO111MODULE="auto"
go install -buildmode=pie std cmd
install -p -m 0644 -Dt licenses LICENSE PATENTS
