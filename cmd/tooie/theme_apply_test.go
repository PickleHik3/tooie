package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmuxOptionValue(block, key string) (string, bool) {
	prefix := "set -g " + key + " \""
	for _, line := range strings.Split(block, "\n") {
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "\"") {
			continue
		}
		v := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "\"")
		return strings.TrimSpace(v), true
	}
	return "", false
}

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
	if !strings.Contains(got, `set -g status-left "#(\$HOME/.config/tooie/configs/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}')`) {
		t.Fatalf("expected status-left to use the widget-left helper, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-separator ""`) {
		t.Fatalf("expected empty window separator, got: %s", got)
	}
	if !strings.Contains(got, `set -g mouse on`) {
		t.Fatalf("expected tmux mouse mode to be enabled, got: %s", got)
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
	if !strings.Contains(got, `set -g status-right "#(\$HOME/.config/tooie/configs/tmux/run-system-widget all)#(\$HOME/.config/tooie/configs/tmux/widget-weather)"`) {
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
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, `set -g window-status-activity-style "`) && strings.Contains(line, ",bg=") {
			t.Fatalf("expected activity style to avoid background fills, got: %s", line)
		}
		if strings.HasPrefix(line, `set -g window-status-bell-style "`) && strings.Contains(line, ",bg=") {
			t.Fatalf("expected bell style to avoid background fills, got: %s", line)
		}
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
	if !strings.Contains(got, `set -g status-left "#(\$HOME/.config/tooie/configs/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}') "`) {
		t.Fatalf("expected rounded theme to keep one gap between session and windows, got: %s", got)
	}
	if !strings.Contains(got, `set -g window-status-format "#{?window_start_flag`) || !strings.Contains(got, ``) {
		t.Fatalf("expected rounded theme to use rounded inactive window chips, got: %s", got)
	}
	if !strings.Contains(got, `#{?window_end_flag,#I,#I }`) {
		t.Fatalf("expected rounded inactive windows to trim trailing space on last window, got: %s", got)
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
	if !strings.Contains(got, `set -g status-left "#(\$HOME/.config/tooie/configs/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}')"`) {
		t.Fatalf("expected rectangle theme to keep left widget flush with windows, got: %s", got)
	}
	if !strings.Contains(got, `set -g @status-tmux-widget-gap-right "none"`) {
		t.Fatalf("expected rectangle theme to remove right widget gaps, got: %s", got)
	}
	if strings.Contains(got, `window-status-format "#[fg=`) && strings.Contains(got, ``) {
		t.Fatalf("expected rectangle theme windows to stay rectangular, got: %s", got)
	}
}

func TestRenderTmuxBlockStatusLayoutOptions(t *testing.T) {
	payload := testThemePayload()
	payload.Meta["status_position"] = "bottom"
	payload.Meta["status_layout"] = "single-line"
	payload.Meta["status_separator"] = "on"
	got := renderTmuxBlock(payload)
	if !strings.Contains(got, `set -g status-position "bottom"`) {
		t.Fatalf("expected status-position bottom, got: %s", got)
	}
	if !strings.Contains(got, `set -g status 1`) {
		t.Fatalf("expected single-line status to set status 1, got: %s", got)
	}
	if strings.Contains(got, `status-format[0] "`) || strings.Contains(got, `status-format[1] "`) {
		t.Fatalf("expected single-line status to clear separator rows, got: %s", got)
	}
}

func TestRenderTmuxBlockTwoLineKeepsSeparatorRule(t *testing.T) {
	payload := testThemePayload()
	payload.Meta["status_layout"] = "two-line"
	payload.Meta["status_separator"] = "off"
	got := renderTmuxBlock(payload)
	if !strings.Contains(got, `set -g status 2`) {
		t.Fatalf("expected two-line status, got: %s", got)
	}
	if !strings.Contains(got, `set -g status-format[1] "`) {
		t.Fatalf("expected two-line layout to keep separator rule line, got: %s", got)
	}
	if strings.Contains(got, "set -gu status-format\n") {
		t.Fatalf("expected two-line layout to avoid global status-format unset, got: %s", got)
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

	termuxColors := managedTermuxFilePath("colors.properties")
	tmuxConf := managedTmuxConfPath()
	peaclockCfg := managedPeaclockPath()
	starshipCfg := managedStarshipPath()

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

func TestApplyThemeFilesUsesSelectedStarshipTemplate(t *testing.T) {
	tmp := t.TempDir()
	oldHome, oldCfg := homeDir, tooieConfigDir
	homeDir = tmp
	tooieConfigDir = filepath.Join(tmp, ".config", "tooie")
	t.Cleanup(func() {
		homeDir = oldHome
		tooieConfigDir = oldCfg
	})

	backupDir := filepath.Join(tmp, "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}
	starshipCfg := managedStarshipPath()
	if err := ensureFileWithDirs(starshipCfg); err != nil {
		t.Fatalf("ensure starship cfg: %v", err)
	}
	if err := os.WriteFile(starshipCfg, []byte("[character]\nsuccess_symbol='x'\n[battery]\ndisabled=true\n"), 0o644); err != nil {
		t.Fatalf("write starship cfg: %v", err)
	}

	payload := testThemePayload()
	payload.Meta["starship_prompt"] = "nerd-font-symbols"
	if err := applyThemeFiles(payload, backupDir); err != nil {
		t.Fatalf("applyThemeFiles() error: %v", err)
	}

	raw, err := os.ReadFile(starshipCfg)
	if err != nil {
		t.Fatalf("read starship cfg: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, `[aws]`) || !strings.Contains(got, `symbol = " "`) {
		t.Fatalf("expected nerd-font-symbols template content, got:\n%s", got)
	}
	if strings.Contains(got, "[battery]") {
		t.Fatalf("expected battery section to be stripped, got:\n%s", got)
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

func TestScoreCandidateAdaptivePrefersWallpaperHueAffinity(t *testing.T) {
	metrics := autoDecisionMetrics{
		MeanLuma:         0.40,
		P50:              0.38,
		DarkPixelRatio:   0.58,
		BrightPixelRatio: 0.06,
		EdgeWeightedLuma: 0.36,
		MeanSat:          0.42,
		P90Sat:           0.70,
		DominantHue:      30.0,
		SecondaryHue:     285.0,
		HueStrength:      0.72,
	}
	matching := matugenCandidate{
		Mode:        "dark",
		Scheme:      "scheme-tonal-spot",
		SourceIndex: 1,
		Roles: map[string]string{
			"background":    "#111218",
			"on_background": "#ececf2",
			"primary":       "#f29f52",
			"secondary":     "#d4833f",
			"tertiary":      "#bb86f2",
			"error":         "#e66a5c",
		},
		Readability: 13.4,
	}
	offHue := matching
	offHue.SourceIndex = 2
	offHue.Roles = map[string]string{
		"background":    "#111218",
		"on_background": "#ececf2",
		"primary":       "#4f8cff",
		"secondary":     "#4ac2ff",
		"tertiary":      "#64d0ff",
		"error":         "#39a2ff",
	}
	if scoreCandidate(matching, metrics) <= scoreCandidate(offHue, metrics) {
		t.Fatalf("expected adaptive scorer to prefer wallpaper-hue-aligned candidate")
	}
}

func TestSourceIndexCandidatesDefault(t *testing.T) {
	got := sourceIndexCandidates(themeApplyConfig{ExtractSourceIndex: -1})
	want := []int{0, 1, 2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("sourceIndexCandidates(default) len=%d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sourceIndexCandidates(default)[%d]=%d, want %d (%v)", i, got[i], want[i], got)
		}
	}
}

func TestCanonicalProfileMapsLegacyStyles(t *testing.T) {
	if got := canonicalProfile("balanced"); got != "auto" {
		t.Fatalf("balanced should map to auto, got %q", got)
	}
	if got := canonicalProfile("mellow"); got != "auto" {
		t.Fatalf("mellow should map to auto, got %q", got)
	}
	if got := canonicalProfile("tokyonight"); got != "auto" {
		t.Fatalf("tokyonight should map to auto, got %q", got)
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
	if cfg.StyleFamily != "auto" || cfg.Profile != "auto" {
		t.Fatalf("expected style family normalization, got style=%q profile=%q", cfg.StyleFamily, cfg.Profile)
	}

	cfg, err = parseThemeApplyFlags([]string{"--profile", "vivid-noir"})
	if err != nil {
		t.Fatalf("parseThemeApplyFlags() profile alias error: %v", err)
	}
	if cfg.StyleFamily != "auto" || cfg.Profile != "auto" {
		t.Fatalf("expected profile alias to map to style-family, got style=%q profile=%q", cfg.StyleFamily, cfg.Profile)
	}
}

func TestParseThemeApplyFlagsRejectsInvalidWidgetToggle(t *testing.T) {
	if _, err := parseThemeApplyFlags([]string{"--widget-weather", "maybe"}); err == nil {
		t.Fatalf("expected invalid widget toggle value to fail")
	}
}

func TestParseThemeApplyFlagsRejectsAutoMode(t *testing.T) {
	if _, err := parseThemeApplyFlags([]string{"--mode", "auto"}); err == nil {
		t.Fatalf("expected auto mode to be rejected")
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
		t.Fatalf("expected scene-locked dark mode candidate, got %v", modes)
	}
}

func TestAutoCandidateModesForBrightScene(t *testing.T) {
	modes := autoCandidateModes("auto", autoDecisionMetrics{
		MeanLuma:         0.61,
		P50:              0.56,
		DarkPixelRatio:   0.12,
		BrightPixelRatio: 0.34,
		EdgeWeightedLuma: 0.60,
	})
	if len(modes) != 1 || modes[0] != "light" {
		t.Fatalf("expected scene-locked light mode candidate, got %v", modes)
	}
}

func TestAutoCandidateModesForMixedSceneFallsBackToDark(t *testing.T) {
	modes := autoCandidateModes("auto", autoDecisionMetrics{
		MeanLuma:         0.49,
		P50:              0.49,
		DarkPixelRatio:   0.27,
		BrightPixelRatio: 0.21,
		EdgeWeightedLuma: 0.49,
	})
	if len(modes) != 1 || modes[0] != "dark" {
		t.Fatalf("expected mixed scene to fallback dark mode candidate, got %v", modes)
	}
}

func TestApplyThemeFilesLinuxWritesGhosttyAndSkipsTermuxColors(t *testing.T) {
	tmp := t.TempDir()
	oldHome, oldCfg := homeDir, tooieConfigDir
	homeDir = tmp
	tooieConfigDir = filepath.Join(tmp, ".config", "tooie")
	t.Cleanup(func() {
		homeDir = oldHome
		tooieConfigDir = oldCfg
	})

	settings := defaultTooieSettings()
	settings.Platform.Profile = "linux"
	if err := saveTooieSettings(settings); err != nil {
		t.Fatalf("saveTooieSettings() error: %v", err)
	}

	backupDir := filepath.Join(tmp, "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := testThemePayload()
	if err := applyThemeFiles(payload, backupDir); err != nil {
		t.Fatalf("applyThemeFiles() error: %v", err)
	}

	ghosttyTheme := filepath.Join(tmp, ".config", "tooie", "configs", "ghostty", "dank-theme.conf")
	raw, err := os.ReadFile(ghosttyTheme)
	if err != nil {
		t.Fatalf("missing ghostty theme: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "palette = 15=") {
		t.Fatalf("ghostty theme missing palette entries: %s", body)
	}

	ghosttyConfig := filepath.Join(tmp, ".config", "ghostty", "config")
	cfgRaw, err := os.ReadFile(ghosttyConfig)
	if err != nil {
		t.Fatalf("missing ghostty config: %v", err)
	}
	if !strings.Contains(string(cfgRaw), "TOOIE GHOSTTY THEME START") {
		t.Fatalf("ghostty bootstrap block missing: %s", string(cfgRaw))
	}

	termuxColors := managedTermuxFilePath("colors.properties")
	if _, err := os.Stat(termuxColors); !os.IsNotExist(err) {
		t.Fatalf("expected linux flow to skip termux colors write")
	}
}

func TestPresetTmuxStatusContrastAcrossFamilies(t *testing.T) {
	tmp := t.TempDir()
	for _, family := range presetFamilyOrder {
		variants := presetVariantsByFamily[family]
		if len(variants) == 0 {
			t.Fatalf("missing variants for family %q", family)
		}
		for _, variant := range variants {
			cfg := themeApplyConfig{
				ThemeSource:   "preset",
				PresetFamily:  family,
				PresetVariant: variant,
				StatusPalette: "default",
				StatusTheme:   "rounded",
				StyleFamily:   "auto",
				Profile:       "auto",
			}
			payload, _, err := computeThemePayload(cfg, tmp)
			if err != nil {
				t.Fatalf("computeThemePayload(%s:%s) error: %v", family, variant, err)
			}
			block := renderTmuxBlock(payload)
			accentFG, ok := tmuxOptionValue(block, "@status-tmux-fg-on-accent")
			if !ok {
				t.Fatalf("missing @status-tmux-fg-on-accent for %s:%s", family, variant)
			}
			bgKeys := []string{
				"@status-tmux-left-bg-session",
				"@status-tmux-left-bg-prefix",
				"@status-tmux-left-bg-copy",
				"@status-tmux-bg-battery",
				"@status-tmux-bg-charging",
				"@status-tmux-bg-cpu",
				"@status-tmux-bg-ram",
				"@status-tmux-bg-weather",
			}
			for _, k := range bgKeys {
				bg, ok := tmuxOptionValue(block, k)
				if !ok {
					t.Fatalf("missing %s for %s:%s", k, family, variant)
				}
				if bg == "default" {
					bg = payload.Background
				}
				if got := contrastRatioHex(accentFG, bg); got < 3.6 {
					t.Fatalf("accent contrast too low for %s:%s (%s): %.2f fg=%s bg=%s", family, variant, k, got, accentFG, bg)
				}
			}
			for _, stateKey := range []string{
				"@status-tmux-color-weather",
				"@status-tmux-color-charging",
			} {
				fg, ok := tmuxOptionValue(block, stateKey)
				if !ok {
					t.Fatalf("missing %s for %s:%s", stateKey, family, variant)
				}
				if got := contrastRatioHex(fg, payload.Background); got < 4.2 {
					t.Fatalf("%s contrast too low for %s:%s: %.2f fg=%s bg=%s", stateKey, family, variant, got, fg, payload.Background)
				}
			}
		}
	}
}
