#!/usr/bin/env sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_BIN=""

if [ -x "$HOME_DIR/.local/bin/tooie" ]; then
  TOOIE_BIN="$HOME_DIR/.local/bin/tooie"
elif command -v tooie >/dev/null 2>&1; then
  TOOIE_BIN="$(command -v tooie)"
fi

if [ -z "$TOOIE_BIN" ]; then
  echo "tooie theme apply: tooie binary not found in ~/.local/bin/tooie or PATH" >&2
  echo "Run ./install.sh or ensure tooie is in PATH." >&2
  exit 1
fi

exec "$TOOIE_BIN" theme apply "$@"
