# Repository Guidelines

## Project Structure & Module Organization
This repository is a Termux-focused Go CLI plus deployment assets.

- `cmd/tooie/main.go`: main Bubble Tea TUI application entrypoint.
- `scripts/`: helper scripts copied into `~/.config/tooie` (`apply-material.sh`, `restore-material.sh`, `list-material-backups.sh`).
- `assets/defaults/`: baseline configs deployed by installer (`.termux`, `tmux`, `nvim`, `fish`, `starship`, `peaclock`).
- `fonts/`: clock glyph sets (`0-9.txt`, `colon.txt`) grouped by font directory.
- Root scripts: `install.sh`, `update.sh`, `uninstall.sh`.

## Build, Test, and Development Commands
- `./install.sh`: installs dependencies, deploys assets, builds `tooie`, and creates backups under `~/.local/state/tooie/backups/<timestamp>/`.
- `go build -o ./tooie ./cmd/tooie`: local build of the CLI binary.
- `./update.sh`: fast-forward pull (if git is available) then reruns install.
- `./uninstall.sh`: removes installed binaries from `~/.local/bin/tooie` and `~/.config/tooie/tooie`.
- `go test ./...`: run Go tests across all packages.

## Coding Style & Naming Conventions
- Go code must be `gofmt`-clean; keep imports grouped by standard library then external packages.
- Use Go naming conventions: `camelCase` for unexported identifiers, `PascalCase` for exported types/functions.
- Shell scripts should stay POSIX `sh` compatible, start with `set -eu`, and use lowercase kebab-case file names.
- Keep new font/theme asset names lowercase and directory-based (match existing `fonts/<name>/` layout).

## Testing Guidelines
- Add tests as `*_test.go` in the same package as the code under test.
- Prefer table-driven tests for parsing, mapping, and formatting logic.
- For UI changes, include a manual smoke check: build and launch `./tooie` (or `~/.config/tooie/tooie`) and verify navigation/actions.
- There is currently limited automated coverage, so PRs should include explicit test commands run.

## Commit & Pull Request Guidelines
- Follow the existing pattern: concise, imperative commit subjects, usually Conventional Commit style (example: `feat: refine TUI layout`).
- Keep commits scoped to one concern (TUI logic, installer behavior, assets, or fonts).
- PRs should include:
  - What changed and why.
  - Commands/tests executed (for example, `go build`, `go test ./...`).
  - Screenshots or short recordings for visible TUI changes.
  - Notes on any files written outside the repo (Termux home paths, backups, deployed configs).
