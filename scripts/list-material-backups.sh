#!/usr/bin/env sh
set -eu

HOME_DIR="${HOME:-/data/data/com.termux/files/home}"
BACKUP_ROOT="$HOME_DIR/.config/tooie/backups"

if [ ! -d "$BACKUP_ROOT" ]; then
  echo "No backups directory: $BACKUP_ROOT"
  exit 0
fi

printf 'Available backups:\n'
for d in "$BACKUP_ROOT"/*; do
  [ -d "$d" ] || continue
  bname="$(basename "$d")"
  if [ -f "$d/meta.env" ]; then
    source="$(sed -n 's/^theme_source=//p' "$d/meta.env" | head -n 1)"
    mode="$(sed -n 's/^effective_mode=//p' "$d/meta.env" | head -n 1)"
    palette="$(sed -n 's/^status_palette=//p' "$d/meta.env" | head -n 1)"
    family="$(sed -n 's/^preset_family=//p' "$d/meta.env" | head -n 1)"
    variant="$(sed -n 's/^preset_variant=//p' "$d/meta.env" | head -n 1)"
    wallpaper="$(sed -n 's/^wallpaper=//p' "$d/meta.env" | head -n 1)"
    if [ -n "$family" ] || [ -n "$variant" ]; then
      preset="${family:-unknown}:${variant:-unknown}"
    else
      preset="-"
    fi
    printf '%s  source=%s  mode=%s  palette=%s  preset=%s  wallpaper=%s\n' \
      "$bname" "${source:-unknown}" "${mode:-unknown}" "${palette:-unknown}" "$preset" "${wallpaper:-unknown}"
  else
    printf '%s\n' "$bname"
  fi
done
