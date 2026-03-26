#!/usr/bin/env sh
set -eu

HOME_DIR="${HOME:-$PWD}"
TOOIE_DIR="$HOME_DIR/.config/tooie"
CACHE_DIR="$HOME_DIR/.cache/tooie"
RUNNER="auto"

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

mkdir -p "$TOOIE_DIR" "$CACHE_DIR"

cat > "$TOOIE_DIR/helper.json" <<JSON
{
  "runner": "${RUNNER}"
}
JSON

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

printf '%s\n' "btop helper configured (runner=${RUNNER})."
printf '%s\n' "Optional stats override file: $CACHE_DIR/helper-stats.json"
