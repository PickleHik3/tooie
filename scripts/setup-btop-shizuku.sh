#!/usr/bin/env sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_DIR="$HOME_DIR/.config/tooie"
LAUNCHERCTL_DIR="$HOME_DIR/.launcherctl"
LAUNCHERCTL_CONFIG="$LAUNCHERCTL_DIR/config.json"
FISH_CONFIG="$HOME_DIR/.config/fish/config.fish"

ANDROID_BTOP_DIR="/data/local/tmp/btop"
ANDROID_BTOP_BIN="$ANDROID_BTOP_DIR/btop"
ANDROID_CFG_STD_DIR="/data/local/tmp/btop-config/btop"
ANDROID_CFG_MINI_DIR="/data/local/tmp/btop-mini/btop"

BTOP_RELEASE_ASSET="${TOOIE_BTOP_RELEASE_ASSET:-btop-aarch64-unknown-linux-musl.tbz}"
BTOP_RELEASE_TAG="${TOOIE_BTOP_RELEASE_TAG:-latest}"
BTOP_RELEASE_URL="${TOOIE_BTOP_RELEASE_URL:-}"
BTOP_RELEASE_REPO="${TOOIE_BTOP_RELEASE_REPO:-}"

log() { printf '%s\n' "$*"; }
fail() { printf 'Setup failed: %s\n' "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

json_get_output() {
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$1" | jq -r '.output // ""'
  else
    printf '%s' "$1"
  fi
}

ensure_exec_policy() {
  [ -f "$LAUNCHERCTL_CONFIG" ] || fail "missing $LAUNCHERCTL_CONFIG"

  if command -v jq >/dev/null 2>&1; then
    tmp="$HOME_DIR/.tmp/launcherctl-config.json.tmp"
    mkdir -p "$HOME_DIR/.tmp"
    jq '
      .execEnabled = true
      | .allowedCommandPrefixes = ((.allowedCommandPrefixes // []) + ["sh -c"] | unique)
    ' "$LAUNCHERCTL_CONFIG" > "$tmp"
    mv "$tmp" "$LAUNCHERCTL_CONFIG"
  else
    fail "jq is required to patch launcherctl exec policy"
  fi
}

launcher_exec_raw() {
  cmd="$1"
  endpoint="$(cat "$LAUNCHERCTL_DIR/endpoint" 2>/dev/null || true)"
  token="$(cat "$LAUNCHERCTL_DIR/token" 2>/dev/null || true)"
  [ -n "$endpoint" ] || fail "missing launcherctl endpoint"
  [ -n "$token" ] || fail "missing launcherctl token"
  payload="$(jq -cn --arg command "$cmd" '{command: $command}')"
  curl -sS --connect-timeout 3 --max-time 120 \
    -X POST \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    --data "$payload" \
    "$(printf '%s' "$endpoint" | sed 's#/$##')/v1/exec"
}

launcher_exec() {
  cmd="$1"
  raw="$(launcher_exec_raw "$cmd")" || return 1
  if command -v jq >/dev/null 2>&1; then
    ok="$(printf '%s' "$raw" | jq -r '.ok // false')"
    out="$(printf '%s' "$raw" | jq -r '.output // ""')"
    if [ "$ok" != "true" ]; then
      [ -n "$out" ] && printf '%s\n' "$out" >&2
      return 1
    fi
    printf '%s' "$out"
    return 0
  fi
  json_get_output "$raw"
}

resolve_download_url() {
  if [ -n "$BTOP_RELEASE_URL" ]; then
    printf '%s\n' "$BTOP_RELEASE_URL"
    return 0
  fi

  if [ -n "$BTOP_RELEASE_REPO" ]; then
    if [ "$BTOP_RELEASE_TAG" = "latest" ]; then
      printf 'https://github.com/%s/releases/latest/download/%s\n' "$BTOP_RELEASE_REPO" "$BTOP_RELEASE_ASSET"
    else
      printf 'https://github.com/%s/releases/download/%s/%s\n' "$BTOP_RELEASE_REPO" "$BTOP_RELEASE_TAG" "$BTOP_RELEASE_ASSET"
    fi
    return 0
  fi

  if [ "$BTOP_RELEASE_TAG" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "aristocratos/btop" "$BTOP_RELEASE_ASSET"
  else
    printf 'https://github.com/%s/releases/download/%s/%s\n' "aristocratos/btop" "$BTOP_RELEASE_TAG" "$BTOP_RELEASE_ASSET"
  fi
}

setup_remote_btop() {
  download_url="$1"
  case "$download_url" in
    *"'"*) return 1 ;;
  esac

  # Download/extract and normalize the executable path.
  launcher_exec "sh -c 'set -eu; mkdir -p $ANDROID_BTOP_DIR; rm -rf $ANDROID_BTOP_DIR/extract'" >/dev/null || return 1
  launcher_exec "sh -c 'set -eu; curl -fsSL \"$download_url\" -o $ANDROID_BTOP_DIR/btop.tbz'" >/dev/null || return 1
  launcher_exec "sh -c 'set -eu; mkdir -p $ANDROID_BTOP_DIR/extract; tar -xjf $ANDROID_BTOP_DIR/btop.tbz -C $ANDROID_BTOP_DIR/extract'" >/dev/null || return 1
  launcher_exec "sh -c 'set -eu; src=\$(find $ANDROID_BTOP_DIR/extract -type f -name btop | head -n 1 || true); [ -n \"\$src\" ] || exit 1; cp \"\$src\" $ANDROID_BTOP_BIN; chmod 755 $ANDROID_BTOP_BIN'" >/dev/null || return 1

  launcher_exec "sh -c 'set -eu; mkdir -p $ANDROID_CFG_STD_DIR; printf \"%s\n\" \"shown_boxes = \\\"cpu mem proc\\\"\" \"io_mode = False\" \"show_io_stat = False\" \"net_auto = False\" \"net_sync = False\" \"show_disks = False\" \"update_ms = 2000\" > $ANDROID_CFG_STD_DIR/btop.conf'" >/dev/null || return 1

  launcher_exec "sh -c 'set -eu; mkdir -p $ANDROID_CFG_MINI_DIR; printf \"%s\n\" \"shown_boxes = \\\"cpu proc\\\"\" \"io_mode = False\" \"show_io_stat = False\" \"net_auto = False\" \"net_sync = False\" \"show_disks = False\" \"cpu_single_graph = True\" \"update_ms = 2000\" > $ANDROID_CFG_MINI_DIR/btop.conf'" >/dev/null || return 1

  return 0
}

write_fish_abbr() {
  mkdir -p "$(dirname "$FISH_CONFIG")"
  touch "$FISH_CONFIG"
  begin="# >>> tooie-btop >>>"
  end="# <<< tooie-btop <<<"
  tmp="$HOME_DIR/.tmp/fish-config.tmp"
  mkdir -p "$HOME_DIR/.tmp"
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
    abbr -a btop 'launcherctl tty-exec "XDG_CONFIG_HOME=/data/local/tmp/btop-config /data/local/tmp/btop/btop --force-utf"'
    abbr -a mini-btop 'launcherctl tty-exec "XDG_CONFIG_HOME=/data/local/tmp/btop-mini /data/local/tmp/btop/btop --force-utf"'
end
# <<< tooie-btop <<<
FISH
  mv "$tmp" "$FISH_CONFIG"

  if command -v fish >/dev/null 2>&1; then
    fish -c 'abbr --erase btop >/dev/null 2>&1; abbr --erase mini-btop >/dev/null 2>&1; abbr -a btop '\''launcherctl tty-exec "XDG_CONFIG_HOME=/data/local/tmp/btop-config /data/local/tmp/btop/btop --force-utf"'\''; abbr -a mini-btop '\''launcherctl tty-exec "XDG_CONFIG_HOME=/data/local/tmp/btop-mini /data/local/tmp/btop/btop --force-utf"'\''' >/dev/null 2>&1 || true
  fi
}

main() {
  require_cmd launcherctl
  require_cmd curl
  require_cmd tar

  mkdir -p "$TOOIE_DIR"

  ensure_exec_policy
  launcher_exec "sh -c 'echo ready'" >/dev/null || fail "launcherctl exec is not available; verify Shizuku permission and endpoint settings"
  launcherctl tty-doctor >/dev/null || fail "launcherctl tty-exec prerequisites are not healthy; run 'launcherctl tty-doctor'"

  log "Setting up btop in privileged path..."
  success_url=""
  for url in $(resolve_download_url); do
    if setup_remote_btop "$url"; then
      success_url="$url"
      break
    fi
  done

  [ -n "$success_url" ] || fail "could not download/install btop from configured release source"

  write_fish_abbr

  log "Btop setup complete."
  log "Release URL: $success_url"
  log "Fish abbreviations installed: btop, mini-btop (launcherctl tty-exec)"
}

main "$@"
