#!/data/data/com.termux/files/usr/bin/sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_DIR="$HOME_DIR/.config/tooie"
BACKUP_ROOT="$TOOIE_DIR/backups"
WALLPAPER_FIXED="$HOME_DIR/.termux/background/background.jpeg"
WALLPAPER_DIR="$HOME_DIR/.termux/backgrounds"
TERMUX_COLORS="$HOME_DIR/.termux/colors.properties"
TMUX_CONF="$HOME_DIR/.tmux.conf"
PEACLOCK_CONFIG="$HOME_DIR/.config/peaclock/config"
STARSHIP_CONFIG="$HOME_DIR/.config/starship.toml"
NVIM_THEME_FILE="$HOME_DIR/.config/nvim/lua/plugins/tooie-material.lua"

MODE="dark"
SCHEME_TYPE="scheme-tonal-spot"
WALLPAPER=""
MATUGEN_BIN="${MATUGEN_BIN:-}"
TEXT_COLOR_OVERRIDE=""
CURSOR_COLOR_OVERRIDE=""
STATUS_PALETTE="default"
STYLE_PRESET="balanced"
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
                         [--style-preset balanced|tokyonight|catppuccin|gruvbox|rose-pine|pure-matugen]
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
    --style-preset)
      STYLE_PRESET="${2:-}"
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

case "$STYLE_PRESET" in
  balanced|tokyonight|catppuccin|gruvbox|rose-pine|pure-matugen) ;;
  *)
    echo "Invalid style preset: $STYLE_PRESET" >&2
    echo "Use one of: balanced, tokyonight, catppuccin, gruvbox, rose-pine, pure-matugen" >&2
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
if [ -f "$STARSHIP_CONFIG" ]; then
  cp "$STARSHIP_CONFIG" "$BACKUP_DIR/starship.toml.bak"
fi
if [ -f "$NVIM_THEME_FILE" ]; then
  cp "$NVIM_THEME_FILE" "$BACKUP_DIR/tooie-material.lua.bak"
fi

JSON_FILE="$BACKUP_DIR/matugen.json"
"$MATUGEN_BIN" image "$WALLPAPER" -m "$MODE" -t "$SCHEME_TYPE" --source-color-index 0 -j hex --dry-run > "$JSON_FILE"

jq_color() {
  role="$1"
  jq -r --arg role "$role" '.colors[$role].default.color' "$JSON_FILE"
}

is_hex_color() {
  case "$1" in
    \#[0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F]) return 0 ;;
    *) return 1 ;;
  esac
}

normalize_hex() {
  c="$(printf '%s' "$1" | tr 'A-F' 'a-f')"
  if is_hex_color "$c"; then
    printf '%s\n' "$c"
  else
    printf '%s\n' "#000000"
  fi
}

hex_to_rgb() {
  h="$(normalize_hex "$1")"
  r_hex="$(printf '%s' "$h" | cut -c2-3)"
  g_hex="$(printf '%s' "$h" | cut -c4-5)"
  b_hex="$(printf '%s' "$h" | cut -c6-7)"
  r="$(printf '%d' "0x$r_hex")"
  g="$(printf '%d' "0x$g_hex")"
  b="$(printf '%d' "0x$b_hex")"
  printf '%s %s %s\n' "$r" "$g" "$b"
}

mix_hex() {
  ca="$(normalize_hex "$1")"
  cb="$(normalize_hex "$2")"
  t="$3"
  set -- $(hex_to_rgb "$ca")
  ar="$1"; ag="$2"; ab="$3"
  set -- $(hex_to_rgb "$cb")
  br="$1"; bg="$2"; bb="$3"
  awk -v ar="$ar" -v ag="$ag" -v ab="$ab" -v br="$br" -v bg="$bg" -v bb="$bb" -v t="$t" '
    function clamp(x) { if (x < 0) return 0; if (x > 255) return 255; return int(x + 0.5) }
    BEGIN {
      if (t < 0) t = 0
      if (t > 1) t = 1
      r = ar + (br - ar) * t
      g = ag + (bg - ag) * t
      b = ab + (bb - ab) * t
      printf "#%02x%02x%02x\n", clamp(r), clamp(g), clamp(b)
    }
  '
}

