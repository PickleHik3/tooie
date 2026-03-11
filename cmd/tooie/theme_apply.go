package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	itheme "github.com/PickleHik3/tooie/internal/theme"
)

const (
	themeBackupKeep = 5
)

type themeApplyConfig struct {
	Mode          string
	SchemeType    string
	ThemeSource   string
	PresetFamily  string
	PresetVariant string
	MatugenBin    string
	StatusPalette string
	StylePreset   string
	TextColor     string
	CursorColor   string
	PreviewOnly   bool
	ReuseBackupID string
	AnsiRed       string
	AnsiGreen     string
	AnsiYellow    string
	AnsiBlue      string
	AnsiMagenta   string
	AnsiCyan      string
}

func runThemeCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "tooie theme: expected subcommand: apply|compute")
		return 2
	}
	switch strings.TrimSpace(args[0]) {
	case "apply":
		return runThemeApplyCommand(args[1:])
	case "compute":
		return runThemeComputeCommand(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "tooie theme: unknown subcommand %q\n", args[0])
		return 2
	}
}

func runThemeComputeCommand(args []string) int {
	cfg, err := parseThemeApplyFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme compute: %v\n", err)
		return 2
	}
	payload, _, err := computeThemePayload(cfg, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme compute: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme compute: %v\n", err)
		return 1
	}
	return 0
}

func runThemeApplyCommand(args []string) int {
	cfg, err := parseThemeApplyFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
		return 2
	}
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
		return 1
	}

	stamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(backupRoot, stamp)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
		return 1
	}
	_ = writeApplyProgress("Preparing theme", 0.05)

	payload, matugenRaw, err := computeThemePayload(cfg, backupDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
		return 1
	}

	if len(matugenRaw) > 0 {
		if err := os.WriteFile(filepath.Join(backupDir, "matugen.json"), matugenRaw, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
			return 1
		}
	}

	meta := map[string]string{}
	for k, v := range payload.Meta {
		meta[k] = v
	}
	meta["backup_id"] = stamp
	meta["theme_source"] = cfg.ThemeSource
	meta["mode"] = cfg.Mode
	meta["effective_mode"] = payload.EffectiveMode
	meta["type"] = cfg.SchemeType
	meta["matugen_bin"] = cfg.MatugenBin
	meta["text_color_override"] = strings.TrimSpace(cfg.TextColor)
	meta["cursor_color_override"] = strings.TrimSpace(cfg.CursorColor)
	meta["status_palette"] = cfg.StatusPalette
	meta["style_preset"] = canonicalStylePreset(cfg.StylePreset)
	meta["ansi_red_override"] = strings.TrimSpace(cfg.AnsiRed)
	meta["ansi_green_override"] = strings.TrimSpace(cfg.AnsiGreen)
	meta["ansi_yellow_override"] = strings.TrimSpace(cfg.AnsiYellow)
	meta["ansi_blue_override"] = strings.TrimSpace(cfg.AnsiBlue)
	meta["ansi_magenta_override"] = strings.TrimSpace(cfg.AnsiMagenta)
	meta["ansi_cyan_override"] = strings.TrimSpace(cfg.AnsiCyan)
	meta["preview_only"] = ternBool(cfg.PreviewOnly)
	if cfg.ThemeSource == "preset" {
		meta["preset_family"] = cfg.PresetFamily
		meta["preset_variant"] = cfg.PresetVariant
	}
	if strings.TrimSpace(payload.Wallpaper) != "" {
		meta["wallpaper"] = payload.Wallpaper
	}
	if strings.TrimSpace(cfg.ReuseBackupID) != "" {
		meta["reused_preview_id"] = strings.TrimSpace(cfg.ReuseBackupID)
	}

	if err := writeMetaEnv(filepath.Join(backupDir, "meta.env"), meta); err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
		return 1
	}

	if cfg.PreviewOnly {
		_ = pruneOldBackups(backupRoot, themeBackupKeep)
		_ = writeApplyProgress("Preview ready", 1.0)
		fmt.Printf("Preview created: %s\n", backupDir)
		return 0
	}

	if err := applyThemeFiles(payload, backupDir); err != nil {
		fmt.Fprintf(os.Stderr, "tooie theme apply: %v\n", err)
		return 1
	}

	_ = pruneOldBackups(backupRoot, themeBackupKeep)
	_ = writeApplyProgress("Finishing theme apply", 1.0)
	fmt.Println("Applied Material theme.")
	fmt.Printf("Backup created: %s\n", backupDir)
	fmt.Println("Shell theme files updated. Reload your session if needed.")
	return 0
}

