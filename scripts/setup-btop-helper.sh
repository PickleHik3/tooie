#!/usr/bin/env sh
set -eu

HOME_DIR="${HOME:-$PWD}"
TOOIE_DIR="$HOME_DIR/.config/tooie"
CACHE_DIR="$HOME_DIR/.cache/tooie"
RUNNER="auto"
FISH_CONFIG="$HOME_DIR/.config/fish/config.fish"
TMP_DIR="$HOME_DIR/.tmp"

ANDROID_BTOP_DIR="/data/local/tmp/btop"
ANDROID_BTOP_BIN="$ANDROID_BTOP_DIR/btop"
ANDROID_CFG_STD_DIR="/data/local/tmp/btop-config/btop"
ANDROID_CFG_MINI_DIR="/data/local/tmp/btop-mini/btop"
BTOP_RELEASE_REPO="${TOOIE_BTOP_RELEASE_REPO:-aristocratos/btop}"
BTOP_RELEASE_TAG="${TOOIE_BTOP_RELEASE_TAG:-latest}"
BTOP_RELEASE_URL="${TOOIE_BTOP_RELEASE_URL:-}"

log() { printf '%s\n' "$*"; }
fail() { printf 'setup-btop-helper: %s\n' "$*" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --runner)
      shift
      RUNNER="${1:-auto}"
      ;;
    *)
      ;;
  esac
  shift || true
done

mkdir -p "$TOOIE_DIR" "$CACHE_DIR" "$TMP_DIR"

read_runner_from_config() {
  helper_cfg="$TOOIE_DIR/helper.json"
  [ -f "$helper_cfg" ] || return 1
  r="$(sed -n 's/.*"runner"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$helper_cfg" | head -n 1)"
  [ -n "$r" ] || return 1
  printf '%s' "$r"
  return 0
}

normalize_runner() {
  in="$(printf '%s' "${1:-auto}" | tr '[:upper:]' '[:lower:]')"
  case "$in" in
    auto)
      if cfg_runner="$(read_runner_from_config 2>/dev/null || true)"; then
        case "$cfg_runner" in
          rish|root|su|tsu|sudo) printf '%s' "$cfg_runner"; return 0 ;;
        esac
      fi
      if command -v rish >/dev/null 2>&1; then
        printf '%s' "rish"
      else
        printf '%s' "auto"
      fi
      ;;
    rish|root|su|tsu|sudo)
      printf '%s' "$in"
      ;;
    *)
      printf '%s' "auto"
      ;;
  esac
}

write_helper_config() {
  selected_runner="$1"
  cat > "$TOOIE_DIR/helper.json" <<JSON
{
  "runner": "${selected_runner}"
}
JSON
}

seed_helper_stats() {
  # Seed helper-stats file so users can inspect/override the shape immediately.
  cat > "$CACHE_DIR/helper-stats.json" <<'JSON'
{
  "cpuPercent": 0,
  "memUsedBytes": 0,
  "memTotalBytes": 0,
  "battery": {
    "levelPercent": 0,
    "charging": false
  },
  "source": "btop-helper",
  "updatedAt": ""
}
JSON
}

rish_exec() {
  cmd="$1"
  rish -c "$cmd"
}

machine_to_asset_candidates() {
  m="$(uname -m 2>/dev/null || echo unknown)"
  case "$m" in
    aarch64|arm64)
      printf '%s\n' \
        "btop-aarch64-unknown-linux-musl.tbz" \
        "btop-aarch64-linux-musl.tbz"
      ;;
    x86_64|amd64)
      printf '%s\n' \
        "btop-x86_64-unknown-linux-musl.tbz" \
        "btop-x86_64-linux-musl.tbz"
      ;;
    armv7l|armv7|arm)
      printf '%s\n' \
        "btop-armv7-unknown-linux-musleabihf.tbz" \
        "btop-armv7l-linux-musleabihf.tbz"
      ;;
    i686|i386)
      printf '%s\n' \
        "btop-i686-unknown-linux-musl.tbz" \
        "btop-i686-linux-musl.tbz"
      ;;
    *)
      return 1
      ;;
  esac
}

