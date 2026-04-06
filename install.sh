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

csv_has() {
  csv="$1"
  item="$2"
  case ",$csv," in
    *",$item,"*) return 0 ;;
    *) return 1 ;;
  esac
}

csv_append_unique() {
  cur="$1"
  val="$2"
  if [ -z "$val" ]; then
    printf '%s' "$cur"
    return 0
  fi
  if csv_has "$cur" "$val"; then
    printf '%s' "$cur"
    return 0
  fi
  if [ -n "$cur" ]; then
    printf '%s,%s' "$cur" "$val"
  else
    printf '%s' "$val"
  fi
}

theme_selection_has() {
  selection="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
  key="$2"
  if csv_has "$selection" "all"; then
    return 0
  fi
  case "$key" in
    starship)
      if csv_has "$selection" "starship" || csv_has "$selection" "shell" || csv_has "$selection" "starship+eza+peaclock"; then
        return 0
      fi
      return 1
      ;;
    tmux|termux)
      csv_has "$selection" "$key"
      return $?
      ;;
    *)
      return 1
      ;;
  esac
}

prompt_multi_theme_items() {
  prompt="$1"
  default_csv="$(printf '%s' "$2" | tr '[:upper:]' '[:lower:]')"
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
  printf 'Select one or more [1-%d], comma separated (default %s): ' "$#" "$default_csv" >&2
  IFS= read -r choice || choice=""
  if [ -z "$choice" ]; then
    printf '%s' "$default_csv"
    return 0
  fi

  picked=""
  normalized="$(printf '%s' "$choice" | tr ',;' '  ')"
  for tok in $normalized; do
    case "$tok" in
      ''|*[!0-9]*)
        err "invalid choice token: $tok"
        return 1
        ;;
    esac
    if [ "$tok" -lt 1 ] || [ "$tok" -gt "$#" ]; then
      err "choice out of range: $tok"
      return 1
    fi
    eval "item=\${$tok}"
    item="$(printf '%s' "$item" | tr '[:upper:]' '[:lower:]')"
    picked="$(csv_append_unique "$picked" "$item")"
  done
  if [ -z "$picked" ]; then
    err "no items selected"
    return 1
  fi
  printf '%s' "$picked"
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
    pkg:zoxide|pacman:zoxide|apt:zoxide|dnf:zoxide) printf '%s' "zoxide" ;;
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
  fish_mode="$2"
  starship_mode="$3"
  deps=""
  deps="$(append_unique_word "$deps" "go")"

  if theme_selection_has "$selection" "tmux"; then
    deps="$(append_unique_word "$deps" "tmux")"
    deps="$(append_unique_word "$deps" "jq")"
    deps="$(append_unique_word "$deps" "matugen")"
  fi
  if theme_selection_has "$selection" "termux"; then
    deps="$(append_unique_word "$deps" "jq")"
    deps="$(append_unique_word "$deps" "matugen")"
  fi
  if theme_selection_has "$selection" "starship"; then
    deps="$(append_unique_word "$deps" "jq")"
    deps="$(append_unique_word "$deps" "matugen")"
    deps="$(append_unique_word "$deps" "eza")"
    deps="$(append_unique_word "$deps" "peaclock")"
  fi

  if [ "$fish_mode" = "on" ]; then
    deps="$(append_unique_word "$deps" "fish")"
    deps="$(append_unique_word "$deps" "eza")"
    deps="$(append_unique_word "$deps" "zoxide")"
  fi
  if [ "$starship_mode" != "off" ]; then
    deps="$(append_unique_word "$deps" "starship")"
  fi

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
fish_mode="off"
starship_mode="off"
btop_mode="auto"

if [ "$platform" = "termux" ]; then
  backend_pick="$(prompt_menu "Choose Termux elevated backend" 1 "none" "rish" "root" "shizuku")"
  backend="$backend_pick"
fi

if [ "$platform" = "linux" ]; then
  theme_items="$(prompt_multi_theme_items "Choose items to theme" "all" "all" "tmux" "starship")"
else
  theme_items="$(prompt_multi_theme_items "Choose items to theme" "all" "all" "tmux" "termux" "starship")"
fi

if [ "$platform" = "linux" ] && theme_selection_has "$theme_items" "termux"; then
  err "termux item is only valid on termux platform."
  exit 1
fi

if theme_selection_has "$theme_items" "starship"; then
  starship_mode="themed"
  fish_pick="$(prompt_menu "Install fish bootstrap snippet + managed bootstrap file?" 1 "yes" "no")"
  case "$fish_pick" in
    yes) fish_mode="on" ;;
    no) fish_mode="off" ;;
    *) err "unknown fish selection: $fish_pick"; exit 1 ;;
  esac
fi

if [ "$platform" = "termux" ] && { [ "$backend" = "rish" ] || [ "$backend" = "shizuku" ]; }; then
  btop_pick="$(prompt_menu "Setup remote btop helper + fish aliases (btop/mini-btop)?" 2 "yes" "no")"
  case "$btop_pick" in
    yes) btop_mode="on" ;;
    no) btop_mode="off" ;;
    *) err "unknown btop selection: $btop_pick"; exit 1 ;;
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

if [ "$platform" = "termux" ] && [ "$backend" = "shizuku" ] && [ "$btop_mode" = "on" ]; then
  if ! have rish; then
    err "btop helper for backend=shizuku requires a working 'rish' binary in PATH."
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

logical_deps="$(resolve_logical_deps "$theme_items" "$fish_mode" "$starship_mode")"
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

fish_summary="$fish_mode"
starship_summary="$starship_mode"
if ! theme_selection_has "$theme_items" "starship"; then
  fish_summary="N/A"
  starship_summary="N/A"
fi

printf '\nInstallation summary\n'
printf '  platform:      %s\n' "$platform"
if [ "$platform" = "termux" ]; then
  printf '  backend:       %s\n' "$backend"
fi
printf '  themed items:  %s\n' "$theme_items"
printf '  fish:          %s\n' "$fish_summary"
printf '  starship:      %s\n' "$starship_summary"
if [ "$platform" = "termux" ] && { [ "$backend" = "rish" ] || [ "$backend" = "shizuku" ]; }; then
  printf '  btop helper:   %s\n' "$btop_mode"
  if [ "$btop_mode" = "on" ]; then
    printf '  btop aliases:  btop, mini-btop\n'
  fi
fi
printf '  package mgr:   %s\n' "$pm"
printf '  packages:      %s\n' "$resolved_pkgs"
if [ -n "$missing_pkgs" ]; then
  printf '  unavailable:   %s (will be skipped)\n' "$missing_pkgs"
fi
printf '  binary target: %s\n' "$BIN_DIR/tooie"
printf '  config root:   %s\n' "$HOME_DIR/.config/tooie"
printf '  wallpaper cmd: %s\n' "tooie /path/to/wallpaper.jpg"

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
  --install-items "$theme_items" \
  --install-fish "$fish_mode" \
  --install-starship "$starship_mode" \
  --install-btop "$btop_mode"
