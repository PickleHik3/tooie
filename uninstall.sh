#!/data/data/com.termux/files/usr/bin/sh
set -eu
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
rm -f "$HOME_DIR/.local/bin/tooie"
rm -f "$HOME_DIR/.config/tooie/tooie"
echo "Removed tooie binaries. Config files were left in place."
