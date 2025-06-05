#!/usr/bin/env bash

set -e

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 $CA_BUNDLE_VERSION"
    echo
    echo "Example: $0 2024-12-31"
    exit 2
fi

TOOLSDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(realpath "${TOOLSDIR}/..")

VERSION="${1}"
OUTPUT="${ROOTDIR}/hashes/ca-certificates"

# Get the CA certificates bundle
# e.g. https://curl.se/ca/cacert-2024-12-31.pem
CA_BUNDLE_PACKAGE="cacert-${VERSION}.pem"
CA_BUNDLE_URL="https://curl.se/ca/${CA_BUNDLE_PACKAGE}"

rm -f "${CA_BUNDLE_PACKAGE}"
curl -s -L -O -C - "${CA_BUNDLE_URL}"

# Calculate SHA512 hash
CA_BUNDLE_512_SHA=$(sha512sum "${CA_BUNDLE_PACKAGE}" | cut -d ' ' -f 1)

# Add the root/header information
echo "# ${CA_BUNDLE_URL}" > "${OUTPUT}"
echo "SHA512 (${CA_BUNDLE_PACKAGE}) = ${CA_BUNDLE_512_SHA}" >> "${OUTPUT}"

DOCKERFILE="${ROOTDIR}/Dockerfile"
sed -i -e "s,^ENV CABUNDLEVER=.*,ENV CABUNDLEVER=\"${VERSION}\",g" "${DOCKERFILE}"

echo "================================================"
echo "ca-certificates updated to ${VERSION}"
echo "================================================"
