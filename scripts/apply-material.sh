#!/data/data/com.termux/files/usr/bin/sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_DIR="$HOME_DIR/.config/tooie"
BACKUP_ROOT="$TOOIE_DIR/backups"
WALLPAPER_FIXED="$HOME_DIR/.termux/background/background_portrait.jpeg"
WALLPAPER_DIR="$HOME_DIR/.termux/background"
BACKUP_KEEP=5
TERMUX_COLORS="$HOME_DIR/.termux/colors.properties"
TMUX_CONF="$HOME_DIR/.tmux.conf"
PEACLOCK_CONFIG="$HOME_DIR/.config/peaclock/config"
STARSHIP_CONFIG="$HOME_DIR/.config/starship.toml"

MODE="dark"
SCHEME_TYPE="scheme-tonal-spot"
WALLPAPER=""
THEME_SOURCE="wallpaper"
PRESET_FAMILY="catppuccin"
PRESET_VARIANT=""
MATUGEN_BIN="${MATUGEN_BIN:-}"
TEXT_COLOR_OVERRIDE=""
CURSOR_COLOR_OVERRIDE=""
STATUS_PALETTE="default"
STYLE_PRESET="default"
ANSI_RED_OVERRIDE=""
ANSI_GREEN_OVERRIDE=""
ANSI_YELLOW_OVERRIDE=""
ANSI_BLUE_OVERRIDE=""
ANSI_MAGENTA_OVERRIDE=""
ANSI_CYAN_OVERRIDE=""
PREVIEW_ONLY=0
REUSE_BACKUP_ID=""

usage() {
  cat <<'EOF'
Usage: apply-material.sh [-m dark|light] [-t scheme-type] [-w wallpaper_path] [-b matugen_bin]
                         [--theme-source wallpaper|preset]
                         [--preset-family catppuccin|rose-pine|tokyo-night|synthwave-84]
                         [--preset-variant name]
                         [--text-color '#rrggbb'] [--cursor-color '#rrggbb']
                         [--status-palette default|vibrant]
                         [--style-preset default|vivid|playful|energetic|creative|friendly|positive]
                         [--preview-only] [--reuse-backup backup_id]
                         [--ansi-red '#rrggbb'] [--ansi-green '#rrggbb']
                         [--ansi-yellow '#rrggbb'] [--ansi-blue '#rrggbb']
                         [--ansi-magenta '#rrggbb'] [--ansi-cyan '#rrggbb']

Defaults:
  mode: dark
  type: scheme-tonal-spot
  wallpaper: ~/.termux/background/background_portrait.jpeg
             (fallback: newest file in ~/.termux/background)
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
    --theme-source)
      THEME_SOURCE="${2:-}"
      shift 2
      ;;
    --preset-family)
      PRESET_FAMILY="${2:-}"
      shift 2
      ;;
    --preset-variant)
      PRESET_VARIANT="${2:-}"
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
    --preview-only)
      PREVIEW_ONLY=1
      shift 1
      ;;
    --reuse-backup)
      REUSE_BACKUP_ID="${2:-}"
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

