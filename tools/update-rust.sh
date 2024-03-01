#!/bin/bash

set -e

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 $RUST_VERSION"
    echo
    echo "Example: $0 1.71.0"
    exit 2
fi

TOOLSDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(realpath "${TOOLSDIR}/..")

VERSION="${1}"
OUTPUT="${ROOTDIR}/hashes/rust"
PACKAGE_ROOT="https://static.rust-lang.org"
METADATA_FILE="metadata.json"
ARCHES=("x86_64" "aarch64")
PACKAGES=("rust-std" "rustc" "cargo")

function get_package_url {
    local package="$1"
    local arch="$2"

    path=$(grep -e "${package}.*${arch}-unknown-linux-gnu.tar.xz" "${METADATA_FILE}" | cut -d '"' -f 2)

    echo "${path}"
}

function add_package {
    local package="$1"
    local arch="$2"

    # Lookup and pull down the package files
    PKG_PATH=$(get_package_url "${package}" "${arch}")
    PKG_FILE=$(echo "${PKG_PATH}" | cut -d '/' -f 3)
    echo "Checking ${PACKAGE_ROOT}/${PKG_PATH}"
    curl -s -O -C - "${PACKAGE_ROOT}/${PKG_PATH}"
    curl -s -O -C - "${PACKAGE_ROOT}/${PKG_PATH}.asc"

    # Verify file integrity
    gpg --verify "${PKG_FILE}.asc" "${PKG_FILE}"

    # Get the package source SHA
    PKG_SHA=$(sha512sum "${PKG_FILE}" | cut -d ' ' -f 1)

    # Clean up
    rm "${PKG_FILE}.asc"

    # Write out details to the hash file
    echo "# ${PACKAGE_ROOT}/${PKG_PATH}" >> "${OUTPUT}"
    echo "SHA512 (${PKG_FILE}) = ${PKG_SHA}" >> "${OUTPUT}"
}

# Make sure the Rust project's public key is imported
curl -s https://keybase.io/rust/pgp_keys.asc | gpg --import

# Get the rustc source package
RUSTC_PACKAGE="rustc-${VERSION}-src.tar.xz"
RUSTC_URL="${PACKAGE_ROOT}/dist/${RUSTC_PACKAGE}"

curl -s -O -C - "${RUSTC_URL}"
curl -s -O -C - "${RUSTC_URL}.asc"
gpg --verify "${RUSTC_PACKAGE}.asc" "${RUSTC_PACKAGE}"

RUSTC_SHA=$(sha512sum "${RUSTC_PACKAGE}" | cut -d ' ' -f 1)
rm "${RUSTC_PACKAGE}.asc"

ARTIFACT_URL="https://raw.githubusercontent.com/rust-lang/rust/${VERSION}/src/stage0.json"
curl -s -o "${METADATA_FILE}" "${ARTIFACT_URL}"

# Add the root/header information
echo "# ${RUSTC_URL}" > "${OUTPUT}"
echo "SHA512 (${RUSTC_PACKAGE}) = ${RUSTC_SHA}" >> "${OUTPUT}"
echo "### See ${ARTIFACT_URL} for what to use below. ###" >> "${OUTPUT}"

# Get the details for each package
for arch in "${ARCHES[@]}"; do
    for package in "${PACKAGES[@]}"; do
        add_package "${package}" "${arch}"
    done
done

rm "${METADATA_FILE}"

DOCKERFILE="${ROOTDIR}/Dockerfile"
sed -i -e "s,^ENV RUSTVER=.*,ENV RUSTVER=\"${VERSION}\",g" "${DOCKERFILE}"

echo "================================================"
echo "Rust toolchain updated to ${VERSION}"
echo "================================================"