type computedPayload struct {
	EffectiveMode string            `json:"effective_mode"`
	Wallpaper     string            `json:"wallpaper,omitempty"`
	Roles         map[string]string `json:"roles"`
	Foreground    string            `json:"foreground"`
	Background    string            `json:"background"`
	Cursor        string            `json:"cursor"`
	Colors        map[int]string    `json:"colors"`
	Meta          map[string]string `json:"meta"`
	Status        struct {
		Separator string   `json:"separator"`
		Weather   string   `json:"weather"`
		Charging  string   `json:"charging"`
		Battery   []string `json:"battery"`
		CPU       []string `json:"cpu"`
		RAM       []string `json:"ram"`
	} `json:"status"`
}

func computeThemePayload(cfg themeApplyConfig, workDir string) (computedPayload, []byte, error) {
	var out computedPayload
	var roles map[string]string
	var matugenRaw []byte
	effectiveMode := canonicalMode(cfg.Mode)
	if cfg.ThemeSource == "preset" {
		presetRoles, mode, err := itheme.BuildPresetRoles(cfg.PresetFamily, cfg.PresetVariant)
		if err != nil {
			return out, nil, err
		}
		roles = presetRoles
		effectiveMode = mode
		matugenRaw, _ = itheme.BuildRolesJSON(roles)
	} else {
		wallpaper, err := resolveWallpaperPath()
		if err != nil {
			return out, nil, err
		}
		out.Wallpaper = wallpaper
		matugenBin, err := resolveMatugen(cfg.MatugenBin)
		if err != nil {
			return out, nil, err
		}
		cfg.MatugenBin = matugenBin

		if cfg.ReuseBackupID != "" {
			reusePath := filepath.Join(backupRoot, cfg.ReuseBackupID, "matugen.json")
			if raw, err := os.ReadFile(reusePath); err == nil && len(raw) > 0 {
				matugenRaw = raw
			}
		}
		if len(matugenRaw) == 0 {
			raw, mode, err := generateMatugenJSON(cfg, wallpaper, workDir)
			if err != nil {
				return out, nil, err
			}
			matugenRaw = raw
			effectiveMode = mode
		}
		parsed, err := itheme.ParseMatugenColors(matugenRaw)
		if err != nil {
			return out, nil, err
		}
		roles = parsed
	}

	ansiOverrides := map[string]string{
		"red":     cfg.AnsiRed,
		"green":   cfg.AnsiGreen,
		"yellow":  cfg.AnsiYellow,
		"blue":    cfg.AnsiBlue,
		"magenta": cfg.AnsiMagenta,
		"cyan":    cfg.AnsiCyan,
	}
	computed, err := itheme.Compute(roles, itheme.Options{
		Source:         itheme.Source(cfg.ThemeSource),
		Mode:           effectiveMode,
		StylePreset:    cfg.StylePreset,
		StatusPalette:  cfg.StatusPalette,
		TextOverride:   cfg.TextColor,
		CursorOverride: cfg.CursorColor,
		AnsiOverrides:  ansiOverrides,
	})
	if err != nil {
		return out, nil, err
	}

	out.EffectiveMode = computed.EffectiveMode
	out.Roles = computed.Roles
	out.Foreground = computed.Foreground
	out.Background = computed.Background
	out.Cursor = computed.Cursor
	out.Colors = computed.Colors
	out.Meta = computed.Meta
	out.Meta["status_palette"] = cfg.StatusPalette
	out.Status.Separator = computed.Status.Separator
	out.Status.Weather = computed.Status.Weather
	out.Status.Charging = computed.Status.Charging
	out.Status.Battery = append([]string{}, computed.Status.Battery[:]...)
	out.Status.CPU = append([]string{}, computed.Status.CPU[:]...)
	out.Status.RAM = append([]string{}, computed.Status.RAM[:]...)
	return out, matugenRaw, nil
}

