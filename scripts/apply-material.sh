#!/data/data/com.termux/files/usr/bin/sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
THEME_DIR="$HOME_DIR/files/theme"
BACKUP_ROOT="$THEME_DIR/backups"
WALLPAPER_FIXED="$HOME_DIR/.termux/background/background.jpeg"
WALLPAPER_DIR="$HOME_DIR/.termux/backgrounds"
TERMUX_COLORS="$HOME_DIR/.termux/colors.properties"
TMUX_CONF="$HOME_DIR/.tmux.conf"
PEACLOCK_CONFIG="$HOME_DIR/.config/peaclock/config"

MODE="dark"
SCHEME_TYPE="scheme-tonal-spot"
WALLPAPER=""
MATUGEN_BIN="${MATUGEN_BIN:-}"
TEXT_COLOR_OVERRIDE=""
CURSOR_COLOR_OVERRIDE=""
STATUS_PALETTE="default"
ANSI_RED_OVERRIDE=""
ANSI_GREEN_OVERRIDE=""
ANSI_YELLOW_OVERRIDE=""
ANSI_BLUE_OVERRIDE=""
ANSI_MAGENTA_OVERRIDE=""
ANSI_CYAN_OVERRIDE=""

usage() {
  cat <<'EOF'
Usage: apply-material.sh [-m dark|light] [-t scheme-type] [-w wallpaper_path] [-b matugen_bin]
                         [--text-color '#rrggbb'] [--cursor-color '#rrggbb']
                         [--status-palette default|vibrant]
                         [--ansi-red '#rrggbb'] [--ansi-green '#rrggbb']
                         [--ansi-yellow '#rrggbb'] [--ansi-blue '#rrggbb']
                         [--ansi-magenta '#rrggbb'] [--ansi-cyan '#rrggbb']

Defaults:
  mode: dark
  type: scheme-tonal-spot
  wallpaper: ~/.termux/background/background.jpeg
             (fallback: newest file in ~/.termux/backgrounds)
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -m|--mode)
      MODE="${2:-}"
      shift 2
      ;;
    -t|--type)
      SCHEME_TYPE="${2:-}"
      shift 2
      ;;
    -w|--wallpaper)
      WALLPAPER="${2:-}"
      shift 2
      ;;
    -b|--matugen-bin)
      MATUGEN_BIN="${2:-}"
      shift 2
      ;;
    --text-color)
      TEXT_COLOR_OVERRIDE="${2:-}"
      shift 2
      ;;
    --cursor-color)
      CURSOR_COLOR_OVERRIDE="${2:-}"
      shift 2
      ;;
    --status-palette)
      STATUS_PALETTE="${2:-}"
      shift 2
      ;;
    --ansi-red)
      ANSI_RED_OVERRIDE="${2:-}"
      shift 2
      ;;
    --ansi-green)
      ANSI_GREEN_OVERRIDE="${2:-}"
      shift 2
      ;;
    --ansi-yellow)
      ANSI_YELLOW_OVERRIDE="${2:-}"
      shift 2
      ;;
    --ansi-blue)
      ANSI_BLUE_OVERRIDE="${2:-}"
      shift 2
      ;;
    --ansi-magenta)
      ANSI_MAGENTA_OVERRIDE="${2:-}"
      shift 2
      ;;
    --ansi-cyan)
      ANSI_CYAN_OVERRIDE="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

resolve_matugen_bin() {
  if [ -n "$MATUGEN_BIN" ] && [ -x "$MATUGEN_BIN" ]; then
    printf '%s\n' "$MATUGEN_BIN"
    return 0
  fi

  if [ -x "$HOME_DIR/cargo/bin/matugen" ]; then
    printf '%s\n' "$HOME_DIR/cargo/bin/matugen"
    return 0
  fi

  if [ -x "$HOME_DIR/.cargo/bin/matugen" ]; then
    printf '%s\n' "$HOME_DIR/.cargo/bin/matugen"
    return 0
  fi

  if command -v matugen >/dev/null 2>&1; then
    command -v matugen
    return 0
  fi

  return 1
}

pick_latest_wallpaper() {
  if [ -f "$WALLPAPER_FIXED" ]; then
    printf '%s\n' "$WALLPAPER_FIXED"
    return 0
  fi

  if [ ! -d "$WALLPAPER_DIR" ]; then
    return 1
  fi

  latest_file="$(ls -1t "$WALLPAPER_DIR" 2>/dev/null | head -n 1 || true)"
  if [ -z "$latest_file" ]; then
    return 1
  fi
  printf '%s/%s\n' "$WALLPAPER_DIR" "$latest_file"
}

MATUGEN_BIN="$(resolve_matugen_bin || true)"
if [ -z "$MATUGEN_BIN" ]; then
  echo "matugen binary not found. Set MATUGEN_BIN or install matugen." >&2
  exit 1
