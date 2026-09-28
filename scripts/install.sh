#!/bin/sh
set -eu

REPO="Himenon/chatgpt-commit-msg-gen"
BINARY_NAME="chatgpt-commit-msg-gen"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in darwin|linux) ;; *) echo "Unsupported OS: $OS" >&2; exit 1;; esac
ARCH="$(uname -m)"
case "$ARCH" in arm64|aarch64) ARCH="arm64";; x86_64|amd64) ARCH="amd64";; *) echo "Unsupported architecture: $ARCH" >&2; exit 1;; esac

VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
fi
ASSET_NAME="${BINARY_NAME}-${OS}-${ARCH}"
mkdir -p "$INSTALL_DIR"
curl -fsSL "https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}" -o "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
echo "Installed: ${INSTALL_DIR}/${BINARY_NAME}"
echo "Run 'codex login' to use Codex without an API key, or set OPENAI_API_KEY for API fallback, then configure Lefthook."
