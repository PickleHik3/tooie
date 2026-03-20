#!/data/data/com.termux/files/usr/bin/sh
set -eu

REPO_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
STATE_DIR="$HOME_DIR/.local/state/tooie"
BACKUP_DIR="$STATE_DIR/backups/$(date +%Y%m%d-%H%M%S)"
BIN_DIR="$HOME_DIR/.local/bin"
TOOIE_DIR="$HOME_DIR/.config/tooie"
LAUNCHERCTL_BACKEND_DIR="$HOME_DIR/.launcherctl"

mkdir -p "$BACKUP_DIR" "$BIN_DIR" "$TOOIE_DIR" "$LAUNCHERCTL_BACKEND_DIR" "$STATE_DIR"

log() { printf '%s\n' "$*"; }

have() { command -v "$1" >/dev/null 2>&1; }

pm_install() {
  if have pkg; then
    pkg install -y "$@"
  elif have pacman; then
    pacman -S --needed --noconfirm "$@"
  else
    log "No supported package manager found (pkg/pacman)."
    exit 1
  fi
}

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

install_base_deps() {
  log "Installing base dependencies..."
  pm_install git curl jq fzf tmux fish starship peaclock matugen termux-api eza zoxide
}

install_go_if_missing() {
  if ! have go; then
    log "Installing Go..."
    pm_install golang
  fi
}

install_matugen() {
  if have matugen; then
    log "matugen already available"
    return 0
  fi
  log "Installing matugen..."
  pm_install matugen
}

ensure_storage_links() {
  link_storage() {
    link_name="$1"
    target_path="$2"
    if [ -L "$target_path" ] || [ -d "$target_path" ]; then
      ln -sfn "$target_path" "$HOME_DIR/$link_name"
    fi
  }

  link_storage "Downloads" "$HOME_DIR/storage/downloads"
  link_storage "Pictures" "$HOME_DIR/storage/pictures"
  link_storage "Shared" "$HOME_DIR/storage/shared"

  if [ ! -d "$HOME_DIR/storage" ]; then
    log "Storage links skipped. Run termux-setup-storage later if you want Downloads/Pictures/Shared symlinks."
  fi
}

build_theme_manager() {
  install_go_if_missing
  log "Building tooie binary..."
  (cd "$REPO_DIR" && go build -o "$BIN_DIR/tooie" ./cmd/tooie)
  chmod +x "$BIN_DIR/tooie"
}

deploy_assets() {
  log "Deploying configs and scripts..."
  touch "$HOME_DIR/.hushlogin"
  install_file "$REPO_DIR/assets/defaults/.tmux.conf" "$HOME_DIR/.tmux.conf"

  install_file "$REPO_DIR/assets/defaults/.termux/termux.properties" "$HOME_DIR/.termux/termux.properties"
  install_file "$REPO_DIR/assets/defaults/.termux/colors.properties" "$HOME_DIR/.termux/colors.properties"
  install_file "$REPO_DIR/assets/defaults/.termux/font.ttf" "$HOME_DIR/.termux/font.ttf"
  install_file "$REPO_DIR/assets/defaults/.termux/font-italic.ttf" "$HOME_DIR/.termux/font-italic.ttf"
  backup_if_exists "$HOME_DIR/.termux/bin"
  mkdir -p "$HOME_DIR/.termux/bin"

  install_file "$REPO_DIR/assets/defaults/.config/starship.toml" "$HOME_DIR/.config/starship.toml"
  install_file "$REPO_DIR/assets/defaults/.config/fish/config.fish" "$HOME_DIR/.config/fish/config.fish"
  install_file "$REPO_DIR/assets/defaults/.config/peaclock/config" "$HOME_DIR/.config/peaclock/config"
  install_file "$REPO_DIR/assets/defaults/.launcherctl/config.json" "$HOME_DIR/.launcherctl/config.json"

  install_dir "$REPO_DIR/assets/defaults/.config/tmux" "$HOME_DIR/.config/tmux"
  install_dir "$REPO_DIR/fonts" "$TOOIE_DIR/fonts"
  install_dir "$REPO_DIR/assets/defaults" "$TOOIE_DIR/bootstrap-defaults"

  install_file "$REPO_DIR/scripts/apply-material.sh" "$TOOIE_DIR/apply-material.sh"
  install_file "$REPO_DIR/scripts/restore-material.sh" "$TOOIE_DIR/restore-material.sh"
  install_file "$REPO_DIR/scripts/list-material-backups.sh" "$TOOIE_DIR/list-material-backups.sh"
  install_file "$REPO_DIR/scripts/reset-bootstrap-defaults.sh" "$TOOIE_DIR/reset-bootstrap-defaults.sh"
  install_file "$REPO_DIR/scripts/setup-btop-shizuku.sh" "$TOOIE_DIR/setup-btop-shizuku.sh"
  chmod +x "$TOOIE_DIR/apply-material.sh" "$TOOIE_DIR/restore-material.sh" "$TOOIE_DIR/list-material-backups.sh" "$TOOIE_DIR/reset-bootstrap-defaults.sh" "$TOOIE_DIR/setup-btop-shizuku.sh"
}

post_setup() {
  if have termux-reload-settings; then
    termux-reload-settings || true
  fi

  if have tmux; then
    if [ -n "${TMUX:-}" ] || tmux ls >/dev/null 2>&1; then
      (tmux source-file "$HOME_DIR/.tmux.conf" >/dev/null 2>&1 || true) &
    fi
  fi

  log "Install complete."
  log "Backup snapshot: $BACKUP_DIR"
  log "Run: $BIN_DIR/tooie"
}

install_base_deps
install_matugen
ensure_storage_links
deploy_assets
build_theme_manager
post_setup