list_backup_dirs_by_mtime_desc() {
  [ -d "$BACKUP_ROOT" ] || return 0
  ls -1dt "$BACKUP_ROOT"/* 2>/dev/null || true
}

prune_old_backups() {
  [ -d "$BACKUP_ROOT" ] || return 0
  list_backup_dirs_by_mtime_desc | awk -v keep="$BACKUP_KEEP" 'NR > keep { print }' | while IFS= read -r dir; do
    [ -n "$dir" ] || continue
    rm -rf "$dir"
  done
}

apply_preset_defaults() {
  case "$PRESET_FAMILY" in
    catppuccin)
      [ -n "$PRESET_VARIANT" ] || PRESET_VARIANT="mocha"
      ;;
    rose-pine)
      [ -n "$PRESET_VARIANT" ] || PRESET_VARIANT="main"
      ;;
    tokyo-night)
      [ -n "$PRESET_VARIANT" ] || PRESET_VARIANT="storm"
      ;;
    synthwave-84)
      PRESET_VARIANT="default"
      ;;
  esac
}

load_preset_theme() {
  case "$PRESET_FAMILY:$PRESET_VARIANT" in
    catppuccin:latte)
      PRESET_MODE="light"
      PRESET_BG="#eff1f5"; PRESET_SURFACE="#e6e9ef"; PRESET_FG="#4c4f69"; PRESET_OUTLINE="#9ca0b0"
      PRESET_PRIMARY="#1e66f5"; PRESET_SECONDARY="#179299"; PRESET_TERTIARY="#8839ef"; PRESET_ERROR="#d20f39"
      PRESET_RED="#d20f39"; PRESET_GREEN="#40a02b"; PRESET_YELLOW="#df8e1d"; PRESET_BLUE="#1e66f5"; PRESET_MAGENTA="#ea76cb"; PRESET_CYAN="#179299"
      ;;
    catppuccin:frappe)
      PRESET_MODE="dark"
      PRESET_BG="#303446"; PRESET_SURFACE="#292c3c"; PRESET_FG="#c6d0f5"; PRESET_OUTLINE="#737994"
      PRESET_PRIMARY="#8caaee"; PRESET_SECONDARY="#81c8be"; PRESET_TERTIARY="#ca9ee6"; PRESET_ERROR="#e78284"
      PRESET_RED="#e78284"; PRESET_GREEN="#a6d189"; PRESET_YELLOW="#e5c890"; PRESET_BLUE="#8caaee"; PRESET_MAGENTA="#f4b8e4"; PRESET_CYAN="#81c8be"
      ;;
    catppuccin:macchiato)
      PRESET_MODE="dark"
      PRESET_BG="#24273a"; PRESET_SURFACE="#1f2230"; PRESET_FG="#cad3f5"; PRESET_OUTLINE="#6e738d"
      PRESET_PRIMARY="#8aadf4"; PRESET_SECONDARY="#8bd5ca"; PRESET_TERTIARY="#c6a0f6"; PRESET_ERROR="#ed8796"
      PRESET_RED="#ed8796"; PRESET_GREEN="#a6da95"; PRESET_YELLOW="#eed49f"; PRESET_BLUE="#8aadf4"; PRESET_MAGENTA="#f5bde6"; PRESET_CYAN="#8bd5ca"
      ;;
    catppuccin:mocha)
      PRESET_MODE="dark"
      PRESET_BG="#1e1e2e"; PRESET_SURFACE="#181825"; PRESET_FG="#cdd6f4"; PRESET_OUTLINE="#6c7086"
      PRESET_PRIMARY="#89b4fa"; PRESET_SECONDARY="#94e2d5"; PRESET_TERTIARY="#cba6f7"; PRESET_ERROR="#f38ba8"
      PRESET_RED="#f38ba8"; PRESET_GREEN="#a6e3a1"; PRESET_YELLOW="#f9e2af"; PRESET_BLUE="#89b4fa"; PRESET_MAGENTA="#f5c2e7"; PRESET_CYAN="#94e2d5"
      ;;
    rose-pine:main)
      PRESET_MODE="dark"
      PRESET_BG="#191724"; PRESET_SURFACE="#1f1d2e"; PRESET_FG="#e0def4"; PRESET_OUTLINE="#524f67"
      PRESET_PRIMARY="#9ccfd8"; PRESET_SECONDARY="#f6c177"; PRESET_TERTIARY="#c4a7e7"; PRESET_ERROR="#eb6f92"
      PRESET_RED="#eb6f92"; PRESET_GREEN="#31748f"; PRESET_YELLOW="#f6c177"; PRESET_BLUE="#9ccfd8"; PRESET_MAGENTA="#c4a7e7"; PRESET_CYAN="#ebbcba"
      ;;
    rose-pine:moon)
      PRESET_MODE="dark"
      PRESET_BG="#232136"; PRESET_SURFACE="#2a273f"; PRESET_FG="#e0def4"; PRESET_OUTLINE="#6e6a86"
      PRESET_PRIMARY="#9ccfd8"; PRESET_SECONDARY="#ea9a97"; PRESET_TERTIARY="#c4a7e7"; PRESET_ERROR="#eb6f92"
      PRESET_RED="#eb6f92"; PRESET_GREEN="#3e8fb0"; PRESET_YELLOW="#f6c177"; PRESET_BLUE="#9ccfd8"; PRESET_MAGENTA="#c4a7e7"; PRESET_CYAN="#ea9a97"
      ;;
    rose-pine:dawn)
      PRESET_MODE="light"
      PRESET_BG="#faf4ed"; PRESET_SURFACE="#fffaf3"; PRESET_FG="#575279"; PRESET_OUTLINE="#9893a5"
      PRESET_PRIMARY="#56949f"; PRESET_SECONDARY="#d7827e"; PRESET_TERTIARY="#907aa9"; PRESET_ERROR="#b4637a"
      PRESET_RED="#b4637a"; PRESET_GREEN="#286983"; PRESET_YELLOW="#ea9d34"; PRESET_BLUE="#56949f"; PRESET_MAGENTA="#907aa9"; PRESET_CYAN="#d7827e"
      ;;
    tokyo-night:storm)
      PRESET_MODE="dark"
      PRESET_BG="#24283b"; PRESET_SURFACE="#1f2335"; PRESET_FG="#c0caf5"; PRESET_OUTLINE="#565f89"
      PRESET_PRIMARY="#7aa2f7"; PRESET_SECONDARY="#7dcfff"; PRESET_TERTIARY="#bb9af7"; PRESET_ERROR="#f7768e"
      PRESET_RED="#f7768e"; PRESET_GREEN="#9ece6a"; PRESET_YELLOW="#e0af68"; PRESET_BLUE="#7aa2f7"; PRESET_MAGENTA="#bb9af7"; PRESET_CYAN="#7dcfff"
      ;;
    tokyo-night:moon)
      PRESET_MODE="dark"
      PRESET_BG="#222436"; PRESET_SURFACE="#1e2030"; PRESET_FG="#c8d3f5"; PRESET_OUTLINE="#636da6"
      PRESET_PRIMARY="#82aaff"; PRESET_SECONDARY="#86e1fc"; PRESET_TERTIARY="#c099ff"; PRESET_ERROR="#ff757f"
      PRESET_RED="#ff757f"; PRESET_GREEN="#c3e88d"; PRESET_YELLOW="#ffc777"; PRESET_BLUE="#82aaff"; PRESET_MAGENTA="#c099ff"; PRESET_CYAN="#86e1fc"
      ;;
    tokyo-night:night)
      PRESET_MODE="dark"
      PRESET_BG="#1a1b26"; PRESET_SURFACE="#16161e"; PRESET_FG="#c0caf5"; PRESET_OUTLINE="#414868"
      PRESET_PRIMARY="#7aa2f7"; PRESET_SECONDARY="#7dcfff"; PRESET_TERTIARY="#bb9af7"; PRESET_ERROR="#f7768e"
      PRESET_RED="#f7768e"; PRESET_GREEN="#9ece6a"; PRESET_YELLOW="#e0af68"; PRESET_BLUE="#7aa2f7"; PRESET_MAGENTA="#bb9af7"; PRESET_CYAN="#7dcfff"
      ;;
    tokyo-night:day)
      PRESET_MODE="light"
      PRESET_BG="#e1e2e7"; PRESET_SURFACE="#d5d6db"; PRESET_FG="#3760bf"; PRESET_OUTLINE="#9699a3"
      PRESET_PRIMARY="#2e7de9"; PRESET_SECONDARY="#007197"; PRESET_TERTIARY="#9854f1"; PRESET_ERROR="#f52a65"
      PRESET_RED="#f52a65"; PRESET_GREEN="#587539"; PRESET_YELLOW="#8c6c3e"; PRESET_BLUE="#2e7de9"; PRESET_MAGENTA="#9854f1"; PRESET_CYAN="#007197"
      ;;
    synthwave-84:default)
      PRESET_MODE="dark"
      PRESET_BG="#241b2f"; PRESET_SURFACE="#2a2139"; PRESET_FG="#f8f8f2"; PRESET_OUTLINE="#495495"
      PRESET_PRIMARY="#36f9f6"; PRESET_SECONDARY="#fede5d"; PRESET_TERTIARY="#ff7edb"; PRESET_ERROR="#ff5c8a"
      PRESET_RED="#ff5c8a"; PRESET_GREEN="#72f1b8"; PRESET_YELLOW="#fede5d"; PRESET_BLUE="#36f9f6"; PRESET_MAGENTA="#ff7edb"; PRESET_CYAN="#36f9f6"
      ;;
    *)
      echo "Invalid preset selection: $PRESET_FAMILY:$PRESET_VARIANT" >&2
      exit 1
      ;;
  esac
}

case "$THEME_SOURCE" in
  wallpaper|preset) ;;
  *)
    echo "Invalid theme source: $THEME_SOURCE (use wallpaper or preset)" >&2
    exit 1
    ;;
esac

if [ "$THEME_SOURCE" = "preset" ]; then
  apply_preset_defaults
  load_preset_theme
  MODE="$PRESET_MODE"
else
  MATUGEN_BIN="$(resolve_matugen_bin || true)"
  if [ -z "$MATUGEN_BIN" ]; then
    echo "matugen binary not found. Set MATUGEN_BIN or install matugen." >&2
    exit 1
  fi
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
  default|vivid|playful|energetic|creative|friendly|positive) ;;
  *)
    echo "Invalid style preset: $STYLE_PRESET" >&2
    echo "Use one of: default, vivid, playful, energetic, creative, friendly, positive" >&2
    exit 1
    ;;
esac

if [ "$THEME_SOURCE" = "wallpaper" ]; then
  if [ -z "$WALLPAPER" ]; then
    WALLPAPER="$(pick_latest_wallpaper || true)"
  fi

  if [ -z "$WALLPAPER" ] || [ ! -f "$WALLPAPER" ]; then
    echo "Wallpaper not found. Expected $WALLPAPER_FIXED (or use -w)." >&2
    exit 1
  fi
else
  WALLPAPER=""
fi

mkdir -p "$BACKUP_ROOT" "$HOME_DIR/.termux"
if [ -n "$REUSE_BACKUP_ID" ]; then
  STAMP="$REUSE_BACKUP_ID"
  BACKUP_DIR="$BACKUP_ROOT/$STAMP"
  if [ ! -d "$BACKUP_DIR" ]; then
    echo "Preview backup not found: $BACKUP_DIR" >&2
    exit 1
  fi
else
  STAMP="${STYLE_PRESET}_$(date +%Y%m%d-%H%M%S)"
  BACKUP_DIR="$BACKUP_ROOT/$STAMP"
  mkdir -p "$BACKUP_DIR"
fi
PROGRESS_FILE="${TOOIE_APPLY_PROGRESS_FILE:-$TOOIE_DIR/apply-progress.json}"

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

write_progress() {
  label="$1"
  progress="$2"
  [ -n "$PROGRESS_FILE" ] || return 0
  mkdir -p "$(dirname "$PROGRESS_FILE")"
  printf '{"label":"%s","progress":%s}\n' "$(json_escape "$label")" "$progress" > "$PROGRESS_FILE"
}

write_progress "Preparing theme" 0.05

if [ -z "$REUSE_BACKUP_ID" ]; then
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
fi

JSON_FILE="$BACKUP_DIR/matugen.json"
if [ -n "$REUSE_BACKUP_ID" ]; then
  write_progress "Reusing updated preview" 0.14
  if [ ! -f "$JSON_FILE" ]; then
    echo "Preview data missing: $JSON_FILE" >&2
    exit 1
  fi
else
  if [ "$THEME_SOURCE" = "preset" ]; then
    write_progress "Loading preset palette" 0.14
    cat > "$JSON_FILE" <<EOF
{"colors":{
  "background":{"default":{"color":"$PRESET_BG"}},
  "surface_container":{"default":{"color":"$PRESET_SURFACE"}},
  "surface_container_high":{"default":{"color":"$PRESET_SURFACE"}},
  "surface_variant":{"default":{"color":"$PRESET_SURFACE"}},
  "surface_dim":{"default":{"color":"$PRESET_BG"}},
  "surface_bright":{"default":{"color":"$PRESET_SURFACE"}},
  "on_background":{"default":{"color":"$PRESET_FG"}},
  "on_surface":{"default":{"color":"$PRESET_FG"}},
  "on_surface_variant":{"default":{"color":"$PRESET_OUTLINE"}},
  "outline":{"default":{"color":"$PRESET_OUTLINE"}},
  "outline_variant":{"default":{"color":"$PRESET_OUTLINE"}},
  "primary":{"default":{"color":"$PRESET_PRIMARY"}},
  "secondary":{"default":{"color":"$PRESET_SECONDARY"}},
  "tertiary":{"default":{"color":"$PRESET_TERTIARY"}},
  "error":{"default":{"color":"$PRESET_ERROR"}},
  "secondary_fixed":{"default":{"color":"$PRESET_SECONDARY"}},
  "tertiary_fixed":{"default":{"color":"$PRESET_TERTIARY"}}
}}
EOF
  else
    write_progress "Extracting wallpaper roles" 0.14
    "$MATUGEN_BIN" image "$WALLPAPER" -m "$MODE" -t "$SCHEME_TYPE" --source-color-index 0 -j hex --dry-run > "$JSON_FILE"
  fi
fi
write_progress "Deriving semantic palette" 0.28

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

if [ "$THEME_SOURCE" = "preset" ]; then
  BG="$(normalize_hex "$PRESET_BG")"
  C0="$(normalize_hex "$PRESET_SURFACE")"
  FG="$(ensure_contrast "$PRESET_FG" "$BG" 7.0)"
  CURSOR="$(ensure_contrast "$PRESET_PRIMARY" "$BG" 4.5)"
  C1="$(ensure_contrast "$PRESET_RED" "$BG" 3.6)"
  C2="$(ensure_contrast "$PRESET_GREEN" "$BG" 3.6)"
  C3="$(ensure_contrast "$PRESET_YELLOW" "$BG" 3.6)"
  C4="$(ensure_contrast "$PRESET_BLUE" "$BG" 3.6)"
  C5="$(ensure_contrast "$PRESET_MAGENTA" "$BG" 3.6)"
  C6="$(ensure_contrast "$PRESET_CYAN" "$BG" 3.6)"
  C7="$(ensure_contrast "$PRESET_FG" "$BG" 6.0)"
  C14="$(ensure_contrast "$PRESET_OUTLINE" "$BG" 3.2)"
  C15="$(ensure_contrast "$(mix_hex "$PRESET_FG" "$BG" 0.34)" "$BG" 4.5)"
  EFFECTIVE_BACKGROUND="$BG"
  EFFECTIVE_SURFACE="$C0"
  EFFECTIVE_ON_SURFACE="$FG"
  EFFECTIVE_OUTLINE="$C14"
  EFFECTIVE_PRIMARY="$(ensure_contrast "$PRESET_PRIMARY" "$BG" 4.0)"
  EFFECTIVE_SECONDARY="$(ensure_contrast "$PRESET_SECONDARY" "$BG" 4.0)"
  EFFECTIVE_TERTIARY="$(ensure_contrast "$PRESET_TERTIARY" "$BG" 4.0)"
  EFFECTIVE_ERROR="$(ensure_contrast "$PRESET_ERROR" "$BG" 4.0)"
  if [ "$MODE" = "dark" ]; then
    C8="$(ensure_contrast "$(mix_hex "$C0" "#ffffff" 0.18)" "$BG" 1.3)"
    C9="$(ensure_contrast "$(mix_hex "$C1" "#ffffff" 0.20)" "$BG" 4.0)"
    C10="$(ensure_contrast "$(mix_hex "$C2" "#ffffff" 0.18)" "$BG" 4.0)"
    C11="$(ensure_contrast "$(mix_hex "$C3" "#ffffff" 0.18)" "$BG" 4.0)"
    C12="$(ensure_contrast "$(mix_hex "$C4" "#ffffff" 0.18)" "$BG" 4.0)"
    C13="$(ensure_contrast "$(mix_hex "$C5" "#ffffff" 0.18)" "$BG" 4.0)"
    C16="$(ensure_contrast "$(mix_hex "$C2" "$C6" 0.36)" "$BG" 3.2)"
    C17="$(ensure_contrast "$(mix_hex "$C5" "$C4" 0.36)" "$BG" 3.2)"
    C18="$(mix_hex "$BG" "#000000" 0.10)"
    C19="$(mix_hex "$BG" "#ffffff" 0.12)"
  else
    C8="$(ensure_contrast "$(mix_hex "$C0" "#000000" 0.12)" "$BG" 1.3)"
    C9="$(ensure_contrast "$(mix_hex "$C1" "#000000" 0.18)" "$BG" 4.0)"
    C10="$(ensure_contrast "$(mix_hex "$C2" "#000000" 0.18)" "$BG" 4.0)"
    C11="$(ensure_contrast "$(mix_hex "$C3" "#000000" 0.18)" "$BG" 4.0)"
    C12="$(ensure_contrast "$(mix_hex "$C4" "#000000" 0.18)" "$BG" 4.0)"
    C13="$(ensure_contrast "$(mix_hex "$C5" "#000000" 0.18)" "$BG" 4.0)"
    C16="$(ensure_contrast "$(mix_hex "$C2" "$C6" 0.32)" "$BG" 3.2)"
    C17="$(ensure_contrast "$(mix_hex "$C5" "$C4" 0.32)" "$BG" 3.2)"
    C18="$(mix_hex "$BG" "#000000" 0.06)"
    C19="$(mix_hex "$BG" "#ffffff" 0.06)"
  fi
  C20="$(ensure_contrast "$(mix_hex "$C0" "$C15" 0.24)" "$BG" 1.5)"
  C21="$(ensure_contrast "$(mix_hex "$C14" "$C0" 0.28)" "$BG" 1.4)"
elif [ "$STYLE_PRESET" = "default" ]; then
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
  EFFECTIVE_BACKGROUND="$BG"
  EFFECTIVE_SURFACE="$C0"
  EFFECTIVE_ON_SURFACE="$FG"
  EFFECTIVE_OUTLINE="$C14"
  EFFECTIVE_PRIMARY="$(ensure_contrast "$PURE_PRIMARY" "$BG" 4.0)"
  EFFECTIVE_SECONDARY="$(ensure_contrast "$PURE_SECONDARY" "$BG" 4.0)"
  EFFECTIVE_TERTIARY="$(ensure_contrast "$PURE_TERTIARY" "$BG" 4.0)"
  EFFECTIVE_ERROR="$(ensure_contrast "$PURE_ERROR" "$BG" 4.0)"
else
  if [ "$MODE" = "dark" ]; then
    case "$STYLE_PRESET" in
      vivid)
        ANCHOR_BG="#15171f"; ANCHOR_FG="#e5e9f0"; ANCHOR_RED="#ff6b7a"; ANCHOR_GREEN="#6fe3a1"; ANCHOR_YELLOW="#ffd166"; ANCHOR_BLUE="#5fb0ff"; ANCHOR_MAGENTA="#d28cff"; ANCHOR_CYAN="#4fe0d0"
        ;;
      playful)
        ANCHOR_BG="#171822"; ANCHOR_FG="#ebe7ff"; ANCHOR_RED="#ff7f96"; ANCHOR_GREEN="#7be495"; ANCHOR_YELLOW="#ffd166"; ANCHOR_BLUE="#7aa2ff"; ANCHOR_MAGENTA="#d792ff"; ANCHOR_CYAN="#66e3ff"
        ;;
      energetic)
        ANCHOR_BG="#1a1612"; ANCHOR_FG="#f2e7de"; ANCHOR_RED="#ff6b5f"; ANCHOR_GREEN="#78d98b"; ANCHOR_YELLOW="#ffbf5a"; ANCHOR_BLUE="#5ba8ff"; ANCHOR_MAGENTA="#ff84c1"; ANCHOR_CYAN="#42d6c5"
        ;;
      creative)
        ANCHOR_BG="#151820"; ANCHOR_FG="#e8edf7"; ANCHOR_RED="#ff7285"; ANCHOR_GREEN="#67d9a2"; ANCHOR_YELLOW="#ffc857"; ANCHOR_BLUE="#57a6ff"; ANCHOR_MAGENTA="#bb86ff"; ANCHOR_CYAN="#32d4ff"
        ;;
      friendly)
        ANCHOR_BG="#17191c"; ANCHOR_FG="#e6ece9"; ANCHOR_RED="#f17c8f"; ANCHOR_GREEN="#78d1a2"; ANCHOR_YELLOW="#f3c96b"; ANCHOR_BLUE="#78aee8"; ANCHOR_MAGENTA="#c09de8"; ANCHOR_CYAN="#68d5d3"
        ;;
      positive)
        ANCHOR_BG="#151912"; ANCHOR_FG="#edf0df"; ANCHOR_RED="#ff7c6b"; ANCHOR_GREEN="#89db6a"; ANCHOR_YELLOW="#ffd95c"; ANCHOR_BLUE="#65a6ff"; ANCHOR_MAGENTA="#d29dff"; ANCHOR_CYAN="#59dbba"
        ;;
      *)
        ANCHOR_BG="#15171f"; ANCHOR_FG="#e5e9f0"; ANCHOR_RED="#ff6b7a"; ANCHOR_GREEN="#6fe3a1"; ANCHOR_YELLOW="#ffd166"; ANCHOR_BLUE="#5fb0ff"; ANCHOR_MAGENTA="#d28cff"; ANCHOR_CYAN="#4fe0d0"
        ;;
    esac
  else
    case "$STYLE_PRESET" in
      vivid)
        ANCHOR_BG="#f3f5fb"; ANCHOR_FG="#243041"; ANCHOR_RED="#cf334f"; ANCHOR_GREEN="#16794a"; ANCHOR_YELLOW="#8f6300"; ANCHOR_BLUE="#005ac1"; ANCHOR_MAGENTA="#7d3aca"; ANCHOR_CYAN="#006a6a"
        ;;
      playful)
        ANCHOR_BG="#f7f3ff"; ANCHOR_FG="#3e3452"; ANCHOR_RED="#c73f68"; ANCHOR_GREEN="#297a49"; ANCHOR_YELLOW="#9a6700"; ANCHOR_BLUE="#375fd3"; ANCHOR_MAGENTA="#8d43d6"; ANCHOR_CYAN="#006f83"
        ;;
      energetic)
        ANCHOR_BG="#fff5ec"; ANCHOR_FG="#432d1b"; ANCHOR_RED="#c63b26"; ANCHOR_GREEN="#2f7b44"; ANCHOR_YELLOW="#9f6200"; ANCHOR_BLUE="#0059b3"; ANCHOR_MAGENTA="#a03c89"; ANCHOR_CYAN="#006e64"
        ;;
      creative)
        ANCHOR_BG="#f4f7ff"; ANCHOR_FG="#28354a"; ANCHOR_RED="#c73757"; ANCHOR_GREEN="#1f7b58"; ANCHOR_YELLOW="#8f6900"; ANCHOR_BLUE="#0057c8"; ANCHOR_MAGENTA="#6f45db"; ANCHOR_CYAN="#006d94"
        ;;
      friendly)
        ANCHOR_BG="#f5f7f5"; ANCHOR_FG="#31403d"; ANCHOR_RED="#b85369"; ANCHOR_GREEN="#2f7457"; ANCHOR_YELLOW="#876a1e"; ANCHOR_BLUE="#466fa8"; ANCHOR_MAGENTA="#7f63aa"; ANCHOR_CYAN="#2f8080"
        ;;
      positive)
        ANCHOR_BG="#f8fbe9"; ANCHOR_FG="#33411f"; ANCHOR_RED="#c54b3c"; ANCHOR_GREEN="#3a7d1d"; ANCHOR_YELLOW="#906700"; ANCHOR_BLUE="#2d66c4"; ANCHOR_MAGENTA="#8f57c9"; ANCHOR_CYAN="#18766d"
        ;;
      *)
        ANCHOR_BG="#f3f5fb"; ANCHOR_FG="#243041"; ANCHOR_RED="#cf334f"; ANCHOR_GREEN="#16794a"; ANCHOR_YELLOW="#8f6300"; ANCHOR_BLUE="#005ac1"; ANCHOR_MAGENTA="#7d3aca"; ANCHOR_CYAN="#006a6a"
        ;;
    esac
  fi

  case "$STYLE_PRESET" in
    vivid)
      BG_T="0.28"; SURFACE_T="0.42"; FG_T="0.12"; MUTED_T="0.30"; OUTLINE_T="0.42"; PRIMARY_T="0.82"; SECONDARY_T="0.80"; TERTIARY_T="0.78"; ERROR_T="0.76"
      PRIMARY_TARGET="$ANCHOR_BLUE"; SECONDARY_TARGET="$ANCHOR_CYAN"; TERTIARY_TARGET="$ANCHOR_MAGENTA"
      ;;
    playful)
      BG_T="0.24"; SURFACE_T="0.38"; FG_T="0.12"; MUTED_T="0.28"; OUTLINE_T="0.40"; PRIMARY_T="0.84"; SECONDARY_T="0.78"; TERTIARY_T="0.86"; ERROR_T="0.74"
      PRIMARY_TARGET="$ANCHOR_MAGENTA"; SECONDARY_TARGET="$ANCHOR_BLUE"; TERTIARY_TARGET="$ANCHOR_YELLOW"
      ;;
    energetic)
      BG_T="0.34"; SURFACE_T="0.48"; FG_T="0.10"; MUTED_T="0.26"; OUTLINE_T="0.46"; PRIMARY_T="0.88"; SECONDARY_T="0.84"; TERTIARY_T="0.74"; ERROR_T="0.84"
      PRIMARY_TARGET="$ANCHOR_YELLOW"; SECONDARY_TARGET="$ANCHOR_RED"; TERTIARY_TARGET="$ANCHOR_BLUE"
      ;;
    creative)
      BG_T="0.30"; SURFACE_T="0.44"; FG_T="0.10"; MUTED_T="0.28"; OUTLINE_T="0.44"; PRIMARY_T="0.86"; SECONDARY_T="0.80"; TERTIARY_T="0.82"; ERROR_T="0.76"
      PRIMARY_TARGET="$ANCHOR_MAGENTA"; SECONDARY_TARGET="$ANCHOR_BLUE"; TERTIARY_TARGET="$ANCHOR_CYAN"
      ;;
    friendly)
      BG_T="0.26"; SURFACE_T="0.36"; FG_T="0.14"; MUTED_T="0.30"; OUTLINE_T="0.38"; PRIMARY_T="0.82"; SECONDARY_T="0.72"; TERTIARY_T="0.72"; ERROR_T="0.70"
      PRIMARY_TARGET="$ANCHOR_GREEN"; SECONDARY_TARGET="$ANCHOR_BLUE"; TERTIARY_TARGET="$ANCHOR_YELLOW"
      ;;
    positive)
      BG_T="0.30"; SURFACE_T="0.40"; FG_T="0.12"; MUTED_T="0.28"; OUTLINE_T="0.40"; PRIMARY_T="0.86"; SECONDARY_T="0.82"; TERTIARY_T="0.70"; ERROR_T="0.72"
      PRIMARY_TARGET="$ANCHOR_GREEN"; SECONDARY_TARGET="$ANCHOR_YELLOW"; TERTIARY_TARGET="$ANCHOR_BLUE"
      ;;
    *)
      BG_T="0.28"; SURFACE_T="0.42"; FG_T="0.12"; MUTED_T="0.30"; OUTLINE_T="0.42"; PRIMARY_T="0.82"; SECONDARY_T="0.80"; TERTIARY_T="0.78"; ERROR_T="0.76"
      PRIMARY_TARGET="$ANCHOR_BLUE"; SECONDARY_TARGET="$ANCHOR_CYAN"; TERTIARY_TARGET="$ANCHOR_MAGENTA"
      ;;
  esac

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

  BG="$(mix_hex "$BG_BASE" "$ANCHOR_BG" "$BG_T")"
  C0="$(mix_hex "$SURFACE_BASE" "$ANCHOR_BG" "$SURFACE_T")"
  FG="$(ensure_contrast "$(mix_hex "$FG_BASE" "$ANCHOR_FG" "$FG_T")" "$BG" 7.0)"
  CURSOR="$(ensure_contrast "$(mix_hex "$CURSOR_BASE" "$PRIMARY_TARGET" "0.72")" "$BG" 4.5)"
  C7="$(ensure_contrast "$(role_color on_surface "$FG")" "$BG" 6.0)"
  C15="$(ensure_contrast "$(mix_hex "$MUTED_BASE" "$ANCHOR_FG" "$MUTED_T")" "$BG" 4.5)"
  C14="$(ensure_contrast "$(mix_hex "$OUTLINE_BASE" "$ANCHOR_FG" "$OUTLINE_T")" "$BG" 3.2)"

  EFFECTIVE_PRIMARY="$(ensure_contrast "$(mix_hex "$PRIMARY_BASE" "$PRIMARY_TARGET" "$PRIMARY_T")" "$BG" 4.0)"
  EFFECTIVE_SECONDARY="$(ensure_contrast "$(mix_hex "$SECONDARY_BASE" "$SECONDARY_TARGET" "$SECONDARY_T")" "$BG" 4.0)"
  EFFECTIVE_TERTIARY="$(ensure_contrast "$(mix_hex "$TERTIARY_BASE" "$TERTIARY_TARGET" "$TERTIARY_T")" "$BG" 4.0)"
  EFFECTIVE_ERROR="$(ensure_contrast "$(mix_hex "$ERROR_BASE" "$ANCHOR_RED" "$ERROR_T")" "$BG" 4.0)"
  EFFECTIVE_BACKGROUND="$BG"
  EFFECTIVE_SURFACE="$C0"
  EFFECTIVE_ON_SURFACE="$FG"
  EFFECTIVE_OUTLINE="$C14"

  C1="$EFFECTIVE_ERROR"
  C2="$(ensure_contrast "$(mix_hex "$TERTIARY_BASE" "$ANCHOR_GREEN" 0.76)" "$BG" 3.6)"
  C3="$(ensure_contrast "$(mix_hex "$SECONDARY_BASE" "$ANCHOR_YELLOW" 0.78)" "$BG" 3.6)"
  C4="$(ensure_contrast "$(mix_hex "$PRIMARY_BASE" "$ANCHOR_BLUE" 0.76)" "$BG" 3.6)"
  C5="$(ensure_contrast "$(mix_hex "$TERTIARY_BASE" "$ANCHOR_MAGENTA" 0.78)" "$BG" 3.6)"
  C6="$(ensure_contrast "$(mix_hex "$SECONDARY_BASE" "$ANCHOR_CYAN" 0.78)" "$BG" 3.6)"

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

STATE_RED="$(ensure_contrast "$C1" "$BG" 3.6)"
STATE_ORANGE="$(ensure_contrast "$(mix_hex "$C1" "$C3" 0.58)" "$BG" 3.6)"
STATE_YELLOW="$(ensure_contrast "$C3" "$BG" 3.6)"
STATE_GREEN="$(ensure_contrast "$C2" "$BG" 3.6)"
STATE_MINT="$(ensure_contrast "$(mix_hex "$C2" "$C6" 0.42)" "$BG" 3.6)"
STATE_BLUE="$(ensure_contrast "$C4" "$BG" 3.6)"
STATE_TEAL="$(ensure_contrast "$C6" "$BG" 3.6)"
STATE_VIOLET="$(ensure_contrast "$(mix_hex "$C4" "$C5" 0.55)" "$BG" 3.6)"
STATE_MAGENTA="$(ensure_contrast "$C5" "$BG" 3.6)"

if [ "$STATUS_PALETTE" = "vibrant" ]; then
  SEP_STATUS="$(ensure_contrast "$(mix_hex "$C14" "$C12" 0.55)" "$BG" 3.2)"
  WEATHER_STATUS="$(ensure_contrast "$(mix_hex "$STATE_BLUE" "$STATE_TEAL" 0.45)" "$BG" 3.6)"
  CHARGING_STATUS="$(ensure_contrast "$(mix_hex "$STATE_GREEN" "$STATE_MINT" 0.40)" "$BG" 3.6)"
else
  SEP_STATUS="$(ensure_contrast "$C14" "$BG" 3.0)"
  WEATHER_STATUS="$(ensure_contrast "$STATE_BLUE" "$BG" 3.4)"
  CHARGING_STATUS="$(ensure_contrast "$STATE_GREEN" "$BG" 3.4)"
fi

BAT_1="$STATE_RED"
BAT_2="$STATE_ORANGE"
BAT_3="$STATE_YELLOW"
BAT_4="$(ensure_contrast "$(mix_hex "$STATE_YELLOW" "$STATE_GREEN" 0.42)" "$BG" 3.6)"
BAT_5="$STATE_MINT"
BAT_6="$STATE_GREEN"

CPU_1="$STATE_GREEN"
CPU_2="$STATE_TEAL"
CPU_3="$STATE_BLUE"
CPU_4="$STATE_VIOLET"
CPU_5="$STATE_ORANGE"
CPU_6="$STATE_RED"

RAM_1="$STATE_BLUE"
RAM_2="$(ensure_contrast "$(mix_hex "$STATE_BLUE" "$STATE_TEAL" 0.55)" "$BG" 3.6)"
RAM_3="$STATE_TEAL"
RAM_4="$STATE_VIOLET"
RAM_5="$STATE_MAGENTA"
RAM_6="$STATE_RED"

{
  echo "backup_id=$STAMP"
  echo "theme_source=$THEME_SOURCE"
  if [ "$THEME_SOURCE" = "preset" ]; then
    echo "preset_family=$PRESET_FAMILY"
    echo "preset_variant=$PRESET_VARIANT"
  fi
  echo "wallpaper=$WALLPAPER"
  echo "mode=$MODE"
  echo "type=$SCHEME_TYPE"
  echo "matugen_bin=$MATUGEN_BIN"
  echo "text_color_override=$TEXT_COLOR_OVERRIDE"
  echo "cursor_color_override=$CURSOR_COLOR_OVERRIDE"
  echo "status_palette=$STATUS_PALETTE"
  echo "style_preset=$STYLE_PRESET"
  echo "effective_background=$EFFECTIVE_BACKGROUND"
  echo "effective_surface=$EFFECTIVE_SURFACE"
  echo "effective_on_surface=$EFFECTIVE_ON_SURFACE"
  echo "effective_outline=$EFFECTIVE_OUTLINE"
  echo "effective_primary=$EFFECTIVE_PRIMARY"
  echo "effective_secondary=$EFFECTIVE_SECONDARY"
  echo "effective_tertiary=$EFFECTIVE_TERTIARY"
  echo "effective_error=$EFFECTIVE_ERROR"
  echo "ansi_red_override=$ANSI_RED_OVERRIDE"
  echo "ansi_green_override=$ANSI_GREEN_OVERRIDE"
  echo "ansi_yellow_override=$ANSI_YELLOW_OVERRIDE"
  echo "ansi_blue_override=$ANSI_BLUE_OVERRIDE"
  echo "ansi_magenta_override=$ANSI_MAGENTA_OVERRIDE"
  echo "ansi_cyan_override=$ANSI_CYAN_OVERRIDE"
  if [ "$PREVIEW_ONLY" -eq 1 ]; then
    echo "preview_only=true"
  else
    echo "preview_only=false"
  fi
  if [ -n "$REUSE_BACKUP_ID" ]; then
    echo "reused_preview_id=$REUSE_BACKUP_ID"
  fi
} > "$BACKUP_DIR/meta.env"

if [ "$PREVIEW_ONLY" -eq 1 ]; then
  prune_old_backups
  write_progress "Preview ready" 1.0
  echo "Preview created: $BACKUP_DIR"
  exit 0
fi

TERMUX_TMP="$BACKUP_DIR/colors.properties.new"
write_progress "Writing Termux colors" 0.42
cat > "$TERMUX_TMP" <<EOF
# Generated by $TOOIE_DIR/apply-material.sh
# theme source: $THEME_SOURCE
# wallpaper: $WALLPAPER
# preset: $PRESET_FAMILY:$PRESET_VARIANT
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
write_progress "Writing tmux theme" 0.56
cat > "$TMUX_BLOCK_FILE" <<EOF
# >>> MATUGEN THEME START >>>
# Generated by $TOOIE_DIR/apply-material.sh
set -g status-style "bg=$BG,fg=$FG"
set -g status-left "#[fg=$STATUS_LEFT_FG,bg=$EFFECTIVE_PRIMARY,bold] #S #[bg=$BG,fg=$FG] "
set -g status-right "#{?client_prefix,PREFIX ,}#(\$HOME/.config/tmux/widget-battery) | #(\$HOME/.config/tmux/widget-cpu) | #(\$HOME/.config/tmux/widget-ram) | #(\$HOME/.config/tmux/widget-weather) "
set -g window-status-format "#[fg=$C14] #I:#W "
set -g window-status-current-format "#[fg=$EFFECTIVE_SECONDARY,bold] #I:#W "
set -g pane-border-style "fg=$C14"
set -g pane-active-border-style "fg=$EFFECTIVE_SECONDARY"
set -g message-style "bg=$BG,fg=$EFFECTIVE_SECONDARY"
set -g message-command-style "bg=$BG,fg=$EFFECTIVE_SECONDARY"
set -g mode-style "bg=$EFFECTIVE_SECONDARY,fg=$MODE_FG"
setw -g clock-mode-colour "$EFFECTIVE_SECONDARY"
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
write_progress "Writing peaclock theme" 0.68
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
style active-bg $EFFECTIVE_PRIMARY
style active-fg clear
style colon-fg $EFFECTIVE_PRIMARY
style colon-bg clear
style date $EFFECTIVE_TERTIARY
style text $C15
style prompt $EFFECTIVE_SECONDARY
style success $C10
style error $EFFECTIVE_ERROR
# <<< MATUGEN PEACLOCK END <<<
EOF
cp "$PEACLOCK_TMP" "$PEACLOCK_CONFIG"

if [ ! -f "$STARSHIP_CONFIG" ]; then
  mkdir -p "$HOME_DIR/.config"
  touch "$STARSHIP_CONFIG"
fi

STARSHIP_TMP="$BACKUP_DIR/starship.toml.new"
write_progress "Writing starship theme" 0.78
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

{
  echo "peaclock_themed=true"
  echo "starship_themed=true"
} >> "$BACKUP_DIR/meta.env"

write_progress "Reloading shell surfaces" 0.94
if command -v termux-reload-settings >/dev/null 2>&1; then
  termux-reload-settings || true
fi
if command -v tmux >/dev/null 2>&1; then
  tmux source-file "$TMUX_CONF" 2>/dev/null || true
fi

prune_old_backups
write_progress "Finishing theme apply" 1.0
echo "Applied Material theme."
echo "Backup created: $BACKUP_DIR"
echo "Fish config updated. Open a new shell or run: exec fish"
