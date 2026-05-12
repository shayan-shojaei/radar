#!/usr/bin/env bash
set -e

REPO="shayan-shojaei/radar"
INSTALL_DIR="/usr/local/bin"
BINARY="radar"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | cut -d'"' -f4)
fi

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version. Pass a version as the first argument." >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}-${OS}-${ARCH}"

echo "Installing radar ${VERSION} (${OS}/${ARCH})..."
TMP=$(mktemp /tmp/radar-XXXXXX)
curl -fSL --retry 3 --retry-delay 2 --progress-bar "$URL" -o "$TMP"
chmod +x "$TMP"
sudo mv "$TMP" "${INSTALL_DIR}/${BINARY}"
echo "radar ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