fi

case "$MODE" in
  dark|light) ;;
  *)
    echo "Invalid mode: $MODE (use dark or light)" >&2
    exit 1
    ;;
esac

case "$STATUS_PALETTE" in
  default|vibrant) ;;
  *)
    echo "Invalid status palette: $STATUS_PALETTE (use default or vibrant)" >&2
    exit 1
    ;;
esac

if [ -z "$WALLPAPER" ]; then
  WALLPAPER="$(pick_latest_wallpaper || true)"
fi

if [ -z "$WALLPAPER" ] || [ ! -f "$WALLPAPER" ]; then
  echo "Wallpaper not found. Expected $WALLPAPER_FIXED (or use -w)." >&2
  exit 1
fi

mkdir -p "$BACKUP_ROOT" "$HOME_DIR/.termux"
STAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="$BACKUP_ROOT/$STAMP"
mkdir -p "$BACKUP_DIR"

if [ -f "$TERMUX_COLORS" ]; then
  cp "$TERMUX_COLORS" "$BACKUP_DIR/colors.properties.bak"
fi
if [ -f "$TMUX_CONF" ]; then
  cp "$TMUX_CONF" "$BACKUP_DIR/tmux.conf.bak"
fi
if [ -f "$PEACLOCK_CONFIG" ]; then
  cp "$PEACLOCK_CONFIG" "$BACKUP_DIR/peaclock.config.bak"
fi

JSON_FILE="$BACKUP_DIR/matugen.json"
"$MATUGEN_BIN" image "$WALLPAPER" -m "$MODE" -t "$SCHEME_TYPE" --source-color-index 0 -j hex --dry-run > "$JSON_FILE"

jq_color() {
  role="$1"
  jq -r --arg role "$role" '.colors[$role].default.color' "$JSON_FILE"
}

BG="$(jq_color background)"
FG="$(jq_color on_background)"
CURSOR="$(jq_color primary)"
C0="$(jq_color surface_container)"
C1="$(jq_color error)"
C2="$(jq_color primary)"
C3="$(jq_color tertiary)"
C4="$(jq_color secondary)"
C5="$(jq_color primary_container)"
C6="$(jq_color tertiary_container)"
C7="$(jq_color on_surface)"
C8="$(jq_color surface_container_high)"
C9="$(jq_color error_container)"
C10="$(jq_color primary_container)"
C11="$(jq_color tertiary_container)"
C12="$(jq_color secondary_container)"
C13="$(jq_color inverse_primary)"
C14="$(jq_color outline)"
C15="$(jq_color on_surface_variant)"
C16="$(jq_color secondary_fixed)"
C17="$(jq_color tertiary_fixed)"
C18="$(jq_color surface_dim)"
C19="$(jq_color surface_bright)"
C20="$(jq_color surface_variant)"
C21="$(jq_color outline_variant)"
P_FIXED="$(jq_color primary_fixed)"
P_FIXED_DIM="$(jq_color primary_fixed_dim)"
S_FIXED="$(jq_color secondary_fixed)"
S_FIXED_DIM="$(jq_color secondary_fixed_dim)"
T_FIXED="$(jq_color tertiary_fixed)"
T_FIXED_DIM="$(jq_color tertiary_fixed_dim)"

is_hex_color() {
  case "$1" in
    \#[0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F]) return 0 ;;
    *) return 1 ;;
  esac
}

if [ -n "$TEXT_COLOR_OVERRIDE" ]; then
  if is_hex_color "$TEXT_COLOR_OVERRIDE"; then
    FG="$TEXT_COLOR_OVERRIDE"
  else
    echo "Invalid --text-color value: $TEXT_COLOR_OVERRIDE (expected #rrggbb)" >&2
    exit 1
  fi
fi

if [ -n "$CURSOR_COLOR_OVERRIDE" ]; then
  if is_hex_color "$CURSOR_COLOR_OVERRIDE"; then
    CURSOR="$CURSOR_COLOR_OVERRIDE"
  else
    echo "Invalid --cursor-color value: $CURSOR_COLOR_OVERRIDE (expected #rrggbb)" >&2
    exit 1
  fi
fi

validate_override_hex() {
  val="$1"
  name="$2"
  [ -z "$val" ] && return 0
  if ! is_hex_color "$val"; then
    echo "Invalid $name value: $val (expected #rrggbb)" >&2
    exit 1
  fi
}

validate_override_hex "$ANSI_RED_OVERRIDE" "--ansi-red"
validate_override_hex "$ANSI_GREEN_OVERRIDE" "--ansi-green"
validate_override_hex "$ANSI_YELLOW_OVERRIDE" "--ansi-yellow"
validate_override_hex "$ANSI_BLUE_OVERRIDE" "--ansi-blue"
validate_override_hex "$ANSI_MAGENTA_OVERRIDE" "--ansi-magenta"
validate_override_hex "$ANSI_CYAN_OVERRIDE" "--ansi-cyan"

