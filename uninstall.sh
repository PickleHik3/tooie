#!/usr/bin/env sh
set -eu
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
rm -f "$HOME_DIR/.local/bin/tooie"
echo "Removed tooie binary. Config files were left in place."
