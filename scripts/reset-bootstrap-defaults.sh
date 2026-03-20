#!/data/data/com.termux/files/usr/bin/sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_DIR="$HOME_DIR/.config/tooie"
STATE_DIR="$HOME_DIR/.local/state/tooie"
BACKUP_DIR="$STATE_DIR/backups/$(date +%Y%m%d-%H%M%S)"

log() { printf '%s\n' "$*"; }

backup_if_exists() {
  backup_src="$1"
  if [ -e "$backup_src" ]; then
    backup_rel="$(printf '%s' "$backup_src" | sed "s#^$HOME_DIR/##")"
    backup_dst="$BACKUP_DIR/$backup_rel"
    mkdir -p "$(dirname "$backup_dst")"
    cp -a "$backup_src" "$backup_dst"
  fi
}

install_file() {
  file_src="$1"
  file_dst="$2"
  backup_if_exists "$file_dst"
  mkdir -p "$(dirname "$file_dst")"
  cp "$file_src" "$file_dst"
}

install_dir() {
  dir_src="$1"
  dir_dst="$2"
  backup_if_exists "$dir_dst"
  mkdir -p "$(dirname "$dir_dst")"
  rm -rf "$dir_dst"
  cp -a "$dir_src" "$dir_dst"
}

resolve_defaults_dir() {
  if [ -d "$TOOIE_DIR/bootstrap-defaults" ]; then
    printf '%s' "$TOOIE_DIR/bootstrap-defaults"
    return 0
  fi
  if [ -d "$HOME_DIR/files/tooie/assets/defaults" ]; then
    printf '%s' "$HOME_DIR/files/tooie/assets/defaults"
    return 0
  fi
  return 1
}

reload_surfaces() {
  if command -v termux-reload-settings >/dev/null 2>&1; then
    termux-reload-settings || true
  fi
  if command -v tmux >/dev/null 2>&1; then
    if [ -n "${TMUX:-}" ] || tmux ls >/dev/null 2>&1; then
      tmux source-file "$HOME_DIR/.tmux.conf" >/dev/null 2>&1 || true
    fi
  fi
}

main() {
  defaults_dir="$(resolve_defaults_dir || true)"
  if [ -z "$defaults_dir" ]; then
    log "Reset failed: defaults directory not found. Re-run ./install.sh first."
    exit 1
  fi

  mkdir -p "$BACKUP_DIR" "$STATE_DIR" "$TOOIE_DIR"

  log "Resetting bootstrap-managed configs..."

  touch "$HOME_DIR/.hushlogin"
  install_file "$defaults_dir/.tmux.conf" "$HOME_DIR/.tmux.conf"

  install_file "$defaults_dir/.termux/termux.properties" "$HOME_DIR/.termux/termux.properties"
  install_file "$defaults_dir/.termux/colors.properties" "$HOME_DIR/.termux/colors.properties"
  install_file "$defaults_dir/.termux/font.ttf" "$HOME_DIR/.termux/font.ttf"
  install_file "$defaults_dir/.termux/font-italic.ttf" "$HOME_DIR/.termux/font-italic.ttf"
  backup_if_exists "$HOME_DIR/.termux/bin"
  mkdir -p "$HOME_DIR/.termux/bin"

  install_file "$defaults_dir/.config/starship.toml" "$HOME_DIR/.config/starship.toml"
  install_file "$defaults_dir/.config/fish/config.fish" "$HOME_DIR/.config/fish/config.fish"
  backup_if_exists "$HOME_DIR/.config/fish/conf.d/tooie-btop.fish"
  rm -f "$HOME_DIR/.config/fish/conf.d/tooie-btop.fish"
  install_file "$defaults_dir/.config/peaclock/config" "$HOME_DIR/.config/peaclock/config"

  # Preserve launcherctl endpoint/token auth files by replacing only config.json.
  install_file "$defaults_dir/.launcherctl/config.json" "$HOME_DIR/.launcherctl/config.json"

  install_dir "$defaults_dir/.config/tmux" "$HOME_DIR/.config/tmux"
  backup_if_exists "$TOOIE_DIR/btop"
  rm -rf "$TOOIE_DIR/btop"

  reload_surfaces

  log "Bootstrap defaults restored. Backup snapshot: $BACKUP_DIR"
}

main "$@"