func generateMatugenJSON(cfg themeApplyConfig, wallpaper, workDir string) ([]byte, string, error) {
	_ = writeApplyProgress("Generating dynamic palette", 0.14)
	mode := canonicalMode(cfg.Mode)
	if mode == "auto" {
		darkRaw, err := runMatugenImage(cfg.MatugenBin, wallpaper, "dark", cfg.SchemeType)
		if err != nil {
			return nil, "", err
		}
		lightRaw, err := runMatugenImage(cfg.MatugenBin, wallpaper, "light", cfg.SchemeType)
		if err != nil {
			return nil, "", err
		}
		darkRoles, err := itheme.ParseMatugenColors(darkRaw)
		if err != nil {
			return nil, "", err
		}
		lightRoles, err := itheme.ParseMatugenColors(lightRaw)
		if err != nil {
			return nil, "", err
		}
		luma := wallpaperLuma(wallpaper)
		if luma >= 0 {
			if luma <= 0.42 {
				return darkRaw, "dark", nil
			}
			if luma >= 0.58 {
				return lightRaw, "light", nil
			}
		}
		ds := readabilityScore(darkRoles)
		ls := readabilityScore(lightRoles)
		if ds >= ls {
			return darkRaw, "dark", nil
		}
		return lightRaw, "light", nil
	}
	raw, err := runMatugenImage(cfg.MatugenBin, wallpaper, mode, cfg.SchemeType)
	if err != nil {
		return nil, "", err
	}
	return raw, mode, nil
}

func readabilityScore(roles map[string]string) float64 {
	bg := getRoleOr(roles, "background", "#1a1b26")
	bgDim := getRoleOr(roles, "surface_dim", blendHexColor(bg, "#000000", 0.08))
	bgBright := getRoleOr(roles, "surface_bright", blendHexColor(bg, "#ffffff", 0.08))
	fg := getRoleOr(roles, "on_background", "#c0caf5")
	p := getRoleOr(roles, "primary", "#7aa2f7")
	s := getRoleOr(roles, "secondary", "#7dcfff")
	t := getRoleOr(roles, "tertiary", "#bb9af7")
	e := getRoleOr(roles, "error", "#ff5f5f")
	fgMin := minContrastHex(fg, bg, bgDim, bgBright)
	accentMin := minFloat(minContrastHex(p, bg, bgDim, bgBright), minContrastHex(s, bg, bgDim, bgBright), minContrastHex(t, bg, bgDim, bgBright), minContrastHex(e, bg, bgDim, bgBright))
	return (fgMin * 2.0) + accentMin
}

func minContrastHex(fg string, bgs ...string) float64 {
	minv := 999.0
	for _, bg := range bgs {
		r := contrastRatioHex(fg, bg)
		if r < minv {
			minv = r
		}
	}
	if minv == 999.0 {
		return 0
	}
	return minv
}

func minFloat(v float64, more ...float64) float64 {
	m := v
	for _, x := range more {
		if x < m {
			m = x
		}
	}
	return m
}

