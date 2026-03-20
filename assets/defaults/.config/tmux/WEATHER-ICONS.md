# Weather Icons Index

This setup uses Nerd Fonts weather glyphs (`weather-*`) for the tmux weather widget.

## Build/Refresh Full Index

```sh
~/.config/tmux/weather-icons-index --refresh > ~/.cache/status-tmux/weather-icons-index.json
```

## Render Human-Readable Table

```sh
~/.config/tmux/weather-icons-index --markdown > ~/.cache/status-tmux/weather-icons-index.md
```

The generated JSON includes every `weather-*` icon currently published by Nerd Fonts in `glyphnames.json`.
