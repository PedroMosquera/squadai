#!/bin/sh
# install.sh — Downloads the latest squadai binary to /usr/local/bin.
# Usage: curl -sSL https://raw.githubusercontent.com/PedroMosquera/squadai/main/scripts/install.sh | sh
#
# Requirements: curl
# Supports: darwin/arm64, darwin/amd64, linux/arm64, linux/amd64
#
# Windows users: This POSIX shell script does not run on Windows natively.
# Install on Windows via one of the following methods:
#   1. go install: go install github.com/PedroMosquera/squadai/cmd/squadai@latest
#   2. Download the .zip release asset for windows/amd64 or windows/arm64 from:
#      https://github.com/PedroMosquera/squadai/releases/latest
#      and add the extracted squadai.exe to a directory on your PATH.

set -e

REPO="PedroMosquera/squadai"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="squadai"

# --- Detect architecture ---
detect_arch() {
    arch=$(uname -m)
    case "$arch" in
        x86_64)  echo "amd64" ;;
        arm64)   echo "arm64" ;;
        aarch64) echo "arm64" ;;
        *)
            echo "Error: unsupported architecture: $arch" >&2
            exit 1
            ;;
    esac
}

# --- Detect OS ---
detect_os() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        darwin) echo "darwin" ;;
        linux)  echo "linux" ;;
        *)
            echo "Error: unsupported OS: $os (only macOS and Linux are supported)" >&2
            exit 1
            ;;
    esac
}

# --- Get latest version tag ---
get_latest_version() {
    version=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        echo "Error: could not determine latest version" >&2
        exit 1
    fi
    echo "$version"
}

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)
    VERSION=$(get_latest_version)
    # Strip leading 'v' for archive name
    VERSION_NUM=$(echo "$VERSION" | sed 's/^v//')

    ARCHIVE="squadai_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    echo "Detected: ${OS}/${ARCH}"
    echo "Latest version: ${VERSION}"
    echo "Downloading: ${DOWNLOAD_URL}"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    curl -sSL -o "${TMPDIR}/${ARCHIVE}" "$DOWNLOAD_URL"

    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

    if [ ! -f "${TMPDIR}/${BINARY_NAME}" ]; then
        echo "Error: binary '${BINARY_NAME}' not found in archive" >&2
        exit 1
    fi

    # Install to INSTALL_DIR (may require sudo)
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        echo "Installing to ${INSTALL_DIR} requires elevated permissions."
        sudo mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    echo ""
    echo "squadai installed to ${INSTALL_DIR}/${BINARY_NAME}"
    echo "Run 'squadai version' to verify."
}

main
