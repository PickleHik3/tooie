#!/data/data/com.termux/files/usr/bin/sh
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
    mode="$(sed -n 's/^mode=//p' "$d/meta.env" | head -n 1)"
    wallpaper="$(sed -n 's/^wallpaper=//p' "$d/meta.env" | head -n 1)"
    printf '%s  mode=%s  wallpaper=%s\n' "$bname" "${mode:-unknown}" "${wallpaper:-unknown}"
  else
    printf '%s\n' "$bname"
  fi
done