luminance() {
  set -- $(hex_to_rgb "$1")
  r="$1"; g="$2"; b="$3"
  awk -v r="$r" -v g="$g" -v b="$b" '
    function lin(c) {
      c = c / 255.0
      if (c <= 0.04045) return c / 12.92
      return ((c + 0.055) / 1.055) ^ 2.4
    }
    BEGIN {
      rl = lin(r); gl = lin(g); bl = lin(b)
      printf "%.6f\n", 0.2126 * rl + 0.7152 * gl + 0.0722 * bl
    }
  '
}

contrast_ratio() {
  la="$(luminance "$1")"
  lb="$(luminance "$2")"
  awk -v a="$la" -v b="$lb" '
    BEGIN {
      if (a < b) { t = a; a = b; b = t }
      printf "%.4f\n", (a + 0.05) / (b + 0.05)
    }
  '
}

ratio_meets() {
  awk -v a="$1" -v b="$2" 'BEGIN { exit !(a + 0 >= b + 0) }'
}

best_contrast_color() {
  bg="$1"
  c1="$(normalize_hex "$2")"
  c2="$(normalize_hex "$3")"
  r1="$(contrast_ratio "$c1" "$bg")"
  r2="$(contrast_ratio "$c2" "$bg")"
  if awk -v a="$r1" -v b="$r2" 'BEGIN { exit !(a >= b) }'; then
    printf '%s\n' "$c1"
  else
    printf '%s\n' "$c2"
  fi
}

ensure_contrast() {
  fg="$(normalize_hex "$1")"
  bg="$(normalize_hex "$2")"
  target="$3"
  current="$(contrast_ratio "$fg" "$bg")"
  if ratio_meets "$current" "$target"; then
    printf '%s\n' "$fg"
    return 0
  fi

  anchor="$(best_contrast_color "$bg" "#f2f2f8" "#111118")"
  for t in 0.15 0.30 0.45 0.60 0.75 0.90 1.00; do
    cand="$(mix_hex "$fg" "$anchor" "$t")"
    cand_ratio="$(contrast_ratio "$cand" "$bg")"
    if ratio_meets "$cand_ratio" "$target"; then
      printf '%s\n' "$cand"
      return 0
    fi
  done

  printf '%s\n' "$anchor"
}

role_color() {
  role="$1"
  fallback="$2"
  val="$(jq_color "$role")"
  if is_hex_color "$val"; then
    normalize_hex "$val"
  else
    normalize_hex "$fallback"
  fi
}