func parseThemeApplyFlags(args []string) (themeApplyConfig, error) {
	cfg := themeApplyConfig{
		Mode:          "auto",
		SchemeType:    "scheme-tonal-spot",
		ThemeSource:   "wallpaper",
		PresetFamily:  "catppuccin",
		PresetVariant: "mocha",
		StatusPalette: "default",
		StylePreset:   "balanced",
	}
	fs := flag.NewFlagSet("theme apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Mode, "m", cfg.Mode, "")
	fs.StringVar(&cfg.Mode, "mode", cfg.Mode, "")
	fs.StringVar(&cfg.SchemeType, "t", cfg.SchemeType, "")
	fs.StringVar(&cfg.SchemeType, "type", cfg.SchemeType, "")
	fs.StringVar(&cfg.ThemeSource, "theme-source", cfg.ThemeSource, "")
	fs.StringVar(&cfg.PresetFamily, "preset-family", cfg.PresetFamily, "")
	fs.StringVar(&cfg.PresetVariant, "preset-variant", cfg.PresetVariant, "")
	fs.StringVar(&cfg.MatugenBin, "b", cfg.MatugenBin, "")
	fs.StringVar(&cfg.MatugenBin, "matugen-bin", cfg.MatugenBin, "")
	fs.StringVar(&cfg.TextColor, "text-color", "", "")
	fs.StringVar(&cfg.CursorColor, "cursor-color", "", "")
	fs.StringVar(&cfg.StatusPalette, "status-palette", cfg.StatusPalette, "")
	fs.StringVar(&cfg.StylePreset, "style-preset", cfg.StylePreset, "")
	fs.BoolVar(&cfg.PreviewOnly, "preview-only", false, "")
	fs.StringVar(&cfg.ReuseBackupID, "reuse-backup", "", "")
	fs.StringVar(&cfg.AnsiRed, "ansi-red", "", "")
	fs.StringVar(&cfg.AnsiGreen, "ansi-green", "", "")
	fs.StringVar(&cfg.AnsiYellow, "ansi-yellow", "", "")
	fs.StringVar(&cfg.AnsiBlue, "ansi-blue", "", "")
	fs.StringVar(&cfg.AnsiMagenta, "ansi-magenta", "", "")
	fs.StringVar(&cfg.AnsiCyan, "ansi-cyan", "", "")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	cfg.Mode = canonicalMode(cfg.Mode)
	cfg.ThemeSource = strings.TrimSpace(strings.ToLower(cfg.ThemeSource))
	if cfg.ThemeSource != "wallpaper" && cfg.ThemeSource != "preset" {
		return cfg, fmt.Errorf("invalid theme source: %s", cfg.ThemeSource)
	}
	cfg.StatusPalette = strings.TrimSpace(strings.ToLower(cfg.StatusPalette))
	if cfg.StatusPalette != "default" && cfg.StatusPalette != "vibrant" {
		return cfg, fmt.Errorf("invalid status palette: %s", cfg.StatusPalette)
	}
	cfg.StylePreset = canonicalStylePreset(cfg.StylePreset)
	for _, item := range []struct {
		v    string
		name string
	}{
		{cfg.TextColor, "--text-color"},
		{cfg.CursorColor, "--cursor-color"},
		{cfg.AnsiRed, "--ansi-red"},
		{cfg.AnsiGreen, "--ansi-green"},
		{cfg.AnsiYellow, "--ansi-yellow"},
		{cfg.AnsiBlue, "--ansi-blue"},
		{cfg.AnsiMagenta, "--ansi-magenta"},
		{cfg.AnsiCyan, "--ansi-cyan"},
	} {
		if strings.TrimSpace(item.v) != "" && !itheme.IsHexColor(item.v) {
			return cfg, fmt.Errorf("invalid %s value: %s (expected #rrggbb)", item.name, item.v)
		}
	}
	return cfg, nil
}

func applyThemeFiles(payload computedPayload, backupDir string) error {
	termuxColors := filepath.Join(homeDir, ".termux", "colors.properties")
	tmuxConf := filepath.Join(homeDir, ".tmux.conf")
	peaclockCfg := filepath.Join(homeDir, ".config", "peaclock", "config")
	starshipCfg := filepath.Join(homeDir, ".config", "starship.toml")

	if err := os.MkdirAll(filepath.Dir(termuxColors), 0o755); err != nil {
		return err
	}
	if raw, err := os.ReadFile(termuxColors); err == nil {
		_ = os.WriteFile(filepath.Join(backupDir, "colors.properties.bak"), raw, 0o644)
	}
	_ = writeApplyProgress("Writing Termux colors", 0.42)
	if err := os.WriteFile(termuxColors, []byte(renderColorsProperties(payload)), 0o644); err != nil {
		return err
	}

	_ = writeApplyProgress("Writing tmux theme", 0.56)
	if err := ensureFileWithDirs(tmuxConf); err != nil {
		return err
	}
	if err := backupIfExists(tmuxConf, filepath.Join(backupDir, "tmux.conf.bak")); err != nil {
		return err
	}
	if err := replaceBlock(tmuxConf, "# >>> MATUGEN THEME START >>>", "# <<< MATUGEN THEME END <<<", renderTmuxBlock(payload)); err != nil {
		return err
	}

	_ = writeApplyProgress("Writing peaclock theme", 0.68)
	if err := ensureFileWithDirs(peaclockCfg); err != nil {
		return err
	}
	if err := backupIfExists(peaclockCfg, filepath.Join(backupDir, "peaclock.config.bak")); err != nil {
		return err
	}
	if err := replaceBlock(peaclockCfg, "# >>> MATUGEN PEACLOCK START >>>", "# <<< MATUGEN PEACLOCK END <<<", renderPeaclockBlock(payload)); err != nil {
		return err
	}

	_ = writeApplyProgress("Writing starship theme", 0.78)
	if err := ensureFileWithDirs(starshipCfg); err != nil {
		return err
	}
	if err := backupIfExists(starshipCfg, filepath.Join(backupDir, "starship.toml.bak")); err != nil {
		return err
	}
	if err := applyStarshipTheme(starshipCfg, payload); err != nil {
		return err
	}

	metaPath := filepath.Join(backupDir, "meta.env")
	f, err := os.OpenFile(metaPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		_, _ = f.WriteString("peaclock_themed=true\nstarship_themed=true\n")
		_ = f.Close()
	}

	_ = writeApplyProgress("Reloading shell surfaces", 0.94)
	if _, err := exec.LookPath("termux-reload-settings"); err == nil {
		_ = exec.Command("termux-reload-settings").Run()
	}
	if _, err := exec.LookPath("tmux"); err == nil {
		_ = exec.Command("tmux", "source-file", tmuxConf).Run()
	}
	return nil
}

func renderColorsProperties(payload computedPayload) string {
	c := payload.Colors
	return fmt.Sprintf(`# Generated by %s/theme apply
foreground=%s
background=%s
cursor=%s

color0=%s
color1=%s
color2=%s
color3=%s
color4=%s
color5=%s
color6=%s
color7=%s

color8=%s
color9=%s
color10=%s
color11=%s
color12=%s
color13=%s
color14=%s
color15=%s

color16=%s
color17=%s
color18=%s
color19=%s
color20=%s
color21=%s
`, tooieConfigDir, payload.Foreground, payload.Background, payload.Cursor, c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7], c[8], c[9], c[10], c[11], c[12], c[13], c[14], c[15], c[16], c[17], c[18], c[19], c[20], c[21])
}

