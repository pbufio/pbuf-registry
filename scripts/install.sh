#!/bin/sh
set -e

# PowerBuf CLI installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh
# Or: wget -qO- https://raw.githubusercontent.com/pbufio/pbuf-registry/main/scripts/install.sh | sh

REPO="pbufio/pbuf-registry"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="pbuf"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux*)     OS="linux" ;;
    darwin*)    OS="darwin" ;;
    mingw*|msys*|cygwin*) 
        echo "Error: This script is for Unix-like systems. For Windows, please see installation instructions at:"
        echo "https://github.com/$REPO#installation"
        exit 1
        ;;
    *)
        echo "Error: Unsupported operating system: $OS"
        exit 1
        ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected OS: $OS"
echo "Detected Architecture: $ARCH"

# Get latest version
echo "Fetching latest version..."
LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "Error: Failed to fetch latest version"
    exit 1
fi

echo "Latest version: $LATEST_VERSION"

# Construct download URL
# Use version without leading 'v' in archive file name
PLAIN_VERSION="${LATEST_VERSION#v}"
ARCHIVE_NAME="pbuf_${PLAIN_VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/$ARCHIVE_NAME"

echo "Downloading $ARCHIVE_NAME..."

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download archive
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
    echo "Error: Failed to download $DOWNLOAD_URL"
    exit 1
fi

# Extract binary
echo "Extracting binary..."
tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

# Check if binary exists
if [ ! -f "$TMP_DIR/$BINARY_NAME" ]; then
    echo "Error: Binary not found in archive"
    exit 1
fi

# Install binary
echo "Installing $BINARY_NAME to $INSTALL_DIR..."

# Check if we need sudo
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
else
    echo "Installing to $INSTALL_DIR requires elevated privileges."
    sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
fi

echo ""
echo "✓ $BINARY_NAME $LATEST_VERSION installed successfully!"
echo ""
echo "Run '$BINARY_NAME --help' to get started."
echo ""
echo "To uninstall, run: sudo rm $INSTALL_DIR/$BINARY_NAME"