if [ "$STYLE_PRESET" = "pure-matugen" ]; then
  if [ "$MODE" = "dark" ]; then
    FALLBACK_BG="#1a1b26"
    FALLBACK_FG="#c0caf5"
    BRIGHT_MIX="#ffffff"
    BRIGHT_T="0.26"
  else
    FALLBACK_BG="#eff1f5"
    FALLBACK_FG="#4c4f69"
    BRIGHT_MIX="#000000"
    BRIGHT_T="0.22"
  fi
  PURE_PRIMARY="$(role_color primary "#7aa2f7")"
  PURE_SECONDARY="$(role_color secondary "#7dcfff")"
  PURE_TERTIARY="$(role_color tertiary "#bb9af7")"
  PURE_ERROR="$(role_color error "#ff5f5f")"
  PURE_ON_SURFACE="$(role_color on_surface "$FALLBACK_FG")"
  PURE_ON_SURFACE_VAR="$(role_color on_surface_variant "$(mix_hex "$FALLBACK_FG" "$FALLBACK_BG" 0.35)")"

  BG="$(role_color background "$FALLBACK_BG")"
  FG="$(ensure_contrast "$(role_color on_background "$FALLBACK_FG")" "$BG" 7.0)"
  CURSOR="$(ensure_contrast "$PURE_PRIMARY" "$BG" 4.5)"
  C0="$(role_color surface_container "$(mix_hex "$BG" "#000000" 0.15)")"
  C1="$(ensure_contrast "$PURE_ERROR" "$BG" 3.2)"
  C2="$(ensure_contrast "$(mix_hex "$PURE_PRIMARY" "$PURE_SECONDARY" 0.62)" "$BG" 3.2)"
  C3="$(ensure_contrast "$(mix_hex "$PURE_ERROR" "$PURE_TERTIARY" 0.54)" "$BG" 3.2)"
  C4="$(ensure_contrast "$PURE_PRIMARY" "$BG" 3.2)"
  C5="$(ensure_contrast "$PURE_TERTIARY" "$BG" 3.2)"
  C6="$(ensure_contrast "$PURE_SECONDARY" "$BG" 3.2)"
  C7="$(ensure_contrast "$PURE_ON_SURFACE" "$BG" 6.0)"
  C8="$(role_color surface_container_high "$(mix_hex "$C0" "#ffffff" 0.12)")"
  C9="$(ensure_contrast "$(mix_hex "$C1" "$BRIGHT_MIX" "$BRIGHT_T")" "$BG" 4.0)"
  C10="$(ensure_contrast "$(mix_hex "$C2" "$BRIGHT_MIX" "$BRIGHT_T")" "$BG" 4.0)"
  C11="$(ensure_contrast "$(mix_hex "$C3" "$BRIGHT_MIX" "$BRIGHT_T")" "$BG" 4.0)"
  C12="$(ensure_contrast "$(mix_hex "$C4" "$BRIGHT_MIX" "$BRIGHT_T")" "$BG" 4.0)"
  C13="$(ensure_contrast "$(mix_hex "$C5" "$BRIGHT_MIX" "$BRIGHT_T")" "$BG" 4.0)"
  C14="$(ensure_contrast "$(role_color outline "$(mix_hex "$FG" "$BG" 0.54)")" "$BG" 3.2)"
  C15="$(ensure_contrast "$PURE_ON_SURFACE_VAR" "$BG" 4.5)"
  C16="$(ensure_contrast "$(role_color secondary_fixed "$(mix_hex "$C2" "$C6" 0.5)")" "$BG" 3.2)"
  C17="$(ensure_contrast "$(role_color tertiary_fixed "$(mix_hex "$C5" "$C4" 0.4)")" "$BG" 3.2)"
  C18="$(role_color surface_dim "$(mix_hex "$BG" "#000000" 0.08)")"
  C19="$(role_color surface_bright "$(mix_hex "$BG" "#ffffff" 0.08)")"
  C20="$(role_color surface_variant "$(mix_hex "$C0" "$C15" 0.25)")"
  C21="$(role_color outline_variant "$(mix_hex "$C14" "$C0" 0.30)")"
