# tooie-theme-manager

Unified Termux theme manager for:
- Termux colors/fonts/properties
- tmux + status widgets
- peaclock
- starship + fish defaults
- LazyVim baseline config

## Install

```sh
cd ~/files/tooie-theme-manager
./install.sh
```

## Run

```sh
~/files/theme/tooie-theme-manager
# or
~/.local/bin/tooie-theme-manager
```

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

`~/.local/state/tooie-theme-manager/backups/<timestamp>/`

## Next planned integration

- LazyVim theme patching page
- Starship theme patching page
- Launcher integration (`~/files/launcher`) as another TUI page
