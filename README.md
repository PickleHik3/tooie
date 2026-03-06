# Tooie

Unified Termux Tooie manager for:
- Termux colors/fonts/properties
- tmux + status widgets
- peaclock
- starship + fish defaults
- LazyVim baseline config

## Install

```sh
cd ~/files/tooie
./install.sh
```

## Run

```sh
~/files/theme/tooie
# or
~/.local/bin/tooie
```

## Backend Commands

```sh
tooie apps
tooie apps --refresh
tooie launch com.termux
tooie launch com.termux/.app.TermuxActivity
tooie exec "am start -n com.termux/.app.TermuxActivity --user 0"
tooie icon com.termux
tooie icons refresh --pinned
```

Notes:
- `tooie apps` caches launcher app discovery in `~/.cache/tooie/apps.json`.
- `tooie icon <package>` caches backend-delivered icons in `~/.cache/tooie/icons/` when an icon endpoint is available.
- `tooie icons refresh --pinned` refreshes pinned-app icons, using backend icon routes first and internet dashboard-icons mappings second.
- Discovery uses local launcher activity queries first, then merges labels from the Tooie/Shizuku endpoint when configured.
- `tooie launch` prefers the Tooie/Shizuku `/v1/exec` endpoint, then falls back to local `am start`.

## Included install targets

`install.sh` installs/configures:
- `matugen` (cargo)
- `tmux`
- `neovim` nightly attempt via `bob`, with packaged `neovim` fallback
- `peaclock`
- `starship`
- `fish`

It supports both `pkg` and `pacman` based setups.

## Safety

Before replacing files, installer stores backups in:

`~/.local/state/tooie/backups/<timestamp>/`

## Next planned integration

- LazyVim theme patching page
- Starship theme patching page
- Launcher integration (`~/files/launcher`) as another TUI page