else
  if [ "$MODE" = "dark" ]; then
    case "$STYLE_PRESET" in
      tokyonight)
        ANCHOR_BG="#1a1b26"; ANCHOR_FG="#c0caf5"; ANCHOR_RED="#f7768e"; ANCHOR_GREEN="#9ece6a"; ANCHOR_YELLOW="#e0af68"; ANCHOR_BLUE="#7aa2f7"; ANCHOR_MAGENTA="#bb9af7"; ANCHOR_CYAN="#7dcfff"
        ;;
      catppuccin)
        ANCHOR_BG="#1e1e2e"; ANCHOR_FG="#cdd6f4"; ANCHOR_RED="#f38ba8"; ANCHOR_GREEN="#a6e3a1"; ANCHOR_YELLOW="#f9e2af"; ANCHOR_BLUE="#89b4fa"; ANCHOR_MAGENTA="#cba6f7"; ANCHOR_CYAN="#94e2d5"
        ;;
      gruvbox)
        ANCHOR_BG="#282828"; ANCHOR_FG="#ebdbb2"; ANCHOR_RED="#fb4934"; ANCHOR_GREEN="#b8bb26"; ANCHOR_YELLOW="#fabd2f"; ANCHOR_BLUE="#83a598"; ANCHOR_MAGENTA="#d3869b"; ANCHOR_CYAN="#8ec07c"
        ;;
      rose-pine)
        ANCHOR_BG="#191724"; ANCHOR_FG="#e0def4"; ANCHOR_RED="#eb6f92"; ANCHOR_GREEN="#9ccfd8"; ANCHOR_YELLOW="#f6c177"; ANCHOR_BLUE="#31748f"; ANCHOR_MAGENTA="#c4a7e7"; ANCHOR_CYAN="#9ccfd8"
        ;;
      *)
        ANCHOR_BG="#1a1b26"; ANCHOR_FG="#c0caf5"; ANCHOR_RED="#f7768e"; ANCHOR_GREEN="#9ece6a"; ANCHOR_YELLOW="#e0af68"; ANCHOR_BLUE="#7aa2f7"; ANCHOR_MAGENTA="#bb9af7"; ANCHOR_CYAN="#7dcfff"
        ;;
    esac
  else
    case "$STYLE_PRESET" in
      tokyonight)
        ANCHOR_BG="#d5d6db"; ANCHOR_FG="#343b58"; ANCHOR_RED="#8c4351"; ANCHOR_GREEN="#485e30"; ANCHOR_YELLOW="#8f5e15"; ANCHOR_BLUE="#34548a"; ANCHOR_MAGENTA="#5a4a78"; ANCHOR_CYAN="#0f4b6e"
        ;;
      catppuccin)
        ANCHOR_BG="#eff1f5"; ANCHOR_FG="#4c4f69"; ANCHOR_RED="#d20f39"; ANCHOR_GREEN="#40a02b"; ANCHOR_YELLOW="#df8e1d"; ANCHOR_BLUE="#1e66f5"; ANCHOR_MAGENTA="#8839ef"; ANCHOR_CYAN="#179299"
        ;;
      gruvbox)
        ANCHOR_BG="#fbf1c7"; ANCHOR_FG="#3c3836"; ANCHOR_RED="#cc241d"; ANCHOR_GREEN="#98971a"; ANCHOR_YELLOW="#d79921"; ANCHOR_BLUE="#458588"; ANCHOR_MAGENTA="#b16286"; ANCHOR_CYAN="#689d6a"
        ;;
      rose-pine)
        ANCHOR_BG="#faf4ed"; ANCHOR_FG="#575279"; ANCHOR_RED="#b4637a"; ANCHOR_GREEN="#56949f"; ANCHOR_YELLOW="#ea9d34"; ANCHOR_BLUE="#286983"; ANCHOR_MAGENTA="#907aa9"; ANCHOR_CYAN="#56949f"
        ;;
      *)
        ANCHOR_BG="#eff1f5"; ANCHOR_FG="#4c4f69"; ANCHOR_RED="#d20f39"; ANCHOR_GREEN="#40a02b"; ANCHOR_YELLOW="#df8e1d"; ANCHOR_BLUE="#1e66f5"; ANCHOR_MAGENTA="#8839ef"; ANCHOR_CYAN="#179299"
        ;;
    esac
  fi

  BG_BASE="$(role_color background "$ANCHOR_BG")"
  SURFACE_BASE="$(role_color surface_container "$(mix_hex "$BG_BASE" "$ANCHOR_BG" 0.22)")"
  FG_BASE="$(role_color on_background "$ANCHOR_FG")"
  MUTED_BASE="$(role_color on_surface_variant "$(mix_hex "$FG_BASE" "$BG_BASE" 0.36)")"
  CURSOR_BASE="$(role_color primary "$ANCHOR_BLUE")"
  PRIMARY_BASE="$(role_color primary "$ANCHOR_BLUE")"
  SECONDARY_BASE="$(role_color secondary "$ANCHOR_CYAN")"
  TERTIARY_BASE="$(role_color tertiary "$ANCHOR_MAGENTA")"
  ERROR_BASE="$(role_color error "$ANCHOR_RED")"
  OUTLINE_BASE="$(role_color outline "$(mix_hex "$FG_BASE" "$BG_BASE" 0.54)")"

  BG="$BG_BASE"
  C0="$SURFACE_BASE"
  FG="$(ensure_contrast "$FG_BASE" "$BG" 7.0)"
  CURSOR="$(ensure_contrast "$CURSOR_BASE" "$BG" 4.5)"
  C7="$(ensure_contrast "$(role_color on_surface "$FG")" "$BG" 6.0)"
  C15="$(ensure_contrast "$MUTED_BASE" "$BG" 4.5)"
  C14="$(ensure_contrast "$OUTLINE_BASE" "$BG" 3.2)"

  C1="$(ensure_contrast "$(mix_hex "$ERROR_BASE" "$ANCHOR_RED" 0.42)" "$BG" 3.2)"
  C2="$(ensure_contrast "$(mix_hex "$TERTIARY_BASE" "$ANCHOR_GREEN" 0.60)" "$BG" 3.2)"
  C3="$(ensure_contrast "$(mix_hex "$SECONDARY_BASE" "$ANCHOR_YELLOW" 0.55)" "$BG" 3.2)"
  C4="$(ensure_contrast "$(mix_hex "$PRIMARY_BASE" "$ANCHOR_BLUE" 0.52)" "$BG" 3.2)"
  C5="$(ensure_contrast "$(mix_hex "$TERTIARY_BASE" "$ANCHOR_MAGENTA" 0.48)" "$BG" 3.2)"
  C6="$(ensure_contrast "$(mix_hex "$SECONDARY_BASE" "$ANCHOR_CYAN" 0.52)" "$BG" 3.2)"

  if [ "$MODE" = "dark" ]; then
    C8="$(ensure_contrast "$(mix_hex "$C0" "#ffffff" 0.20)" "$BG" 1.3)"
    C9="$(ensure_contrast "$(mix_hex "$C1" "#ffffff" 0.24)" "$BG" 4.0)"
    C10="$(ensure_contrast "$(mix_hex "$C2" "#ffffff" 0.24)" "$BG" 4.0)"
    C11="$(ensure_contrast "$(mix_hex "$C3" "#ffffff" 0.24)" "$BG" 4.0)"
    C12="$(ensure_contrast "$(mix_hex "$C4" "#ffffff" 0.24)" "$BG" 4.0)"
    C13="$(ensure_contrast "$(mix_hex "$C5" "#ffffff" 0.24)" "$BG" 4.0)"
    C18="$(ensure_contrast "$(mix_hex "$BG" "#000000" 0.10)" "$BG" 1.05)"
    C19="$(ensure_contrast "$(mix_hex "$BG" "#ffffff" 0.16)" "$BG" 1.25)"
  else
    C8="$(ensure_contrast "$(mix_hex "$C0" "#000000" 0.14)" "$BG" 1.3)"
    C9="$(ensure_contrast "$(mix_hex "$C1" "#000000" 0.22)" "$BG" 4.0)"
    C10="$(ensure_contrast "$(mix_hex "$C2" "#000000" 0.22)" "$BG" 4.0)"
    C11="$(ensure_contrast "$(mix_hex "$C3" "#000000" 0.22)" "$BG" 4.0)"
    C12="$(ensure_contrast "$(mix_hex "$C4" "#000000" 0.22)" "$BG" 4.0)"
    C13="$(ensure_contrast "$(mix_hex "$C5" "#000000" 0.22)" "$BG" 4.0)"
    C18="$(ensure_contrast "$(mix_hex "$BG" "#000000" 0.08)" "$BG" 1.05)"
    C19="$(ensure_contrast "$(mix_hex "$BG" "#ffffff" 0.06)" "$BG" 1.15)"
  fi

  C16="$(ensure_contrast "$(mix_hex "$C2" "$C6" 0.36)" "$BG" 3.2)"
  C17="$(ensure_contrast "$(mix_hex "$C5" "$C4" 0.36)" "$BG" 3.2)"
  C20="$(ensure_contrast "$(mix_hex "$C0" "$C15" 0.30)" "$BG" 1.5)"
  C21="$(ensure_contrast "$(mix_hex "$C14" "$C0" 0.30)" "$BG" 1.4)"
