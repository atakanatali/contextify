#!/bin/sh
# Contextify CLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/atakanatali/contextify/main/scripts/install-cli.sh | sh
set -e

REPO="atakanatali/contextify"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="contextify"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin|linux) ;;
    *) echo "Error: Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET="${BINARY_NAME}-${OS}-${ARCH}"

# Get latest version or use specified version
VERSION="${CONTEXTIFY_VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
    URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}"
fi

echo "Installing Contextify CLI for ${OS}/${ARCH}..."
echo "Downloading from: ${URL}"

# Download
TMP_FILE=$(mktemp)
if ! curl -fsSL "$URL" -o "$TMP_FILE"; then
    echo "Error: Failed to download ${ASSET}"
    echo "Check https://github.com/${REPO}/releases for available versions."
    rm -f "$TMP_FILE"
    exit 1
fi

chmod +x "$TMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "Need sudo to install to ${INSTALL_DIR}"
    sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "Installed: $(${BINARY_NAME} version)"
echo ""
echo "Get started:"
echo "  contextify install    # Set up Contextify"
echo "  contextify help       # Show all commands"
