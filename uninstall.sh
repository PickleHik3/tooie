#!/usr/bin/env sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
TOOIE_BIN="$HOME_DIR/.local/bin/tooie"
TOOIE_DIR="$HOME_DIR/.config/tooie"
TOOIE_STATE_DIR="$HOME_DIR/.local/state/tooie"

remove_block() {
  file_path="$1"
  start_marker="$2"
  end_marker="$3"
  [ -f "$file_path" ] || return 0
  tmp_file="${file_path}.tooie.tmp.$$"
  awk -v begin="$start_marker" -v end="$end_marker" '
    $0 == begin {skip=1; next}
    $0 == end {skip=0; next}
    !skip {print}
  ' "$file_path" > "$tmp_file"
  mv "$tmp_file" "$file_path"
}

unlink_if_points_to_tooie() {
  path="$1"
  if [ -L "$path" ]; then
    target="$(readlink "$path" 2>/dev/null || true)"
    case "$target" in
      *"/.config/tooie/"*) rm -f "$path" ;;
    esac
  fi
}

cleanup_managed_integrations() {
  remove_block "$HOME_DIR/.tmux.conf" "# >>> TOOIE TMUX BOOTSTRAP START >>>" "# <<< TOOIE TMUX BOOTSTRAP END <<<"
  remove_block "$HOME_DIR/.tmux.conf" "# >>> MATUGEN THEME START >>>" "# <<< MATUGEN THEME END <<<"

  rm -f "$HOME_DIR/.config/fish/conf.d/tooie.fish"
  remove_block "$HOME_DIR/.config/fish/config.fish" "# >>> tooie-btop >>>" "# <<< tooie-btop <<<"

  unlink_if_points_to_tooie "$HOME_DIR/.config/peaclock/config"
  unlink_if_points_to_tooie "$HOME_DIR/.termux/termux.properties"
  unlink_if_points_to_tooie "$HOME_DIR/.termux/colors.properties"
  unlink_if_points_to_tooie "$HOME_DIR/.termux/font.ttf"
  unlink_if_points_to_tooie "$HOME_DIR/.termux/font-italic.ttf"
}

restored=0
if [ -x "$TOOIE_BIN" ]; then
  if "$TOOIE_BIN" helper uninstall --snapshot latest; then
    restored=1
    echo "Restored files from latest Tooie install snapshot."
  fi
fi

cleanup_managed_integrations

rm -f "$TOOIE_BIN"
rm -rf "$TOOIE_DIR" "$TOOIE_STATE_DIR"

if [ "$restored" -eq 1 ]; then
  echo "Tooie uninstall completed (snapshot restored + managed files removed)."
else
  echo "Tooie uninstall completed (no snapshot restore; removed managed files and binary)."
fi
