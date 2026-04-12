#!/bin/sh
# install.sh — Downloads the latest agent-manager binary to /usr/local/bin.
# Usage: curl -sSL https://raw.githubusercontent.com/alexmosquera/agent-manager-pro/main/scripts/install.sh | sh
#
# Requirements: curl (pre-installed on macOS)
# Supports: darwin/arm64 (Apple Silicon), darwin/amd64 (Intel)

set -e

REPO="alexmosquera/agent-manager-pro"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="agent-manager"

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
        *)
            echo "Error: unsupported OS: $os (only macOS is supported)" >&2
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

    ARCHIVE="agent-manager-pro_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
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
    echo "agent-manager installed to ${INSTALL_DIR}/${BINARY_NAME}"
    echo "Run 'agent-manager version' to verify."
}

main
