#!/data/data/com.termux/files/usr/bin/sh
set -eu

REPO_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
STATE_DIR="$HOME_DIR/.local/state/tooie-theme-manager"
BACKUP_DIR="$STATE_DIR/backups/$(date +%Y%m%d-%H%M%S)"
BIN_DIR="$HOME_DIR/.local/bin"
THEME_DIR="$HOME_DIR/files/theme"

mkdir -p "$BACKUP_DIR" "$BIN_DIR" "$THEME_DIR" "$STATE_DIR"

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
  src="$1"
  if [ -e "$src" ]; then
    rel="$(printf '%s' "$src" | sed "s#^$HOME_DIR/##")"
    dst="$BACKUP_DIR/$rel"
    mkdir -p "$(dirname "$dst")"
    cp -a "$src" "$dst"
  fi
}

install_file() {
  src="$1"
  dst="$2"
  backup_if_exists "$dst"
  mkdir -p "$(dirname "$dst")"
  cp "$src" "$dst"
}

install_dir() {
  src="$1"
  dst="$2"
  backup_if_exists "$dst"
  mkdir -p "$(dirname "$dst")"
  rm -rf "$dst"
  cp -a "$src" "$dst"
}

install_base_deps() {
  log "Installing base dependencies..."
  pm_install git curl jq fzf tmux fish starship peaclock
}

install_go_if_missing() {
  if ! have go; then
    log "Installing Go..."
    pm_install golang
  fi
}

install_matugen() {
  if have matugen || [ -x "$HOME_DIR/.cargo/bin/matugen" ] || [ -x "$HOME_DIR/cargo/bin/matugen" ]; then
    log "matugen already available"
    return 0
  fi

  if ! have cargo; then
    log "Installing Rust toolchain for matugen..."
    pm_install rust
  fi

  log "Installing matugen via cargo..."
  cargo install matugen || true
}

install_neovim_nightly() {
  if have nvim && nvim --version 2>/dev/null | head -n1 | grep -qi nightly; then
    log "Neovim nightly already installed"
    return 0
  fi

  log "Attempting Neovim nightly via bob..."
  if ! have cargo; then
    pm_install rust
  fi

  if ! have bob; then
    cargo install bob-nvim || true
  fi

  if have bob; then
    bob install nightly || true
    bob use nightly || true
  fi

  if ! have nvim; then
    log "Falling back to packaged neovim"
    pm_install neovim
  fi
}

build_theme_manager() {
  install_go_if_missing
  log "Building tooie-theme-manager binary..."
  (cd "$REPO_DIR" && go build -o "$THEME_DIR/tooie-theme-manager" ./cmd/tooie-theme-manager)
  cp "$THEME_DIR/tooie-theme-manager" "$BIN_DIR/tooie-theme-manager"
  chmod +x "$THEME_DIR/tooie-theme-manager" "$BIN_DIR/tooie-theme-manager"
}

deploy_assets() {
  log "Deploying configs and scripts..."
  install_file "$REPO_DIR/assets/defaults/.tmux.conf" "$HOME_DIR/.tmux.conf"

  install_file "$REPO_DIR/assets/defaults/.termux/termux.properties" "$HOME_DIR/.termux/termux.properties"
  install_file "$REPO_DIR/assets/defaults/.termux/colors.properties" "$HOME_DIR/.termux/colors.properties"
  install_file "$REPO_DIR/assets/defaults/.termux/font.ttf" "$HOME_DIR/.termux/font.ttf"
  install_file "$REPO_DIR/assets/defaults/.termux/font-italic.ttf" "$HOME_DIR/.termux/font-italic.ttf"

  install_file "$REPO_DIR/assets/defaults/.config/starship.toml" "$HOME_DIR/.config/starship.toml"
  install_file "$REPO_DIR/assets/defaults/.config/fish/config.fish" "$HOME_DIR/.config/fish/config.fish"
  install_file "$REPO_DIR/assets/defaults/.config/peaclock/config" "$HOME_DIR/.config/peaclock/config"

  install_dir "$REPO_DIR/assets/defaults/.config/tmux" "$HOME_DIR/.config/tmux"
  install_dir "$REPO_DIR/assets/defaults/.config/nvim" "$HOME_DIR/.config/nvim"

  install_file "$REPO_DIR/scripts/apply-material.sh" "$THEME_DIR/apply-material.sh"
  install_file "$REPO_DIR/scripts/restore-material.sh" "$THEME_DIR/restore-material.sh"
  install_file "$REPO_DIR/scripts/list-material-backups.sh" "$THEME_DIR/list-material-backups.sh"
  chmod +x "$THEME_DIR/apply-material.sh" "$THEME_DIR/restore-material.sh" "$THEME_DIR/list-material-backups.sh"
}

post_setup() {
  if have termux-reload-settings; then
    termux-reload-settings || true
  fi

  if have tmux; then
    tmux source-file "$HOME_DIR/.tmux.conf" 2>/dev/null || true
  fi

  log "Install complete."
  log "Backup snapshot: $BACKUP_DIR"
  log "Run: $THEME_DIR/tooie-theme-manager"
}

install_base_deps
install_matugen
install_neovim_nightly
deploy_assets
build_theme_manager
post_setup