fi

STATUS_LEFT_FG="$(ensure_contrast "$FG" "$C4" 4.5)"
MODE_FG="$(ensure_contrast "$BG" "$C2" 4.5)"

if [ -n "$TEXT_COLOR_OVERRIDE" ]; then
  if is_hex_color "$TEXT_COLOR_OVERRIDE"; then
    FG="$(ensure_contrast "$TEXT_COLOR_OVERRIDE" "$BG" 7.0)"
  else
    echo "Invalid --text-color value: $TEXT_COLOR_OVERRIDE (expected #rrggbb)" >&2
    exit 1
  fi
fi

if [ -n "$CURSOR_COLOR_OVERRIDE" ]; then
  if is_hex_color "$CURSOR_COLOR_OVERRIDE"; then
    CURSOR="$(ensure_contrast "$CURSOR_COLOR_OVERRIDE" "$BG" 4.5)"
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
  SEP_STATUS="$(ensure_contrast "$C12" "$BG" 3.0)"
  WEATHER_STATUS="$(ensure_contrast "$C6" "$BG" 3.0)"
  CHARGING_STATUS="$(ensure_contrast "$C10" "$BG" 3.0)"
  BAT_1="$(ensure_contrast "$C1" "$BG" 3.0)"
  BAT_2="$(ensure_contrast "$(mix_hex "$C1" "$C3" 0.45)" "$BG" 3.0)"
  BAT_3="$(ensure_contrast "$C3" "$BG" 3.0)"
  BAT_4="$(ensure_contrast "$(mix_hex "$C3" "$C2" 0.45)" "$BG" 3.0)"
  BAT_5="$(ensure_contrast "$(mix_hex "$C2" "$C10" 0.35)" "$BG" 3.0)"
  BAT_6="$(ensure_contrast "$C10" "$BG" 3.0)"
  CPU_1="$(ensure_contrast "$(mix_hex "$C1" "$C9" 0.25)" "$BG" 3.0)"
  CPU_2="$(ensure_contrast "$C9" "$BG" 3.0)"
  CPU_3="$(ensure_contrast "$(mix_hex "$C11" "$C3" 0.45)" "$BG" 3.0)"
  CPU_4="$(ensure_contrast "$C11" "$BG" 3.0)"
  CPU_5="$(ensure_contrast "$(mix_hex "$C10" "$C2" 0.35)" "$BG" 3.0)"
  CPU_6="$(ensure_contrast "$C10" "$BG" 3.0)"
  RAM_1="$(ensure_contrast "$(mix_hex "$C1" "$C5" 0.30)" "$BG" 3.0)"
  RAM_2="$(ensure_contrast "$(mix_hex "$C1" "$C6" 0.40)" "$BG" 3.0)"
  RAM_3="$(ensure_contrast "$(mix_hex "$C3" "$C6" 0.42)" "$BG" 3.0)"
  RAM_4="$(ensure_contrast "$(mix_hex "$C2" "$C6" 0.35)" "$BG" 3.0)"
  RAM_5="$(ensure_contrast "$C2" "$BG" 3.0)"
  RAM_6="$(ensure_contrast "$C10" "$BG" 3.0)"
