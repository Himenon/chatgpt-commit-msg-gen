#!/bin/sh
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64) ARCH="amd64" ;;
esac
SCRIPT="$0"
while [ -L "$SCRIPT" ]; do
  LINK="$(readlink "$SCRIPT")"
  case "$LINK" in
    /*) SCRIPT="$LINK" ;;
    *) SCRIPT="$(dirname "$SCRIPT")/$LINK" ;;
  esac
done
DIR="$(cd "$(dirname "$SCRIPT")" && pwd)"
BINARY="$DIR/chatgpt-commit-msg-gen-${OS}-${ARCH}"
if [ ! -x "$BINARY" ]; then
  echo "[chatgpt-commit-msg-gen] Binary not found: $BINARY" >&2
  exit 0
fi
exec "$BINARY" "$@"
