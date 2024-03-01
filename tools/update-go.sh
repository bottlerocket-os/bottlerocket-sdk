#!/usr/bin/env bash

set -e

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 $GO_VERSION"
    echo
    echo "Example: $0 1.21.1"
    exit 2
fi

TOOLSDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(realpath "${TOOLSDIR}/..")

VERSION="${1}"
OUTPUT="${ROOTDIR}/hashes/go"
PACKAGE_ROOT="https://go.dev/dl"

# Get the go source package
# e.g. https://go.dev/dl/go1.21.1.src.tar.gz
GO_SRC_PACKAGE="go${VERSION}.src.tar.gz"
GO_SRC_URL="${PACKAGE_ROOT}/${GO_SRC_PACKAGE}"

curl -s -L -O -C - "${GO_SRC_URL}"

# SHA256 is only used to validate the package with the published checksum
GO_256_SHA=$(sha256sum "${GO_SRC_PACKAGE}" | cut -d ' ' -f 1)

# The Go downloads are not signed, and there is not an easy checksum file to
# fetch to validate the download. This is a little fragile (will need to update
# if they change the download page structure) but it at least gives some level
# of validation that we have a good download.
PUBLISHED_MATCH=$(curl -s https://go.dev/dl/ | grep -B5 "${GO_256_SHA}" | grep "${GO_SRC_PACKAGE}")
if [[ -z "${PUBLISHED_MATCH}" ]]; then
    echo "Unable to verify source package checksum!!!"
    echo
    echo "Check ${GO_SRC_PACKAGE} contents compared to ${PACKAGE_ROOT}"
    echo "Local checksum: ${GO_256_SHA}"
    exit 2
fi

GO_512_SHA=$(sha512sum "${GO_SRC_PACKAGE}" | cut -d ' ' -f 1)

# Add the root/header information
echo "# ${GO_SRC_URL}" > "${OUTPUT}"
echo "SHA512 (${GO_SRC_PACKAGE}) = ${GO_512_SHA}" >> "${OUTPUT}"

DOCKERFILE="${ROOTDIR}/Dockerfile"
sed -i -e "s,^ENV GOVER=.*,ENV GOVER=\"${VERSION}\",g" "${DOCKERFILE}"

echo "================================================"
echo "go toolchain updated to ${VERSION}"
echo "================================================"
