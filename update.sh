#!/data/data/com.termux/files/usr/bin/sh
set -eu
REPO_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if command -v git >/dev/null 2>&1; then
  (cd "$REPO_DIR" && git pull --ff-only)
fi
exec "$REPO_DIR/install.sh"
