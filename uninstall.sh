#!/data/data/com.termux/files/usr/bin/sh
set -eu
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
rm -f "$HOME_DIR/.local/bin/tooie-theme-manager"
rm -f "$HOME_DIR/files/theme/tooie-theme-manager"
echo "Removed tooie-theme-manager binaries. Config files were left in place."
