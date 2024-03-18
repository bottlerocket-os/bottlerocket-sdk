#!/usr/bin/env bash

set -e

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 $AWS_LC_VERSION"
    echo
    echo "Example: $0 2.0.9"
    exit 2
fi

TOOLSDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(realpath "${TOOLSDIR}/..")

VERSION="${1}"
OUTPUT="${ROOTDIR}/hashes/aws-lc"

# Get the AWS-LC-FIPS source package
# e.g. FIXME
AWS_LC_SRC_PACKAGE="AWS-LC-FIPS-${VERSION}.tar.gz"
AWS_LC_SRC_URL="https://github.com/aws/aws-lc/archive/refs/tags/${AWS_LC_SRC_PACKAGE}"

curl -s -L -O -C - "${AWS_LC_SRC_URL}"

AWS_LC_512_SHA=$(sha512sum "${AWS_LC_SRC_PACKAGE}" | cut -d ' ' -f 1)

# Add the root/header information
echo "# ${AWS_LC_SRC_URL}" > "${OUTPUT}"
echo "SHA512 (${AWS_LC_SRC_PACKAGE}) = ${AWS_LC_512_SHA}" >> "${OUTPUT}"

DOCKERFILE="${ROOTDIR}/Dockerfile"
sed -i -e "s,^ENV AWS_LC_FIPS_VER=.*,ENV AWS_LC_FIPS_VER=\"${VERSION}\",g" "${DOCKERFILE}"

echo "================================================"
echo "AWS-LC updated to ${VERSION}"
echo "================================================"
