#!/bin/sh
set -e

REPO="eddmann/whatsapp-cli"
INSTALL_DIR="${HOME}/.local/bin"

# Detect OS
OS=$(uname -s)
case "$OS" in
    Darwin) OS_NAME="macos" ;;
    Linux)  OS_NAME="linux" ;;
    *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH_NAME="x64" ;;
    aarch64) ARCH_NAME="arm64" ;;
    arm64)   ARCH_NAME="arm64" ;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Linux only has x64 builds
if [ "$OS_NAME" = "linux" ] && [ "$ARCH_NAME" = "arm64" ]; then
    echo "Linux arm64 builds not available, falling back to x64"
    ARCH_NAME="x64"
fi

BINARY_NAME="whatsapp-${OS_NAME}-${ARCH_NAME}"

echo "Installing whatsapp-cli..."
echo "  OS: $OS_NAME"
echo "  Arch: $ARCH_NAME"

# Get latest release URL
LATEST_URL=$(curl -sI "https://github.com/${REPO}/releases/latest" | grep -i "^location:" | sed 's/.*tag\///' | tr -d '\r\n')
if [ -z "$LATEST_URL" ]; then
    echo "Error: Could not determine latest release"
    exit 1
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_URL}/${BINARY_NAME}"

echo "  Version: $LATEST_URL"
echo "  URL: $DOWNLOAD_URL"

# Create install directory
mkdir -p "$INSTALL_DIR"

# Download binary
echo "Downloading..."
curl -fsSL "$DOWNLOAD_URL" -o "${INSTALL_DIR}/whatsapp"
chmod +x "${INSTALL_DIR}/whatsapp"

echo ""
echo "Installed whatsapp to ${INSTALL_DIR}/whatsapp"
echo "(Re-run this script to update)"

# Check if in PATH
if echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo ""
    echo "Run 'whatsapp --help' to get started"
else
    echo ""
    echo "Add ${INSTALL_DIR} to your PATH:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    echo "Then run 'whatsapp --help' to get started"
fi

echo ""
echo "Next step: whatsapp auth login"
