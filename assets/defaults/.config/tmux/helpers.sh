#!/usr/bin/env sh

get_tmux_option() {
  key="$1"
  default="${2-}"
  if command -v tmux >/dev/null 2>&1; then
    v="$(tmux show-option -gqv "$key" 2>/dev/null || true)"
  else
    v=""
  fi
  if [ -n "$v" ]; then
    printf '%s' "$v"
  else
    printf '%s' "$default"
  fi
}

is_on() {
  case "$1" in
    1|on|true|yes|enabled) return 0 ;;
    *) return 1 ;;
  esac
}

load_tmux_profile_env() {
  profile_file="$1"
  if [ ! -f "$profile_file" ]; then
    return 0
  fi
  set -a
  # shellcheck disable=SC1090
  . "$profile_file"
  set +a
}
