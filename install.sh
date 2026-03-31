#!/usr/bin/env sh
set -eu

REPO_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
HOME_DIR="${HOME:-$PWD}"
BIN_DIR="$HOME_DIR/.local/bin"

log() { printf '%s\n' "$*"; }
err() { printf 'install.sh: %s\n' "$*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }

prompt_menu() {
  prompt="$1"
  default_idx="$2"
  shift 2
  if [ "$#" -eq 0 ]; then
    return 1
  fi
  printf '\n%s\n' "$prompt" >&2
  idx=1
  for item in "$@"; do
    printf '  %d) %s\n' "$idx" "$item" >&2
    idx=$((idx + 1))
  done
  printf 'Select [1-%d] (default %s): ' "$#" "$default_idx" >&2
  IFS= read -r choice || choice=""
  if [ -z "$choice" ]; then
    choice="$default_idx"
  fi
  case "$choice" in
    ''|*[!0-9]*)
      err "invalid choice: $choice"
      return 1
      ;;
  esac
  if [ "$choice" -lt 1 ] || [ "$choice" -gt "$#" ]; then
    err "choice out of range: $choice"
    return 1
  fi
  eval "printf '%s' \"\${$choice}\""
}

confirm_yes_no() {
  prompt="$1"
  printf '%s ' "$prompt" >&2
  IFS= read -r answer || answer=""
  case "$(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]')" in
    y|yes) return 0 ;;
    *) return 1 ;;
  esac
}

detect_pm() {
  if have pkg; then
    printf '%s' "pkg"
    return 0
  fi
  if have pacman; then
    printf '%s' "pacman"
    return 0
  fi
  if have apt-get; then
    printf '%s' "apt"
    return 0
  fi
  if have dnf; then
    printf '%s' "dnf"
    return 0
  fi
  return 1
}

pm_has_package() {
  pm="$1"
  pkg_name="$2"
  case "$pm" in
    pacman)
      pacman -Si "$pkg_name" >/dev/null 2>&1
      ;;
    *)
      return 0
      ;;
  esac
}

map_pkg_name() {
  pm="$1"
  logical="$2"
  case "$pm:$logical" in
    pkg:go) printf '%s' "golang" ;;
    pacman:go) printf '%s' "go" ;;
    apt:go) printf '%s' "golang-go" ;;
    dnf:go) printf '%s' "golang" ;;

    pkg:tmux|pacman:tmux|apt:tmux|dnf:tmux) printf '%s' "tmux" ;;
    pkg:jq|pacman:jq|apt:jq|dnf:jq) printf '%s' "jq" ;;
    pkg:fish|pacman:fish|apt:fish|dnf:fish) printf '%s' "fish" ;;
    pkg:starship|pacman:starship|apt:starship|dnf:starship) printf '%s' "starship" ;;
    pkg:eza|pacman:eza|apt:eza|dnf:eza) printf '%s' "eza" ;;
    pkg:peaclock|pacman:peaclock|apt:peaclock|dnf:peaclock) printf '%s' "peaclock" ;;
    pkg:matugen|pacman:matugen|apt:matugen|dnf:matugen) printf '%s' "matugen" ;;
    *) printf '%s' "" ;;
  esac
}

append_unique_word() {
  cur="$1"
  val="$2"
  if [ -z "$val" ]; then
    printf '%s' "$cur"
    return 0
  fi
  for w in $cur; do
    if [ "$w" = "$val" ]; then
      printf '%s' "$cur"
      return 0
    fi
  done
  if [ -n "$cur" ]; then
    printf '%s %s' "$cur" "$val"
  else
    printf '%s' "$val"
  fi
}

resolve_logical_deps() {
  selection="$1"
  deps=""
  deps="$(append_unique_word "$deps" "go")"

  case "$selection" in
    all)
      deps="$(append_unique_word "$deps" "tmux")"
      deps="$(append_unique_word "$deps" "jq")"
      deps="$(append_unique_word "$deps" "matugen")"
      deps="$(append_unique_word "$deps" "fish")"
      deps="$(append_unique_word "$deps" "starship")"
      deps="$(append_unique_word "$deps" "eza")"
      deps="$(append_unique_word "$deps" "peaclock")"
      ;;
    tmux)
      deps="$(append_unique_word "$deps" "tmux")"
      deps="$(append_unique_word "$deps" "jq")"
      deps="$(append_unique_word "$deps" "matugen")"
      ;;
    termux)
      deps="$(append_unique_word "$deps" "jq")"
      deps="$(append_unique_word "$deps" "matugen")"
      ;;
    shell)
      deps="$(append_unique_word "$deps" "jq")"
      deps="$(append_unique_word "$deps" "matugen")"
      deps="$(append_unique_word "$deps" "fish")"
      deps="$(append_unique_word "$deps" "starship")"
      deps="$(append_unique_word "$deps" "eza")"
      deps="$(append_unique_word "$deps" "peaclock")"
      ;;
  esac

  printf '%s' "$deps"
}

