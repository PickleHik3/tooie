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