[ -n "$ANSI_RED_OVERRIDE" ] && { C1="$ANSI_RED_OVERRIDE"; C9="$ANSI_RED_OVERRIDE"; }
[ -n "$ANSI_GREEN_OVERRIDE" ] && { C2="$ANSI_GREEN_OVERRIDE"; C10="$ANSI_GREEN_OVERRIDE"; }
[ -n "$ANSI_YELLOW_OVERRIDE" ] && { C3="$ANSI_YELLOW_OVERRIDE"; C11="$ANSI_YELLOW_OVERRIDE"; }
[ -n "$ANSI_BLUE_OVERRIDE" ] && { C4="$ANSI_BLUE_OVERRIDE"; C12="$ANSI_BLUE_OVERRIDE"; }
[ -n "$ANSI_MAGENTA_OVERRIDE" ] && { C5="$ANSI_MAGENTA_OVERRIDE"; C13="$ANSI_MAGENTA_OVERRIDE"; }
[ -n "$ANSI_CYAN_OVERRIDE" ] && { C6="$ANSI_CYAN_OVERRIDE"; C14="$ANSI_CYAN_OVERRIDE"; }

if [ "$STATUS_PALETTE" = "vibrant" ]; then
  SEP_STATUS="$P_FIXED"
  WEATHER_STATUS="$P_FIXED"
  CHARGING_STATUS="$T_FIXED"
  BAT_1="$C1"
  BAT_2="$C9"
  BAT_3="$T_FIXED_DIM"
  BAT_4="$S_FIXED_DIM"
  BAT_5="$S_FIXED"
  BAT_6="$P_FIXED"
  CPU_1="$P_FIXED"
  CPU_2="$S_FIXED"
  CPU_3="$T_FIXED"
  CPU_4="$T_FIXED_DIM"
  CPU_5="$C9"
  CPU_6="$C1"
  RAM_1="$S_FIXED"
  RAM_2="$S_FIXED_DIM"
  RAM_3="$P_FIXED"
  RAM_4="$T_FIXED"
  RAM_5="$C9"
  RAM_6="$C1"
else
  SEP_STATUS="$C14"
  WEATHER_STATUS="$C2"
  CHARGING_STATUS="$C2"
  BAT_1="$C1"
  BAT_2="$C3"
  BAT_3="$C11"
  BAT_4="$C4"
  BAT_5="$C12"
  BAT_6="$C2"
  CPU_1="$C2"
  CPU_2="$C5"
  CPU_3="$C3"
  CPU_4="$C11"
  CPU_5="$C9"
  CPU_6="$C1"
  RAM_1="$C6"
  RAM_2="$C12"
  RAM_3="$C13"
  RAM_4="$C3"
  RAM_5="$C9"
  RAM_6="$C1"
fi

TERMUX_TMP="$BACKUP_DIR/colors.properties.new"
cat > "$TERMUX_TMP" <<EOF
# Generated by $THEME_DIR/apply-material.sh
# source wallpaper: $WALLPAPER
# mode: $MODE
# type: $SCHEME_TYPE
foreground=$FG
background=$BG
cursor=$CURSOR

color0=$C0
color1=$C1
color2=$C2
color3=$C3
color4=$C4
color5=$C5
color6=$C6
color7=$C7

color8=$C8
color9=$C9
color10=$C10
color11=$C11
color12=$C12
color13=$C13
color14=$C14
color15=$C15

color16=$C16
color17=$C17
color18=$C18
color19=$C19
color20=$C20
color21=$C21
EOF
cp "$TERMUX_TMP" "$TERMUX_COLORS"

