package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testThemePayload() computedPayload {
	p := computedPayload{}
	p.Foreground = "#e4e3d7"
	p.Background = "#13140d"
	p.Cursor = "#c0ce7e"
	p.Roles = map[string]string{
		"primary":   "#c5c9a8",
		"secondary": "#c0ce7e",
		"tertiary":  "#a1d0c4",
		"error":     "#ffb4ab",
	}
	p.Meta = map[string]string{"status_palette": "vibrant", "status_theme": "default"}
	p.Colors = map[int]string{}
	for i := 0; i <= 21; i++ {
		p.Colors[i] = "#111111"
	}
	p.Colors[1] = "#ffb4ab"
	p.Colors[2] = "#c0ce7e"
	p.Colors[3] = "#a1d0c4"
	p.Colors[4] = "#c5c9a8"
	p.Colors[5] = "#414c08"
	p.Colors[6] = "#214e45"
	p.Colors[7] = "#e4e3d7"
	p.Colors[10] = "#414c08"
	p.Colors[14] = "#919283"
	p.Colors[15] = "#c7c7b7"
	p.Status.Separator = "#919283"
	p.Status.Weather = "#c0ce7e"
	p.Status.Charging = "#c0ce7e"
	p.Status.Battery = []string{"#ffb4ab", "#a1d0c4", "#214e45", "#c5c9a8", "#45492f", "#c0ce7e"}
	p.Status.CPU = []string{"#c0ce7e", "#414c08", "#a1d0c4", "#214e45", "#93000a", "#ffb4ab"}
	p.Status.RAM = []string{"#214e45", "#45492f", "#586420", "#a1d0c4", "#93000a", "#ffb4ab"}
	return p
}

func TestRenderTmuxBlockIncludesPaletteKey(t *testing.T) {
	got := renderTmuxBlock(testThemePayload())
	if !strings.Contains(got, `set -g @status-tmux-palette "vibrant"`) {
		t.Fatalf("renderTmuxBlock() missing @status-tmux-palette: %s", got)
	}
}

