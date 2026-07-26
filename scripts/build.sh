#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
GO_DIR="$ROOT_DIR/go"
BIN_DIR="$ROOT_DIR/bin"
mkdir -p "$BIN_DIR"

generate_wrapper() {
  local WRAPPER="$BIN_DIR/chatgpt-commit-msg-gen"
  cp "$SCRIPT_DIR/wrapper.sh" "$WRAPPER"
  chmod +x "$WRAPPER"
  chmod +x "$BIN_DIR"/chatgpt-commit-msg-gen-* 2>/dev/null || true
}

if [[ "${1:-}" == "--wrapper-only" ]]; then
  generate_wrapper
  exit 0
fi

VERSION="v$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' "$ROOT_DIR/package.json" | head -1)"
for PLATFORM in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do
  OS="${PLATFORM%/*}"
  ARCH="${PLATFORM#*/}"
  OUTPUT="$BIN_DIR/chatgpt-commit-msg-gen-${OS}-${ARCH}"
  echo "Building ${OS}/${ARCH} -> $(basename "$OUTPUT") (${VERSION})"
  (cd "$GO_DIR" && GOOS="$OS" GOARCH="$ARCH" go build -ldflags "-X main.version=${VERSION}" -o "$OUTPUT" .)
done
generate_wrapper