install_packages() {
  pm="$1"
  pkgs="$2"
  if [ -z "$pkgs" ]; then
    return 0
  fi

  need_sudo=0
  sudo_prefix=""
  if [ "$pm" != "pkg" ] && [ "$(id -u)" -ne 0 ]; then
    need_sudo=1
    if ! have sudo; then
      err "sudo is required for package installation on Linux"
      return 1
    fi
    log "Requesting elevation for package installation..."
    sudo -v
    sudo_prefix="sudo"
  fi

  case "$pm" in
    pkg)
      pkg update -y
      # shellcheck disable=SC2086
      pkg install -y $pkgs
      ;;
    pacman)
      # shellcheck disable=SC2086
      $sudo_prefix pacman -S --needed --noconfirm $pkgs
      ;;
    apt)
      $sudo_prefix apt-get update -y
      # shellcheck disable=SC2086
      $sudo_prefix apt-get install -y $pkgs
      ;;
    dnf)
      # shellcheck disable=SC2086
      $sudo_prefix dnf install -y $pkgs
      ;;
    *)
      err "unsupported package manager: $pm"
      return 1
      ;;
  esac

  if [ "$need_sudo" -eq 1 ]; then
    sudo -k || true
  fi
}

platform_pick="$(prompt_menu "Choose target platform" 1 "termux" "linux")"
platform="$platform_pick"
backend="none"

if [ "$platform" = "termux" ]; then
  backend_pick="$(prompt_menu "Choose Termux elevated backend" 1 "none" "rish" "root" "shizuku")"
  backend="$backend_pick"
fi

if [ "$platform" = "linux" ]; then
  items_pick="$(prompt_menu "Choose items to theme" 1 "all" "tmux" "starship+eza+peaclock")"
  case "$items_pick" in
    all) theme_items="all" ;;
    tmux) theme_items="tmux" ;;
    starship+eza+peaclock) theme_items="shell" ;;
    *) err "unknown items selection: $items_pick"; exit 1 ;;
  esac
else
  items_pick="$(prompt_menu "Choose items to theme" 1 "all" "tmux" "termux" "starship+eza+peaclock")"
  case "$items_pick" in
    all) theme_items="all" ;;
    tmux) theme_items="tmux" ;;
    termux) theme_items="termux" ;;
    starship+eza+peaclock) theme_items="shell" ;;
    *) err "unknown items selection: $items_pick"; exit 1 ;;
  esac
fi

if [ "$platform" = "termux" ] && [ "$backend" = "shizuku" ]; then
  if ! have launcherctl; then
    err "backend=shizuku requires launcherctl (termux-launcher build)."
    exit 1
  fi
fi

if [ "$platform" = "termux" ] && [ "$backend" = "rish" ]; then
  if ! have rish; then
    err "backend=rish requires a working 'rish' binary in PATH."
    exit 1
  fi
fi

if [ "$platform" = "termux" ] && [ "$backend" = "root" ]; then
  if ! have su && ! have tsu && ! have sudo; then
    err "backend=root requires one of: su, tsu, or sudo."
    exit 1
  fi
fi

pm="$(detect_pm || true)"
if [ -z "$pm" ]; then
  err "No supported package manager found (pkg/pacman/apt-get/dnf)."
  exit 1
fi

logical_deps="$(resolve_logical_deps "$theme_items")"
resolved_pkgs=""
missing_pkgs=""
for logical in $logical_deps; do
  pkg_name="$(map_pkg_name "$pm" "$logical")"
  if [ -z "$pkg_name" ]; then
    err "No package mapping for '$logical' on '$pm'."
    exit 1
  fi
  if pm_has_package "$pm" "$pkg_name"; then
    resolved_pkgs="$(append_unique_word "$resolved_pkgs" "$pkg_name")"
  else
    missing_pkgs="$(append_unique_word "$missing_pkgs" "$pkg_name")"
  fi
done

printf '\nInstallation summary\n'
printf '  platform:      %s\n' "$platform"
if [ "$platform" = "termux" ]; then
  printf '  backend:       %s\n' "$backend"
fi
printf '  themed items:  %s\n' "$theme_items"
printf '  package mgr:   %s\n' "$pm"
printf '  packages:      %s\n' "$resolved_pkgs"
if [ -n "$missing_pkgs" ]; then
  printf '  unavailable:   %s (will be skipped)\n' "$missing_pkgs"
fi
printf '  binary target: %s\n' "$BIN_DIR/tooie"
printf '  config root:   %s\n' "$HOME_DIR/.config/tooie"

if ! confirm_yes_no "Proceed with installation (y/n)?"; then
  log "Installation cancelled."
  exit 0
fi

log "Installing dependencies..."
install_packages "$pm" "$resolved_pkgs"

if ! have go; then
  err "go is required but was not found after dependency installation."
  exit 1
fi

mkdir -p "$BIN_DIR"
log "Building tooie binary..."
(cd "$REPO_DIR" && go build -o "$BIN_DIR/tooie" ./cmd/tooie)
chmod +x "$BIN_DIR/tooie"

log "Applying setup selection..."
exec env TOOIE_REPO_DIR="$REPO_DIR" "$BIN_DIR/tooie" setup --non-interactive \
  --install-platform "$platform" \
  --install-backend "$backend" \
  --install-items "$theme_items"