else
  SEP_STATUS="$(ensure_contrast "$C14" "$BG" 3.0)"
  WEATHER_STATUS="$(ensure_contrast "$C4" "$BG" 3.0)"
  CHARGING_STATUS="$(ensure_contrast "$C2" "$BG" 3.0)"
  BAT_1="$(ensure_contrast "$C1" "$BG" 3.0)"
  BAT_2="$(ensure_contrast "$(mix_hex "$C1" "$C3" 0.35)" "$BG" 3.0)"
  BAT_3="$(ensure_contrast "$C3" "$BG" 3.0)"
  BAT_4="$(ensure_contrast "$(mix_hex "$C3" "$C2" 0.35)" "$BG" 3.0)"
  BAT_5="$(ensure_contrast "$C2" "$BG" 3.0)"
  BAT_6="$(ensure_contrast "$C10" "$BG" 3.0)"
  CPU_1="$(ensure_contrast "$C1" "$BG" 3.0)"
  CPU_2="$(ensure_contrast "$(mix_hex "$C1" "$C11" 0.42)" "$BG" 3.0)"
  CPU_3="$(ensure_contrast "$C11" "$BG" 3.0)"
  CPU_4="$(ensure_contrast "$(mix_hex "$C11" "$C2" 0.48)" "$BG" 3.0)"
  CPU_5="$(ensure_contrast "$C2" "$BG" 3.0)"
  CPU_6="$(ensure_contrast "$C10" "$BG" 3.0)"
  RAM_1="$(ensure_contrast "$(mix_hex "$C1" "$C6" 0.28)" "$BG" 3.0)"
  RAM_2="$(ensure_contrast "$(mix_hex "$C3" "$C6" 0.32)" "$BG" 3.0)"
  RAM_3="$(ensure_contrast "$C6" "$BG" 3.0)"
  RAM_4="$(ensure_contrast "$(mix_hex "$C6" "$C2" 0.40)" "$BG" 3.0)"
  RAM_5="$(ensure_contrast "$C2" "$BG" 3.0)"
  RAM_6="$(ensure_contrast "$C10" "$BG" 3.0)"
