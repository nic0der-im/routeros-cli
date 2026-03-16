#!/bin/sh
# routeros-cli installer
# Usage: curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | sh
set -e

REPO="nic0der-im/routeros-cli"
BINARY="routeros-cli"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)
        echo "Error: unsupported OS: $OS"
        exit 1
        ;;
esac

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release."
    echo "Visit https://github.com/${REPO}/releases to download manually."
    exit 1
fi

echo "Installing ${BINARY} v${TAG} (${OS}/${ARCH})..."

# Download
ARCHIVE="${BINARY}_${TAG}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${TAG}/${ARCHIVE}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

curl -sSL "$URL" -o "${TMPDIR}/${ARCHIVE}"

# Extract
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

# Install
BINARY_PATH=$(find "$TMPDIR" -name "$BINARY" -type f | head -1)

if [ -z "$BINARY_PATH" ]; then
    echo "Error: binary not found in archive."
    exit 1
fi

chmod +x "$BINARY_PATH"

if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"
fi

echo ""
echo "routeros-cli v${TAG} installed to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  routeros-cli device add myrouter --address 192.168.88.1:8728 --username admin --password-stdin"
echo "  routeros-cli device test"
echo "  routeros-cli system info"
echo ""
echo "Documentation: https://github.com/${REPO}"