func TestRenderTmuxBlockDefaultTheme(t *testing.T) {
	got := renderTmuxBlock(testThemePayload())
	if !strings.Contains(got, `set -g status-style "bg=default,fg=`) {
		t.Fatalf("expected transparent status-style, got: %s", got)
	}
	if !strings.Contains(got, `set -g @status-tmux-edge-style "rounded"`) {
		t.Fatalf("expected default theme to keep rounded widget edges, got: %s", got)
	}
	if !strings.Contains(got, `set -g status-left " #(\$HOME/.config/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}')`) {
		t.Fatalf("expected status-left to use the widget-left helper, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-separator ""`) {
		t.Fatalf("expected empty window separator, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-format "#[fg=`) || !strings.Contains(got, ",bg=") {
		t.Fatalf("expected default inactive window format to stay rectangular, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-current-format "#[fg=`) || !strings.Contains(got, ",bg=") {
		t.Fatalf("expected default active window format to stay rectangular, got: %s", got)
	}
	if !strings.Contains(got, `set -g mode-style "bg=`) || strings.Contains(got, `set -g mode-style "bg=default`) {
		t.Fatalf("expected copy mode to use explicit highlight background, got: %s", got)
	}
	if !strings.Contains(got, `set -g status-right "#(\$HOME/.config/tmux/run-system-widget all)#(\$HOME/.config/tmux/widget-weather)"`) {
		t.Fatalf("expected canonical status-right widget pipeline, got: %s", got)
	}
	if !strings.Contains(got, `set -g @status-tmux-widget-gap-right "space"`) {
		t.Fatalf("expected default theme to keep spaced right widgets, got: %s", got)
	}
	for _, key := range []string{
		`set -g @status-tmux-widget-battery "on"`,
		`set -g @status-tmux-widget-cpu "on"`,
		`set -g @status-tmux-widget-ram "on"`,
		`set -g @status-tmux-widget-weather "on"`,
	} {
		if !strings.Contains(got, key) {
			t.Fatalf("expected renderTmuxBlock() to include %s, got: %s", key, got)
		}
	}
	if !strings.Contains(got, `set -g window-status-format "#[fg=`) || !strings.Contains(got, `nounderscore`) {
		t.Fatalf("expected inactive windows to avoid emphasis styles, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-current-format "#[fg=`) || !strings.Contains(got, `bold,noitalics,nounderscore`) {
		t.Fatalf("expected active window to emphasize without underline, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-activity-style "fg=`) || !strings.Contains(got, `set -g window-status-bell-style "fg=`) {
		t.Fatalf("expected explicit bright styles for activity/bell windows, got: %s", got)
	}
	var paneBorderLine, paneActiveBorderLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, `set -g pane-border-style "fg=`) {
			paneBorderLine = line
		}
		if strings.HasPrefix(line, `set -g pane-active-border-style "fg=`) {
			paneActiveBorderLine = line
		}
	}
	if paneBorderLine == "" || paneActiveBorderLine == "" {
		t.Fatalf("expected pane border style lines, got: %s", got)
	}
	if paneBorderLine == strings.Replace(paneActiveBorderLine, "pane-active-border-style", "pane-border-style", 1) {
		t.Fatalf("expected active pane border color to differ from inactive border, got: %s", got)
	}
}

func TestRenderTmuxBlockRoundedTheme(t *testing.T) {
	payload := testThemePayload()
	payload.Meta["status_theme"] = "rounded"
	got := renderTmuxBlock(payload)
	if !strings.Contains(got, `set -g @status-tmux-edge-style "rounded"`) {
		t.Fatalf("expected rounded theme to keep rounded widget edges, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-format "#{?window_start_flag`) || !strings.Contains(got, ``) {
		t.Fatalf("expected rounded theme to use rounded inactive window chips, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-current-format "#{?window_start_flag`) || !strings.Contains(got, ``) {
		t.Fatalf("expected rounded theme to use rounded active window chips, got: %s", got)
	}
}

func TestRenderTmuxBlockRectangleTheme(t *testing.T) {
	payload := testThemePayload()
	payload.Meta["status_theme"] = "rectangle"
	got := renderTmuxBlock(payload)
	if !strings.Contains(got, `set -g @status-tmux-edge-style "flat"`) {
		t.Fatalf("expected rectangle theme to use flat widget edges, got: %s", got)
	}
	if !strings.Contains(got, `set -g status-left " #(\$HOME/.config/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}') "`) {
		t.Fatalf("expected rectangle theme to add a gap after the left widget, got: %s", got)
	}
	if !strings.Contains(got, `set -g @status-tmux-widget-gap-right "none"`) {
		t.Fatalf("expected rectangle theme to remove right widget gaps, got: %s", got)
	}
	if strings.Contains(got, `window-status-format "#[fg=`) && strings.Contains(got, ``) {
		t.Fatalf("expected rectangle theme windows to stay rectangular, got: %s", got)
	}
}

func TestApplyThemeFilesCreatesBackupsAndIdempotentBlocks(t *testing.T) {
	tmp := t.TempDir()
	oldHome, oldCfg := homeDir, tooieConfigDir
	homeDir = tmp
	tooieConfigDir = filepath.Join(tmp, ".config", "tooie")
	t.Cleanup(func() {
		homeDir = oldHome
		tooieConfigDir = oldCfg
	})

	termuxColors := filepath.Join(tmp, ".termux", "colors.properties")
	tmuxConf := filepath.Join(tmp, ".tmux.conf")
	peaclockCfg := filepath.Join(tmp, ".config", "peaclock", "config")
	starshipCfg := filepath.Join(tmp, ".config", "starship.toml")

	mustWrite := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(termuxColors, "foreground=#fff\n")
	mustWrite(tmuxConf, "set -g status on\n")
	mustWrite(peaclockCfg, "style text white\n")
	mustWrite(starshipCfg, "[character]\nsuccess_symbol='x'\n")

	backupDir := filepath.Join(tmp, "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := testThemePayload()

	if err := applyThemeFiles(payload, backupDir); err != nil {
		t.Fatalf("applyThemeFiles() error: %v", err)
	}
	for _, rel := range []string{"colors.properties.bak", "tmux.conf.bak", "peaclock.config.bak", "starship.toml.bak"} {
		if _, err := os.Stat(filepath.Join(backupDir, rel)); err != nil {
			t.Fatalf("missing backup %s: %v", rel, err)
		}
	}

	if err := applyThemeFiles(payload, backupDir); err != nil {
		t.Fatalf("second applyThemeFiles() error: %v", err)
	}
	raw, err := os.ReadFile(tmuxConf)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Count(body, "# >>> MATUGEN THEME START >>>") != 1 {
		t.Fatalf("tmux block duplicated: %s", body)
	}
	if strings.Count(body, "# <<< MATUGEN THEME END <<<") != 1 {
		t.Fatalf("tmux block end duplicated: %s", body)
	}
}

func TestScoreCandidatePrefersDarkForDarkScene(t *testing.T) {
	metrics := autoDecisionMetrics{
		MeanLuma:         0.22,
		P10:              0.04,
		P50:              0.18,
		P90:              0.55,
		DarkPixelRatio:   0.84,
		BrightPixelRatio: 0.02,
		EdgeWeightedLuma: 0.24,
		MeanSat:          0.34,
		P90Sat:           0.61,
	}
	dark := matugenCandidate{
		Mode:        "dark",
		Scheme:      "scheme-expressive",
		SourceIndex: 2,
		Roles: map[string]string{
			"background":    "#111218",
			"on_background": "#e7e8f0",
			"primary":       "#7aa2f7",
			"secondary":     "#7dcfff",
			"tertiary":      "#bb9af7",
			"error":         "#f7768e",
		},
		Readability: 13.8,
	}
	light := dark
	light.Roles = map[string]string{}
	for k, v := range dark.Roles {
		light.Roles[k] = v
	}
	light.Mode = "light"
	light.Roles["background"] = "#eff1f5"
	light.Roles["on_background"] = "#3a3f50"
	if scoreCandidate(dark, metrics) <= scoreCandidate(light, metrics) {
		t.Fatalf("expected dark candidate to win for dark scene")
	}
}

func TestScoreCandidatePenalizesLowContrast(t *testing.T) {
	metrics := autoDecisionMetrics{MeanLuma: 0.40, P50: 0.40, DarkPixelRatio: 0.50, P90Sat: 0.5}
	bad := matugenCandidate{
		Mode:        "dark",
		Scheme:      "scheme-tonal-spot",
		SourceIndex: 0,
		Roles: map[string]string{
			"background":    "#1a1b26",
			"on_background": "#222433",
			"primary":       "#7aa2f7",
			"secondary":     "#7dcfff",
			"tertiary":      "#bb9af7",
			"error":         "#f7768e",
		},
		Readability: 8.2,
	}
	good := bad
	good.Roles = map[string]string{}
	for k, v := range bad.Roles {
		good.Roles[k] = v
	}
	good.Roles["on_background"] = "#e7e8f0"
	if scoreCandidate(bad, metrics) >= scoreCandidate(good, metrics) {
		t.Fatalf("expected low-contrast candidate to be penalized")
	}
}

func TestCanonicalProfileMapsLegacyStyles(t *testing.T) {
	if got := canonicalProfile("balanced"); got != "adaptive" {
		t.Fatalf("balanced should map to adaptive, got %q", got)
	}
	if got := canonicalProfile("mellow"); got != "adaptive" {
		t.Fatalf("mellow should map to adaptive, got %q", got)
	}
	if got := canonicalProfile("tokyonight"); got != "neon-night" {
		t.Fatalf("tokyonight should map to neon-night, got %q", got)
	}
}

func TestParseThemeApplyFlagsWidgetToggles(t *testing.T) {
	cfg, err := parseThemeApplyFlags([]string{
		"--widget-battery", "off",
		"--widget-cpu", "1",
		"--widget-ram", "false",
		"--widget-weather", "on",
	})
	if err != nil {
		t.Fatalf("parseThemeApplyFlags() error: %v", err)
	}
	if cfg.WidgetBattery {
		t.Fatalf("widget battery should be off")
	}
	if !cfg.WidgetCPU {
		t.Fatalf("widget cpu should be on")
	}
	if cfg.WidgetRAM {
		t.Fatalf("widget ram should be off")
	}
	if !cfg.WidgetWeather {
		t.Fatalf("widget weather should be on")
	}
}

func TestParseThemeApplyFlagsStyleFamilyAndProfileAlias(t *testing.T) {
	cfg, err := parseThemeApplyFlags([]string{"--style-family", "warm-retro"})
	if err != nil {
		t.Fatalf("parseThemeApplyFlags() style-family error: %v", err)
	}
	if cfg.StyleFamily != "warm-retro" || cfg.Profile != "warm-retro" {
		t.Fatalf("expected style family normalization, got style=%q profile=%q", cfg.StyleFamily, cfg.Profile)
	}

	cfg, err = parseThemeApplyFlags([]string{"--profile", "vivid-noir"})
	if err != nil {
		t.Fatalf("parseThemeApplyFlags() profile alias error: %v", err)
	}
	if cfg.StyleFamily != "vivid-noir" || cfg.Profile != "vivid-noir" {
		t.Fatalf("expected profile alias to map to style-family, got style=%q profile=%q", cfg.StyleFamily, cfg.Profile)
	}
}

func TestParseThemeApplyFlagsRejectsInvalidWidgetToggle(t *testing.T) {
	if _, err := parseThemeApplyFlags([]string{"--widget-weather", "maybe"}); err == nil {
		t.Fatalf("expected invalid widget toggle value to fail")
	}
}

func TestDarkDominantSceneDetection(t *testing.T) {
	m := autoDecisionMetrics{
		MeanLuma:         0.30,
		P50:              0.26,
		DarkPixelRatio:   0.72,
		EdgeWeightedLuma: 0.31,
	}
	if !darkDominantScene(m) {
		t.Fatalf("expected dark dominant scene")
	}
}

func TestNonBlackStatusColorPromotesNearBlack(t *testing.T) {
	got := nonBlackStatusColor("#050505", "#c7c7b7")
	if strings.EqualFold(strings.TrimSpace(got), "#050505") || strings.EqualFold(strings.TrimSpace(got), "#000000") {
		t.Fatalf("expected non-black promoted status color, got %s", got)
	}
}

func TestAvoidRedHueSwitchesToFallback(t *testing.T) {
	got := avoidRedHue("#ff3333", "#4f8dff", "#56c8ff")
	if strings.EqualFold(strings.TrimSpace(got), "#ff3333") {
		t.Fatalf("expected red hue to be replaced, got %s", got)
	}
}

func TestAutoCandidateModesForDarkScene(t *testing.T) {
	modes := autoCandidateModes("auto", autoDecisionMetrics{
		MeanLuma:         0.30,
		P50:              0.26,
		DarkPixelRatio:   0.72,
		EdgeWeightedLuma: 0.31,
	})
	if len(modes) != 1 || modes[0] != "dark" {
		t.Fatalf("expected forced dark mode candidates, got %v", modes)
	}
}

func TestAutoCandidateModesForMixedScene(t *testing.T) {
	modes := autoCandidateModes("auto", autoDecisionMetrics{
		MeanLuma:         0.50,
		P50:              0.50,
		DarkPixelRatio:   0.35,
		BrightPixelRatio: 0.20,
		EdgeWeightedLuma: 0.50,
	})
	if len(modes) != 1 || modes[0] != "dark" {
		t.Fatalf("expected readability-first dark candidates, got %v", modes)
	}
}