fi

TERMUX_TMP="$BACKUP_DIR/colors.properties.new"
cat > "$TERMUX_TMP" <<EOF
# Generated by $TOOIE_DIR/apply-material.sh
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
# Generated by $TOOIE_DIR/apply-material.sh
set -g status-style "bg=$BG,fg=$FG"
set -g status-left "#[fg=$STATUS_LEFT_FG,bg=$C4,bold] #S #[bg=$BG,fg=$FG] "
set -g status-right "#{?client_prefix,PREFIX ,}#(/data/data/com.termux/files/home/.config/tmux/widget-battery) | #(/data/data/com.termux/files/home/.config/tmux/widget-cpu) | #(/data/data/com.termux/files/home/.config/tmux/widget-ram) | #(/data/data/com.termux/files/home/.config/tmux/widget-weather) "
set -g window-status-format "#[fg=$C14] #I:#W "
set -g window-status-current-format "#[fg=$C2,bold] #I:#W "
set -g pane-border-style "fg=$C14"
set -g pane-active-border-style "fg=$C2"
set -g message-style "bg=$BG,fg=$C2"
set -g message-command-style "bg=$BG,fg=$C2"
set -g mode-style "bg=$C2,fg=$MODE_FG"
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
# Generated by $TOOIE_DIR/apply-material.sh
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

if [ ! -f "$STARSHIP_CONFIG" ]; then
  mkdir -p "$HOME_DIR/.config"
  touch "$STARSHIP_CONFIG"
fi

STARSHIP_TMP="$BACKUP_DIR/starship.toml.new"
awk '
  BEGIN { skip=0 }
  /^# >>> MATUGEN STARSHIP START >>>/ { skip=1; next }
  /^# <<< MATUGEN STARSHIP END <<</ { skip=0; next }
  skip==0 { print }
' "$STARSHIP_CONFIG" > "$STARSHIP_TMP"
cp "$STARSHIP_TMP" "$STARSHIP_CONFIG"

toml_upsert() {
  _file="$1"
  _section="$2"
  _key="$3"
  _value="$4"
  _tmp="${_file}.tmp.$$"
  awk -v sec="$_section" -v key="$_key" -v val="$_value" '
    BEGIN { insec=0; sec_found=0; done=0 }
    $0 ~ "^[[:space:]]*\\[" sec "\\][[:space:]]*$" {
      sec_found=1; insec=1; print; next
    }
    $0 ~ "^[[:space:]]*\\[[^]]+\\][[:space:]]*$" {
      if (insec && !done) { print key " = " val; done=1 }
      insec=0
    }
    {
      if (insec && $0 ~ "^[[:space:]]*" key "[[:space:]]*=") {
        if (!done) { print key " = " val; done=1 }
        next
      }
      print
    }
    END {
      if (insec && !done) { print key " = " val; done=1 }
      if (!sec_found) {
        if (NR > 0) print ""
        print "[" sec "]"
        print key " = " val
      }
    }
  ' "$_file" > "$_tmp"
  mv "$_tmp" "$_file"
}

