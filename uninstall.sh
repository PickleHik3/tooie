#!/usr/bin/env sh
set -eu
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_BIN="$HOME_DIR/.local/bin/tooie"

if [ -x "$TOOIE_BIN" ]; then
  if "$TOOIE_BIN" helper uninstall --snapshot latest; then
    echo "Restored files from latest Tooie install snapshot."
    exit 0
  fi
fi

rm -f "$TOOIE_BIN"
echo "No valid install snapshot found. Removed tooie binary only."