func renderTmuxBlock(payload computedPayload) string {
	c := payload.Colors
	statusLeftFG := ensureReadableTextColor(c[4], payload.Foreground, payload.Background)
	modeFG := ensureReadableTextColor(c[2], payload.Background, payload.Foreground)
	statusPalette := strings.TrimSpace(payload.Meta["status_palette"])
	if statusPalette == "" {
		statusPalette = "default"
	}
	return fmt.Sprintf(`# >>> MATUGEN THEME START >>>
# Generated by %s/theme apply
set -g status-style "bg=%s,fg=%s"
set -g status-left "#[fg=%s,bg=%s,bold] #S #[bg=%s,fg=%s] "
set -g status-right "#{?client_prefix,PREFIX ,}#(\$HOME/.config/tmux/widget-battery) | #(\$HOME/.config/tmux/widget-cpu) | #(\$HOME/.config/tmux/widget-ram) | #(\$HOME/.config/tmux/widget-weather) "
set -g window-status-format "#[fg=%s] #I:#W "
set -g window-status-current-format "#[fg=%s,bold] #I:#W "
set -g pane-border-style "fg=%s"
set -g pane-active-border-style "fg=%s"
set -g message-style "bg=%s,fg=%s"
set -g message-command-style "bg=%s,fg=%s"
set -g mode-style "bg=%s,fg=%s"
setw -g clock-mode-colour "%s"
set -g @status-tmux-palette "%s"
set -g @status-tmux-color-separator "%s"
set -g @status-tmux-color-weather "%s"
set -g @status-tmux-color-charging "%s"
set -g @status-tmux-color-battery-1 "%s"
set -g @status-tmux-color-battery-2 "%s"
set -g @status-tmux-color-battery-3 "%s"
set -g @status-tmux-color-battery-4 "%s"
set -g @status-tmux-color-battery-5 "%s"
set -g @status-tmux-color-battery-6 "%s"
set -g @status-tmux-color-cpu-1 "%s"
set -g @status-tmux-color-cpu-2 "%s"
set -g @status-tmux-color-cpu-3 "%s"
set -g @status-tmux-color-cpu-4 "%s"
set -g @status-tmux-color-cpu-5 "%s"
set -g @status-tmux-color-cpu-6 "%s"
set -g @status-tmux-color-ram-1 "%s"
set -g @status-tmux-color-ram-2 "%s"
set -g @status-tmux-color-ram-3 "%s"
set -g @status-tmux-color-ram-4 "%s"
set -g @status-tmux-color-ram-5 "%s"
set -g @status-tmux-color-ram-6 "%s"
# <<< MATUGEN THEME END <<<
`, tooieConfigDir,
		payload.Background, payload.Foreground, statusLeftFG, payload.Roles["primary"], payload.Background, payload.Foreground,
		c[14], payload.Roles["secondary"], c[14], payload.Roles["secondary"], payload.Background, payload.Roles["secondary"], payload.Background, payload.Roles["secondary"], payload.Roles["secondary"], modeFG, payload.Roles["secondary"],
		statusPalette,
		payload.Status.Separator, payload.Status.Weather, payload.Status.Charging,
		payload.Status.Battery[0], payload.Status.Battery[1], payload.Status.Battery[2], payload.Status.Battery[3], payload.Status.Battery[4], payload.Status.Battery[5],
		payload.Status.CPU[0], payload.Status.CPU[1], payload.Status.CPU[2], payload.Status.CPU[3], payload.Status.CPU[4], payload.Status.CPU[5],
		payload.Status.RAM[0], payload.Status.RAM[1], payload.Status.RAM[2], payload.Status.RAM[3], payload.Status.RAM[4], payload.Status.RAM[5],
	)
}

