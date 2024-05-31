#!/usr/bin/env bash

set -e

if [[ $# -ne 0 ]]; then
    echo "Usage: $0"
    exit 2
fi

TOOLSDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(realpath "${TOOLSDIR}/..")

OUTPUT="${ROOTDIR}/hashes/grub"

# Retrieve the latest GRUB source package
cmd='dnf install -q -y --releasever=latest yum-utils && yumdownloader -q --releasever=latest --source --urls grub2'
GRUB_SRPM_URL="$(docker run --rm amazonlinux:2023 sh -c "${cmd}" \
    | grep '^http' \
    | xargs --max-args=1 --no-run-if-empty realpath --canonicalize-missing --relative-to=. \
    | sed 's_:/_://_')"
curl -s -L -O -C - "${GRUB_SRPM_URL}"

# Find the hash and version.
GRUB_SRPM_PACKAGE="${GRUB_SRPM_URL##*/}"
GRUB_SRPM_512_SHA=$(sha512sum "${GRUB_SRPM_PACKAGE}" | awk '{print $1}')
GRUB_VERSION="$(echo "${GRUB_SRPM_PACKAGE}" | awk 'match($0, /grub2-(.*)\.src\.rpm/, a) {print a[1]}')"

# Add the root/header information
echo "# ${GRUB_SRPM_URL}" > "${OUTPUT}"
echo "SHA512 (${GRUB_SRPM_PACKAGE}) = ${GRUB_SRPM_512_SHA}" >> "${OUTPUT}"

DOCKERFILE="${ROOTDIR}/Dockerfile"
sed -i -e "s,^ENV GRUB_VER=.*,ENV GRUB_VER=\"${GRUB_VERSION}\",g" "${DOCKERFILE}"

echo "================================================"
echo "GRUB updated to ${GRUB_VERSION}"
echo "================================================"
