#!/data/data/com.termux/files/usr/bin/sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
THEME_DIR="$HOME_DIR/files/theme"
BACKUP_ROOT="$THEME_DIR/backups"
TERMUX_COLORS="$HOME_DIR/.termux/colors.properties"
TMUX_CONF="$HOME_DIR/.tmux.conf"
PEACLOCK_CONFIG="$HOME_DIR/.config/peaclock/config"
STARSHIP_CONFIG="$HOME_DIR/.config/starship.toml"
NVIM_THEME_FILE="$HOME_DIR/.config/nvim/lua/plugins/tooie-material.lua"

usage() {
  cat <<'EOF'
Usage: restore-material.sh [backup_id]

If backup_id is omitted, restores the latest backup.
Use list-material-backups.sh to see available backup ids.
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

if [ ! -d "$BACKUP_ROOT" ]; then
  echo "No backup directory found: $BACKUP_ROOT" >&2
  exit 1
fi

BACKUP_ID="${1:-}"
if [ -z "$BACKUP_ID" ]; then
  BACKUP_ID="$(ls -1 "$BACKUP_ROOT" 2>/dev/null | tail -n 1 || true)"
fi

if [ -z "$BACKUP_ID" ]; then
  echo "No backups available in $BACKUP_ROOT" >&2
  exit 1
fi

BACKUP_DIR="$BACKUP_ROOT/$BACKUP_ID"
if [ ! -d "$BACKUP_DIR" ]; then
  echo "Backup not found: $BACKUP_DIR" >&2
  exit 1
fi

restored_any=0

if [ -f "$BACKUP_DIR/colors.properties.bak" ]; then
  cp "$BACKUP_DIR/colors.properties.bak" "$TERMUX_COLORS"
  restored_any=1
fi

if [ -f "$BACKUP_DIR/tmux.conf.bak" ]; then
  cp "$BACKUP_DIR/tmux.conf.bak" "$TMUX_CONF"
  restored_any=1
fi

if [ -f "$BACKUP_DIR/peaclock.config.bak" ]; then
  mkdir -p "$HOME_DIR/.config/peaclock"
  cp "$BACKUP_DIR/peaclock.config.bak" "$PEACLOCK_CONFIG"
  restored_any=1
fi

if [ -f "$BACKUP_DIR/starship.toml.bak" ]; then
  mkdir -p "$HOME_DIR/.config"
  cp "$BACKUP_DIR/starship.toml.bak" "$STARSHIP_CONFIG"
  restored_any=1
fi

if [ -f "$BACKUP_DIR/tooie-material.lua.bak" ]; then
  mkdir -p "$(dirname "$NVIM_THEME_FILE")"
  cp "$BACKUP_DIR/tooie-material.lua.bak" "$NVIM_THEME_FILE"
  restored_any=1
fi

if [ "$restored_any" -eq 0 ]; then
  echo "No restorable files in: $BACKUP_DIR" >&2
  exit 1
fi

if command -v termux-reload-settings >/dev/null 2>&1; then
  termux-reload-settings || true
fi
if command -v tmux >/dev/null 2>&1; then
  tmux source-file "$TMUX_CONF" 2>/dev/null || true
fi

echo "Restored backup: $BACKUP_ID"