func renderPeaclockBlock(payload computedPayload) string {
	c := payload.Colors
	return fmt.Sprintf(`# >>> MATUGEN PEACLOCK START >>>
# Generated by %s/theme apply
style inactive-fg %s
style active-bg %s
style active-fg clear
style colon-fg %s
style colon-bg clear
style date %s
style text %s
style prompt %s
style success %s
style error %s
# <<< MATUGEN PEACLOCK END <<<
`, tooieConfigDir, c[14], payload.Roles["primary"], payload.Roles["primary"], payload.Roles["tertiary"], c[15], payload.Roles["secondary"], c[10], payload.Roles["error"])
}

func applyStarshipTheme(path string, payload computedPayload) error {
	c := payload.Colors
	kv := []struct{ sec, key, val string }{
		{"character", "success_symbol", fmt.Sprintf("\"[◎](bold %s)\"", c[3])},
		{"character", "error_symbol", fmt.Sprintf("\"[○](bold %s)\"", c[1])},
		{"character", "vimcmd_symbol", fmt.Sprintf("\"[■](bold %s)\"", c[2])},
		{"directory", "style", fmt.Sprintf("\"italic %s\"", c[4])},
		{"directory", "repo_root_style", fmt.Sprintf("\"bold %s\"", c[2])},
		{"cmd_duration", "format", fmt.Sprintf("\"[◄ $duration ](italic %s)\"", c[15])},
		{"git_branch", "symbol", fmt.Sprintf("\"[△](bold italic %s)\"", c[4])},
		{"git_branch", "style", fmt.Sprintf("\"italic %s\"", c[4])},
		{"git_status", "style", fmt.Sprintf("\"bold italic %s\"", c[2])},
		{"time", "style", fmt.Sprintf("\"italic %s\"", c[14])},
		{"username", "style_user", fmt.Sprintf("\"%s bold italic\"", c[3])},
		{"username", "style_root", fmt.Sprintf("\"%s bold italic\"", c[1])},
		{"sudo", "style", fmt.Sprintf("\"bold italic %s\"", c[5])},
		{"jobs", "style", fmt.Sprintf("\"%s\"", c[15])},
		{"jobs", "symbol", fmt.Sprintf("\"[▶](%s italic)\"", c[4])},
	}
	for _, item := range kv {
		if err := tomlUpsert(path, item.sec, item.key, item.val); err != nil {
			return err
		}
	}
	return nil
}

