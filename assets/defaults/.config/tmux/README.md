# status-tmux

Portable tmux status widgets for Termux.

## Widgets

- `widget-cpu` (Shizuku backend first, local fallback)
- `widget-ram` (Shizuku backend first, local fallback)
- `widget-battery`
- `widget-weather`
- `widget-apps` (opens popup omni bar)

## Omni Apps Launcher

`widget-apps` now opens a tmux popup (`open-apps-popup`) running `search-apps` with `fzf`.

Behavior:
- Search Android + terminal entries in one list.
- Show pinned app row at the top (`[1]..[9]`).
- Launch pinned apps via `alt-1`..`alt-9`.
- Android launch path is **Shizuku/Tooie first** (`tooie exec "am start ..."`), then local `am start` fallback.
- Android app list is filtered to launcher-exposed apps only (`MAIN` + `LAUNCHER`) to avoid clutter from non-launcher packages/intents.

## Files

- `run-system-widget` - wrapper that reads tmux options and calls `system-widgets`
- `system-widgets` - CPU/RAM/Battery logic + caching
- `weather-cache` - weather fetch + cache
- `helpers.sh` - tmux option helper
- `open-apps-popup` - tmux popup opener
- `search-apps` - fzf omni bar + launcher logic
- `show-cheatsheet-popup` - quick keybind reference popup
- `show-cheatsheet-view` - popup content renderer
- `apps-menu.conf.example` - optional custom entries (`android` + `terminal`)

## Add To ~/.tmux.conf

```tmux
set -g status-interval 5
set -g status-right-length 140
set -g status-right "#{?client_prefix,PREFIX ,}#(/data/data/com.termux/files/home/files/status-tmux/widget-cpu) | #(/data/data/com.termux/files/home/files/status-tmux/widget-ram) | #(/data/data/com.termux/files/home/files/status-tmux/widget-weather) | #(/data/data/com.termux/files/home/files/status-tmux/widget-apps) "

bind -n M-Enter run-shell '/data/data/com.termux/files/home/files/status-tmux/open-apps-popup'
bind -n MouseDown1StatusRight if-shell -F '#{==:#{mouse_status_range},launch}' "run-shell '/data/data/com.termux/files/home/files/status-tmux/open-apps-popup'" ''
```

Reload:

```sh
tmux source-file ~/.tmux.conf
```

## Apps Config

Optional custom file (default path):

`~/.config/status-tmux/apps-menu.conf`

Format:

```text
type|label|arg1|arg2|pin
```

- `android`: `arg1=package/activity`, `arg2` ignored
- `terminal`: `arg1=command`, `arg2=tmux window name` (optional)
- `pin`: optional `1..9`

Start from:

```sh
mkdir -p ~/.config/status-tmux
cp ~/files/status-tmux/apps-menu.conf.example ~/.config/status-tmux/apps-menu.conf
```

## Tmux Options (optional)

```tmux
set -g @status-tmux-apps-label 'Apps'
set -g @status-tmux-apps-popup-width '85%'
set -g @status-tmux-apps-popup-height '70%'
set -g @status-tmux-apps-popup-title 'Omni Apps'

# CSV list used for auto-pin matching (package, label, or android:/terminal: prefixes)
set -g @status-tmux-apps-pinned 'com.android.chrome,com.termux,terminal:codex'

# Optional alternate config path
set -g @status-tmux-apps-menu-file '/absolute/path/apps-menu.conf'
```

## Dependencies

Required:
- `tmux`
- `curl`
- `fzf`

Recommended:
- `tooie` (Shizuku-backed launch/data)
- `jq` (better Tooie app-label parsing)
- `termux-api` (`termux-battery-status`) for battery fallback

## Notes

- If `~/.apps` / `~/.app_names` are missing, `search-apps` rebuilds launcher cache using `pm/cmd query-activities` with `MAIN+LAUNCHER`.
- Terminal entries come from your config file; Android entries come from launcher cache (and are relabeled from `tooie apps` when available).