release_asset_url() {
  asset="$1"
  if [ -n "$BTOP_RELEASE_URL" ]; then
    printf '%s\n' "$BTOP_RELEASE_URL"
    return 0
  fi
  if [ "$BTOP_RELEASE_TAG" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "$BTOP_RELEASE_REPO" "$asset"
  else
    printf 'https://github.com/%s/releases/download/%s/%s\n' "$BTOP_RELEASE_REPO" "$BTOP_RELEASE_TAG" "$asset"
  fi
}

download_release_btop() {
  command -v curl >/dev/null 2>&1 || fail "missing curl (required to fetch btop release)"
  command -v tar >/dev/null 2>&1 || fail "missing tar (required to extract btop release)"
  work="$TMP_DIR/tooie-btop-release"
  rm -rf "$work"
  mkdir -p "$work/extract"

  if [ -n "$BTOP_RELEASE_URL" ]; then
    archive="$work/btop.tbz"
    if curl -fsSL "$BTOP_RELEASE_URL" -o "$archive"; then
      tar -xjf "$archive" -C "$work/extract" || fail "failed extracting archive from TOOIE_BTOP_RELEASE_URL"
      found="$(find "$work/extract" -type f -name btop | head -n 1 || true)"
      [ -n "$found" ] || fail "downloaded archive did not contain a btop binary"
      chmod 755 "$found"
      printf '%s' "$found"
      return 0
    fi
    fail "failed downloading TOOIE_BTOP_RELEASE_URL"
  fi

  candidates="$(machine_to_asset_candidates || true)"
  [ -n "$candidates" ] || return 1

  old_ifs="${IFS:-}"
  IFS='
'
  for asset in $candidates; do
    [ -n "$asset" ] || continue
    url="$(release_asset_url "$asset")"
    archive="$work/$asset"
    if ! curl -fsSL "$url" -o "$archive"; then
      continue
    fi
    rm -rf "$work/extract"
    mkdir -p "$work/extract"
    if ! tar -xjf "$archive" -C "$work/extract"; then
      continue
    fi
    found="$(find "$work/extract" -type f -name btop | head -n 1 || true)"
    if [ -n "$found" ]; then
      chmod 755 "$found"
      printf '%s' "$found"
      IFS="$old_ifs"
      return 0
    fi
  done
  IFS="$old_ifs"
  return 1
}

deploy_btop_via_rish() {
  command -v rish >/dev/null 2>&1 || fail "runner=rish requires 'rish' in PATH"
  src_btop="$(download_release_btop || true)"
  [ -n "$src_btop" ] || fail "failed to fetch a suitable btop release archive for this device"
  [ -f "$src_btop" ] || fail "local btop binary not found: $src_btop"

  std_cfg="$TMP_DIR/tooie-btop.conf"
  mini_cfg="$TMP_DIR/tooie-mini-btop.conf"
  cat > "$std_cfg" <<'CONF'
shown_boxes = "cpu mem proc"
io_mode = False
show_io_stat = False
net_auto = False
net_sync = False
show_disks = False
update_ms = 2000
CONF
  cat > "$mini_cfg" <<'CONF'
shown_boxes = "cpu proc"
io_mode = False
show_io_stat = False
net_auto = False
net_sync = False
show_disks = False
cpu_single_graph = True
update_ms = 2000
CONF

  rish_exec "set -eu; mkdir -p '$ANDROID_BTOP_DIR' '$ANDROID_CFG_STD_DIR' '$ANDROID_CFG_MINI_DIR'"
  cat "$src_btop" | rish_exec "set -eu; cat > '$ANDROID_BTOP_BIN'; chmod 755 '$ANDROID_BTOP_BIN'"
  cat "$std_cfg" | rish_exec "set -eu; cat > '$ANDROID_CFG_STD_DIR/btop.conf'"
  cat "$mini_cfg" | rish_exec "set -eu; cat > '$ANDROID_CFG_MINI_DIR/btop.conf'"
}

write_fish_abbr_block() {
  mkdir -p "$(dirname "$FISH_CONFIG")"
  touch "$FISH_CONFIG"
  begin="# >>> tooie-btop >>>"
  end="# <<< tooie-btop <<<"
  tmp="$TMP_DIR/fish-config.tmp"
  awk -v begin="$begin" -v end="$end" '
    $0 == begin {skip=1; next}
    $0 == end {skip=0; next}
    !skip {print}
  ' "$FISH_CONFIG" > "$tmp"
  cat >> "$tmp" <<'FISH'
# >>> tooie-btop >>>
if status is-interactive
    abbr --erase btop >/dev/null 2>&1
    abbr --erase mini-btop >/dev/null 2>&1
    abbr -a btop 'rish -c "XDG_CONFIG_HOME=/data/local/tmp/btop-config /data/local/tmp/btop/btop --force-utf"'
    abbr -a mini-btop 'rish -c "XDG_CONFIG_HOME=/data/local/tmp/btop-mini /data/local/tmp/btop/btop --force-utf"'
end
# <<< tooie-btop <<<
FISH
  mv "$tmp" "$FISH_CONFIG"

  if command -v fish >/dev/null 2>&1; then
    fish -c 'abbr --erase btop >/dev/null 2>&1; abbr --erase mini-btop >/dev/null 2>&1; abbr -a btop '\''rish -c "XDG_CONFIG_HOME=/data/local/tmp/btop-config /data/local/tmp/btop/btop --force-utf"'\''; abbr -a mini-btop '\''rish -c "XDG_CONFIG_HOME=/data/local/tmp/btop-mini /data/local/tmp/btop/btop --force-utf"'\''' >/dev/null 2>&1 || true
  fi
}

main() {
  runner="$(normalize_runner "$RUNNER")"
  write_helper_config "$runner"
  seed_helper_stats

  case "$runner" in
    rish)
      deploy_btop_via_rish
      write_fish_abbr_block
      log "btop helper configured (runner=${runner})."
      log "btop source: github.com/${BTOP_RELEASE_REPO} (${BTOP_RELEASE_TAG})"
      log "remote binary: $ANDROID_BTOP_BIN"
      log "fish abbreviations installed: btop, mini-btop"
      ;;
    root|su|tsu|sudo)
      log "btop helper configured (runner=${runner})."
      log "runner ${runner} is configured, but automatic remote btop deployment is currently implemented only for runner=rish."
      ;;
    *)
      log "btop helper configured (runner=${runner})."
      log "no privileged backend available for remote btop deployment."
      ;;
  esac
  log "Optional stats override file: $CACHE_DIR/helper-stats.json"
}

main "$@"