func tomlUpsert(path, section, key, value string) error {
	raw, _ := os.ReadFile(path)
	lines := strings.Split(string(raw), "\n")
	secHdr := "[" + section + "]"
	secStart := -1
	secEnd := len(lines)
	for i, ln := range lines {
		if strings.TrimSpace(ln) == secHdr {
			secStart = i
			continue
		}
		if secStart >= 0 && strings.HasPrefix(strings.TrimSpace(ln), "[") && strings.HasSuffix(strings.TrimSpace(ln), "]") {
			secEnd = i
			break
		}
	}
	if secStart < 0 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, secHdr, fmt.Sprintf("%s = %s", key, value))
		return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
	replaced := false
	for i := secStart + 1; i < secEnd; i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, key+" ") || strings.HasPrefix(trim, key+"=") {
			lines[i] = fmt.Sprintf("%s = %s", key, value)
			replaced = true
			break
		}
	}
	if !replaced {
		insertAt := secEnd
		lines = append(lines[:insertAt], append([]string{fmt.Sprintf("%s = %s", key, value)}, lines[insertAt:]...)...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func replaceBlock(path, startMarker, endMarker, block string) error {
	raw, _ := os.ReadFile(path)
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines)+16)
	skip := false
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == strings.TrimSpace(startMarker) {
			skip = true
			continue
		}
		if trim == strings.TrimSpace(endMarker) {
			skip = false
			continue
		}
		if !skip {
			out = append(out, ln)
		}
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	out = append(out, "", strings.TrimRight(block, "\n"), "")
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

func writeApplyProgress(label string, progress float64) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	state := applyProgressState{Label: label, Progress: progress}
	raw, _ := json.Marshal(state)
	if err := os.MkdirAll(tooieConfigDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(applyProgressPath(), raw, 0o644)
}

func ensureFileWithDirs(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(""), 0o644)
}

func backupIfExists(srcPath, backupPath string) error {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(backupPath, raw, 0o644)
}

func writeMetaEnv(path string, meta map[string]string) error {
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strings.TrimSpace(meta[k]))
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func pruneOldBackups(root string, keep int) error {
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	type di struct {
		path string
		mod  time.Time
	}
	items := []di{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, di{path: filepath.Join(root, e.Name()), mod: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	for i := keep; i < len(items); i++ {
		_ = os.RemoveAll(items[i].path)
	}
	return nil
}

func resolveWallpaperPath() (string, error) {
	if _, err := os.Stat(defaultWall); err == nil {
		return defaultWall, nil
	}
	bgDir := filepath.Join(homeDir, ".termux", "background")
	ents, err := os.ReadDir(bgDir)
	if err != nil {
		return "", fmt.Errorf("wallpaper not found at %s", defaultWall)
	}
	type fi struct {
		name string
		mod  time.Time
	}
	items := []fi{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, fi{name: e.Name(), mod: info.ModTime()})
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no wallpapers found in %s", bgDir)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	return filepath.Join(bgDir, items[0].name), nil
}

func resolveMatugen(given string) (string, error) {
	if strings.TrimSpace(given) != "" {
		if st, err := os.Stat(given); err == nil && st.Mode()&0o111 != 0 {
			return given, nil
		}
	}
	for _, cand := range []string{filepath.Join(homeDir, "cargo", "bin", "matugen"), filepath.Join(homeDir, ".cargo", "bin", "matugen")} {
		if st, err := os.Stat(cand); err == nil && st.Mode()&0o111 != 0 {
			return cand, nil
		}
	}
	if p, err := exec.LookPath("matugen"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("matugen binary not found. Set --matugen-bin or install matugen")
}

func runMatugenImage(bin, wallpaper, mode, schemeType string) ([]byte, error) {
	args := []string{"image", wallpaper, "-m", mode, "-t", schemeType, "--source-color-index", "0", "-j", "hex", "--dry-run"}
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("matugen failed for mode=%s: %v (%s)", mode, err, strings.TrimSpace(string(out)))
	}
	return bytes.TrimSpace(out), nil
}

func wallpaperLuma(path string) float64 {
	for _, tool := range [][]string{{"magick", path, "-colorspace", "Gray", "-resize", "1x1!", "-format", "%[fx:intensity]", "info:"}, {"convert", path, "-colorspace", "Gray", "-resize", "1x1!", "-format", "%[fx:intensity]", "info:"}} {
		if _, err := exec.LookPath(tool[0]); err != nil {
			continue
		}
		out, err := exec.Command(tool[0], tool[1:]...).CombinedOutput()
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(out))
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return -1
}

func getRoleOr(m map[string]string, key, fallback string) string {
	if v := strings.TrimSpace(m[key]); v != "" {
		return normalizeHexColor(v)
	}
	return normalizeHexColor(fallback)
}

func ternBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
