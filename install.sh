#!/usr/bin/env sh
set -eu

REPO_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
HOME_DIR="${HOME:-$PWD}"
BIN_DIR="$HOME_DIR/.local/bin"

log() { printf '%s\n' "$*"; }
have() { command -v "$1" >/dev/null 2>&1; }

pm_install() {
  if have pkg; then
    pkg install -y "$@"
  elif have pacman; then
    pacman -S --needed --noconfirm "$@"
  elif have apt-get; then
    apt-get update -y
    apt-get install -y "$@"
  elif have dnf; then
    dnf install -y "$@"
  elif have yum; then
    yum install -y "$@"
  else
    log "No supported package manager found (pkg/pacman/apt-get/dnf/yum)."
    return 1
  fi
}

ensure_prereqs() {
  need_pkgs=""
  have go || need_pkgs="$need_pkgs golang"
  have gum || need_pkgs="$need_pkgs gum"

  if [ -n "$(printf '%s' "$need_pkgs" | tr -d ' ')" ]; then
    log "Installing missing prerequisites:$need_pkgs"
    # shellcheck disable=SC2086
    pm_install $need_pkgs
  fi
}

mkdir -p "$BIN_DIR"
ensure_prereqs || {
  log "Install prerequisites manually, then rerun ./install.sh"
  exit 1
}

log "Building tooie binary..."
(cd "$REPO_DIR" && go build -o "$BIN_DIR/tooie" ./cmd/tooie)
chmod +x "$BIN_DIR/tooie"

log "Launching guided setup..."
exec env TOOIE_REPO_DIR="$REPO_DIR" "$BIN_DIR/tooie" setup
