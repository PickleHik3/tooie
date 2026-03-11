# Tooie

This is a companion bootstrap script + TUI I vibe-coded for my Termux launcher for Android: https://github.com/PickleHik3/termux-launcher. As such, I do not expect it to work in other Termux setups.

What it does:
- Installs necessary packages (`tmux`, `fish`, `starship`, `zoxide`, `eza`, `go`, `matugen`, etc.)
- Copies over config files to their respective locations (~/.config & ~/.termux)
- Builds the 'tooie' binary and places it at ~/.local/bin/tooie

## Usage Notes

Press keybind "prefix + i" to bring up quick reference to the tmux keybinds. The prefix is "Ctrl + b" and "Ctrl + Space".

The TUI is split into 2 pages:
- First page includes the clock, live system stats and an android app launcher.
  - Pressing `/` brings up app search. From there you can press `Ctrl+p` to pin the highlighted app to the home screen.
  - Pinned apps can be launched directly by pressing their respective number keys `1-7`.
- Second page includes color scheme controls. It uses `matugen` to generate color palettes from the terminal background situated at `~/.termux/background/`. Optionally, you can override extracted colors and choose adaptive or curated profiles (Catppuccin/OneDark/TokyoNight/Gruvbox/Dracula/Nord), as well as fixed preset themes.

## Screenshots

<table>
  <tr>
    <td><img src="docs/screenshots/Screenshot_20260307-212609.png" alt="Tooie screenshot 1"></td>
    <td><img src="docs/screenshots/Screenshot_20260307-211944.png" alt="Tooie screenshot 2"></td>
  </tr>
  <tr>
    <td><img src="docs/screenshots/Screenshot_20260307-212006.png" alt="Tooie screenshot 3"></td>
    <td><img src="docs/screenshots/Screenshot_20260307-211957.png" alt="Tooie screenshot 4"></td>
  </tr>
  <tr>
    <td colspan="2" align="center"><img src="docs/screenshots/readme-bottom-guide.png" alt="Tooie bottom screenshot"></td>
  </tr>
</table>

### `tooie --clock --cal`
![tooie --clock --cal](docs/screenshots/Screenshot_20260309-073307.png)

### `tooie --clock`
![tooie --clock](docs/screenshots/Screenshot_20260309-073330.png)

### `tooie --cal`
![tooie --cal](docs/screenshots/Screenshot_20260309-073344.png)

## Install

```sh
pkg update -y
pkg i -y git
termux-setup-storage
git clone https://github.com/PickleHik3/tooie
cd ~/tooie
./install.sh
chsh -s fish
~/.local/bin/tooie --restart
```

## Run

```sh
~/.local/bin/tooie
```

## CLI

```sh
tooie --help
tooie --clock
tooie --cal
tooie --clock --cal
tooie apps
tooie apps --refresh
tooie theme compute --theme-source preset --preset-family catppuccin --preset-variant mocha
tooie theme apply --theme-source wallpaper --mode auto --status-palette vibrant
```

## Installed Paths

The installer places files here:

- binary: `~/.local/bin/tooie`
- Tooie helper scripts and state files: `~/.config/tooie/`
- app cache: `~/.cache/tooie/apps.json`
- icon cache: `~/.cache/tooie/icons/`
- pinned apps: `~/.config/tooie/pinned-apps.json`
- theme backups: `~/.config/tooie/backups/`
- installer safety backups: `~/.local/state/tooie/backups/<timestamp>/`

## What `install.sh` Deploys

- `~/.tmux.conf`
- `~/.termux/termux.properties`
- `~/.termux/colors.properties`
- `~/.termux/font.ttf`
- `~/.termux/font-italic.ttf`
- `~/.termux/bin/`
- `~/.config/starship.toml`
- `~/.config/fish/config.fish`
- `~/.config/peaclock/config`
- `~/.config/tmux/`
- `~/.config/tooie/apply-material.sh`
- `~/.config/tooie/restore-material.sh`
- `~/.config/tooie/list-material-backups.sh`

It supports both `pkg` and `pacman`.

## CLI Notes

- `tooie apps` caches launcher app discovery in `~/.cache/tooie/apps.json`.
- `tooie theme apply` is the runtime theme engine for Termux, tmux, peaclock, and starship.
- `~/.config/tooie/apply-material.sh` is a compatibility shim that forwards to `tooie theme apply`.
- `tooie --clock` starts the low-CPU standalone clock widget.
- `tooie --cal` starts the low-CPU standalone date/month calendar widget.
- `tooie --clock --cal` starts the side-by-side clock + calendar widget view.

## Uninstall

```sh
rm ~/.local/bin/tooie
rm -rf ~/.config/tooie
```
or
```sh
cd ~/tooie
./uninstall.sh
```
The script removes only the installed binary at `~/.local/bin/tooie`. Configs, helper scripts, and backups are left in place.

## Acknowledgements

- Clock font work in Tooie was created with `bit` by superstarryeyes:
  https://github.com/superstarryeyes/bit
- Uses JetBrainsMono NF:
  https://github.com/JetBrains/JetBrainsMono