TMUX_BLOCK_FILE="$BACKUP_DIR/tmux-material-block.conf"
cat > "$TMUX_BLOCK_FILE" <<EOF
# >>> MATUGEN THEME START >>>
# Generated by $THEME_DIR/apply-material.sh
set -g status-style "bg=$BG,fg=$FG"
set -g status-left "#[fg=$FG,bg=$C4,bold] #S #[bg=$BG,fg=$FG] "
set -g status-right "#{?client_prefix,PREFIX ,}#(/data/data/com.termux/files/home/.config/tmux/widget-battery) | #(/data/data/com.termux/files/home/.config/tmux/widget-cpu) | #(/data/data/com.termux/files/home/.config/tmux/widget-ram) | #(/data/data/com.termux/files/home/.config/tmux/widget-weather) "
set -g window-status-format "#[fg=$C14] #I:#W "
set -g window-status-current-format "#[fg=$C2,bold] #I:#W "
set -g pane-border-style "fg=$C14"
set -g pane-active-border-style "fg=$C2"
set -g message-style "bg=$BG,fg=$C2"
set -g message-command-style "bg=$BG,fg=$C2"
set -g mode-style "bg=$C2,fg=$BG"
setw -g clock-mode-colour "$C2"
set -g @status-tmux-palette "$STATUS_PALETTE"
set -g @status-tmux-color-separator "$SEP_STATUS"
set -g @status-tmux-color-weather "$WEATHER_STATUS"
set -g @status-tmux-color-charging "$CHARGING_STATUS"
set -g @status-tmux-color-battery-1 "$BAT_1"
set -g @status-tmux-color-battery-2 "$BAT_2"
set -g @status-tmux-color-battery-3 "$BAT_3"
set -g @status-tmux-color-battery-4 "$BAT_4"
set -g @status-tmux-color-battery-5 "$BAT_5"
set -g @status-tmux-color-battery-6 "$BAT_6"
set -g @status-tmux-color-cpu-1 "$CPU_1"
set -g @status-tmux-color-cpu-2 "$CPU_2"
set -g @status-tmux-color-cpu-3 "$CPU_3"
set -g @status-tmux-color-cpu-4 "$CPU_4"
set -g @status-tmux-color-cpu-5 "$CPU_5"
set -g @status-tmux-color-cpu-6 "$CPU_6"
set -g @status-tmux-color-ram-1 "$RAM_1"
set -g @status-tmux-color-ram-2 "$RAM_2"
set -g @status-tmux-color-ram-3 "$RAM_3"
set -g @status-tmux-color-ram-4 "$RAM_4"
set -g @status-tmux-color-ram-5 "$RAM_5"
set -g @status-tmux-color-ram-6 "$RAM_6"
# <<< MATUGEN THEME END <<<
EOF

if [ ! -f "$TMUX_CONF" ]; then
  touch "$TMUX_CONF"
fi

TMUX_TMP="$BACKUP_DIR/tmux.conf.new"
awk '
  BEGIN { skip=0 }
  /^# >>> MATUGEN THEME START >>>/ { skip=1; next }
  /^# <<< MATUGEN THEME END <<</ { skip=0; next }
  skip==0 { print }
' "$TMUX_CONF" > "$TMUX_TMP"
printf "\n" >> "$TMUX_TMP"
cat "$TMUX_BLOCK_FILE" >> "$TMUX_TMP"
cp "$TMUX_TMP" "$TMUX_CONF"

if [ ! -f "$PEACLOCK_CONFIG" ]; then
  mkdir -p "$HOME_DIR/.config/peaclock"
  touch "$PEACLOCK_CONFIG"
fi

PEACLOCK_TMP="$BACKUP_DIR/peaclock.config.new"
awk '
  BEGIN { skip=0 }
  /^# >>> MATUGEN PEACLOCK START >>>/ { skip=1; next }
  /^# <<< MATUGEN PEACLOCK END <<</ { skip=0; next }
  skip==0 { print }
' "$PEACLOCK_CONFIG" > "$PEACLOCK_TMP"

cat >> "$PEACLOCK_TMP" <<EOF

# >>> MATUGEN PEACLOCK START >>>
# Generated by $THEME_DIR/apply-material.sh
style inactive-fg $C14
style active-bg $C2
style active-fg clear
style colon-fg $C2
style colon-bg clear
style date $C3
style text $C15
style prompt $C2
style success $C10
style error $C1
# <<< MATUGEN PEACLOCK END <<<
EOF
cp "$PEACLOCK_TMP" "$PEACLOCK_CONFIG"

{
  echo "backup_id=$STAMP"
  echo "wallpaper=$WALLPAPER"
  echo "mode=$MODE"
  echo "type=$SCHEME_TYPE"
  echo "matugen_bin=$MATUGEN_BIN"
  echo "text_color_override=$TEXT_COLOR_OVERRIDE"
  echo "cursor_color_override=$CURSOR_COLOR_OVERRIDE"
  echo "status_palette=$STATUS_PALETTE"
  echo "ansi_red_override=$ANSI_RED_OVERRIDE"
  echo "ansi_green_override=$ANSI_GREEN_OVERRIDE"
  echo "ansi_yellow_override=$ANSI_YELLOW_OVERRIDE"
  echo "ansi_blue_override=$ANSI_BLUE_OVERRIDE"
  echo "ansi_magenta_override=$ANSI_MAGENTA_OVERRIDE"
  echo "ansi_cyan_override=$ANSI_CYAN_OVERRIDE"
  echo "peaclock_themed=true"
} > "$BACKUP_DIR/meta.env"

if command -v termux-reload-settings >/dev/null 2>&1; then
  termux-reload-settings || true
fi
if command -v tmux >/dev/null 2>&1; then
  tmux source-file "$TMUX_CONF" 2>/dev/null || true
fi

echo "Applied Material theme."
echo "Backup created: $BACKUP_DIR"