toml_upsert "$STARSHIP_CONFIG" "character" "success_symbol" "\"[◎](bold $C3)\""
toml_upsert "$STARSHIP_CONFIG" "character" "error_symbol" "\"[○](bold $C1)\""
toml_upsert "$STARSHIP_CONFIG" "character" "vimcmd_symbol" "\"[■](bold $C2)\""
toml_upsert "$STARSHIP_CONFIG" "directory" "style" "\"italic $C4\""
toml_upsert "$STARSHIP_CONFIG" "directory" "repo_root_style" "\"bold $C2\""
toml_upsert "$STARSHIP_CONFIG" "cmd_duration" "format" "\"[◄ \$duration ](italic $C15)\""
toml_upsert "$STARSHIP_CONFIG" "git_branch" "symbol" "\"[△](bold italic $C4)\""
toml_upsert "$STARSHIP_CONFIG" "git_branch" "style" "\"italic $C4\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "style" "\"bold italic $C2\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "conflicted" "\"[◪◦](italic $C5)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "ahead" "\"[▴│[\${count}](bold $C7)│](italic $C2)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "behind" "\"[▿│[\${count}](bold $C7)│](italic $C1)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "diverged" "\"[◇ ▴┤[\${ahead_count}](regular $C7)│▿┤[\${behind_count}](regular $C7)│](italic $C5)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "untracked" "\"[◌◦](italic $C3)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "stashed" "\"[◃◈](italic $C15)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "modified" "\"[●◦](italic $C3)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "staged" "\"[▪┤[\$count](bold $C7)│](italic $C6)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "renamed" "\"[◎◦](italic $C4)\""
toml_upsert "$STARSHIP_CONFIG" "git_status" "deleted" "\"[✕](italic $C1)\""
toml_upsert "$STARSHIP_CONFIG" "time" "style" "\"italic $C14\""
toml_upsert "$STARSHIP_CONFIG" "username" "style_user" "\"$C3 bold italic\""
toml_upsert "$STARSHIP_CONFIG" "username" "style_root" "\"$C1 bold italic\""
toml_upsert "$STARSHIP_CONFIG" "sudo" "style" "\"bold italic $C5\""
toml_upsert "$STARSHIP_CONFIG" "jobs" "style" "\"$C15\""
toml_upsert "$STARSHIP_CONFIG" "jobs" "symbol" "\"[▶]($C4 italic)\""

mkdir -p "$(dirname "$NVIM_THEME_FILE")"
cat > "$NVIM_THEME_FILE" <<EOF
-- Generated by $TOOIE_DIR/apply-material.sh
return {
  {
    "folke/tokyonight.nvim",
    opts = function(_, opts)
      opts = opts or {}
      opts.style = "night"
      opts.on_colors = function(c)
        c.bg = "$BG"
        c.bg_dark = "$C0"
        c.bg_statusline = "$C0"
        c.fg = "$FG"
        c.fg_dark = "$C15"
        c.border = "$C14"
        c.comment = "$C14"
        c.error = "$C1"
        c.warning = "$C3"
        c.info = "$C4"
        c.hint = "$C6"
      end
      opts.on_highlights = function(hl, c)
        hl.Normal = { fg = c.fg, bg = c.bg }
        hl.NormalFloat = { fg = c.fg, bg = "$C0" }
        hl.FloatBorder = { fg = "$C14", bg = "$C0" }
        hl.CursorLine = { bg = "$C18" }
        hl.Visual = { bg = "$C8" }
        hl.Search = { fg = "$BG", bg = "$C3", bold = true }
        hl.IncSearch = { fg = "$BG", bg = "$C2", bold = true }
        hl.StatusLine = { fg = "$FG", bg = "$C0" }
        hl.StatusLineNC = { fg = "$C15", bg = "$C0" }
      end
    end,
  },
  {
    "LazyVim/LazyVim",
    opts = {
      colorscheme = "tokyonight",
    },
  },
}
EOF

{
  echo "backup_id=$STAMP"
  echo "wallpaper=$WALLPAPER"
  echo "mode=$MODE"
  echo "type=$SCHEME_TYPE"
  echo "matugen_bin=$MATUGEN_BIN"
  echo "text_color_override=$TEXT_COLOR_OVERRIDE"
  echo "cursor_color_override=$CURSOR_COLOR_OVERRIDE"
  echo "status_palette=$STATUS_PALETTE"
  echo "style_preset=$STYLE_PRESET"
  echo "ansi_red_override=$ANSI_RED_OVERRIDE"
  echo "ansi_green_override=$ANSI_GREEN_OVERRIDE"
  echo "ansi_yellow_override=$ANSI_YELLOW_OVERRIDE"
  echo "ansi_blue_override=$ANSI_BLUE_OVERRIDE"
  echo "ansi_magenta_override=$ANSI_MAGENTA_OVERRIDE"
  echo "ansi_cyan_override=$ANSI_CYAN_OVERRIDE"
  echo "peaclock_themed=true"
  echo "starship_themed=true"
  echo "nvim_themed=true"
} > "$BACKUP_DIR/meta.env"

if command -v termux-reload-settings >/dev/null 2>&1; then
  termux-reload-settings || true
fi
if command -v tmux >/dev/null 2>&1; then
  tmux source-file "$TMUX_CONF" 2>/dev/null || true
fi

echo "Applied Material theme."
echo "Backup created: $BACKUP_DIR"
