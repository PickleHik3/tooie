#!/data/data/com.termux/files/usr/bin/sh

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
