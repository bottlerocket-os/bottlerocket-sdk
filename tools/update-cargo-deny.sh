#!/usr/bin/env bash

set -e

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 $CARGO_DENY_VERSION"
    echo
    echo "Example: $0 0.14.20"
    exit 2
fi

TOOLSDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(realpath "${TOOLSDIR}/..")

VERSION="${1}"
OUTPUT="${ROOTDIR}/hashes/cargo-deny"

# Get the cargo-deny source package
# e.g. https://github.com/EmbarkStudios/cargo-deny/archive/0.14.20/cargo-deny-0.14.20.tar.gz
CARGO_DENY_SRC_PACKAGE="cargo-deny-${VERSION}.tar.gz"
CARGO_DENY_SRC_URL="https://github.com/EmbarkStudios/cargo-deny/archive/${VERSION}/${CARGO_DENY_SRC_PACKAGE}"

curl -s -L -O -C - "${CARGO_DENY_SRC_URL}"

CARGO_DENY_512_SHA=$(sha512sum "${CARGO_DENY_SRC_PACKAGE}" | cut -d ' ' -f 1)

# Add the root/header information
echo "# ${CARGO_DENY_SRC_URL}" > "${OUTPUT}"
echo "SHA512 (${CARGO_DENY_SRC_PACKAGE}) = ${CARGO_DENY_512_SHA}" >> "${OUTPUT}"

DOCKERFILE="${ROOTDIR}/Dockerfile"
sed -i -e "s,^ENV DENYVER=.*,ENV DENYVER=\"${VERSION}\",g" "${DOCKERFILE}"

echo "================================================"
echo "cargo-deny updated to ${VERSION}"
echo "================================================"
