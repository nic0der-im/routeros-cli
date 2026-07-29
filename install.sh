#!/bin/sh
# ros (routeros-cli) installer
# Usage: curl -sSL https://raw.githubusercontent.com/nic0der-im/routeros-cli/main/install.sh | sh
set -e

REPO="nic0der-im/routeros-cli"
BINARY="ros"
LEGACY_BINARY="routeros-cli"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

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

# Prefer new ros_ naming; fall back to legacy routeros-cli_ archives.
ARCHIVE="${BINARY}_${TAG}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${TAG}/${ARCHIVE}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -sSL -f "$URL" -o "${TMPDIR}/${ARCHIVE}" 2>/dev/null; then
    ARCHIVE="${LEGACY_BINARY}_${TAG}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/v${TAG}/${ARCHIVE}"
    echo "Trying legacy archive name..."
    curl -sSL -f "$URL" -o "${TMPDIR}/${ARCHIVE}"
fi

tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

BINARY_PATH=$(find "$TMPDIR" -name "$BINARY" -type f | head -1)
if [ -z "$BINARY_PATH" ]; then
    BINARY_PATH=$(find "$TMPDIR" -name "$LEGACY_BINARY" -type f | head -1)
fi

if [ -z "$BINARY_PATH" ]; then
    echo "Error: binary not found in archive."
    exit 1
fi

chmod +x "$BINARY_PATH"

install_bin() {
    src="$1"
    dest="$2"
    if [ -w "$INSTALL_DIR" ]; then
        mv "$src" "$dest"
    else
        echo "Installing to ${dest} (requires sudo)..."
        sudo mv "$src" "$dest"
    fi
}

install_bin "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"

# Compatibility symlink
if [ ! -e "${INSTALL_DIR}/${LEGACY_BINARY}" ] || [ -L "${INSTALL_DIR}/${LEGACY_BINARY}" ]; then
    if [ -w "$INSTALL_DIR" ]; then
        ln -sf "$BINARY" "${INSTALL_DIR}/${LEGACY_BINARY}"
    else
        sudo ln -sf "$BINARY" "${INSTALL_DIR}/${LEGACY_BINARY}"
    fi
fi

echo ""
echo "ros v${TAG} installed to ${INSTALL_DIR}/${BINARY}"
echo "Compatibility alias: ${LEGACY_BINARY}"
echo ""
echo "Get started:"
echo "  echo 'password' | ros device add myrouter --address 192.168.88.1:8728 --username admin --password-stdin"
echo "  ros device test"
echo "  ros get system info"
echo "  ros --read-only audit -o json"
echo ""
echo "Documentation: https://github.com/${REPO}"
