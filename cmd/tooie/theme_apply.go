package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
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
	StatusTheme   string
	StatusPosition string
	StatusLayout   string
	StatusSeparator string
	StyleFamily   string
	Profile       string
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
	WidgetBattery bool
	WidgetCPU     bool
	WidgetRAM     bool
	WidgetWeather bool
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
	meta["status_theme"] = cfg.StatusTheme
	meta["status_position"] = cfg.StatusPosition
	meta["status_layout"] = cfg.StatusLayout
	meta["status_separator"] = cfg.StatusSeparator
	meta["widget_battery"] = onOffFlag(cfg.WidgetBattery)
	meta["widget_cpu"] = onOffFlag(cfg.WidgetCPU)
	meta["widget_ram"] = onOffFlag(cfg.WidgetRAM)
	meta["widget_weather"] = onOffFlag(cfg.WidgetWeather)
	if cfg.ThemeSource != "preset" {
		family := canonicalProfile(cfg.StyleFamily)
		meta["style_family"] = family
		meta["style_family_version"] = "1"
		meta["profile"] = family // backward compatibility for existing readers
	}
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

type autoDecisionMetrics struct {
	MeanLuma         float64
	P10              float64
	P50              float64
	P90              float64
	DarkPixelRatio   float64
	BrightPixelRatio float64
	EdgeWeightedLuma float64
	MeanSat          float64
	P90Sat           float64
	DominantHue      float64
	SecondaryHue     float64
	HueStrength      float64
}

func computeThemePayload(cfg themeApplyConfig, workDir string) (computedPayload, []byte, error) {
	var out computedPayload
	var roles map[string]string
	var matugenRaw []byte
	autoMeta := map[string]string{}
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
			raw, mode, meta, err := generateMatugenJSON(cfg, wallpaper, workDir)
			if err != nil {
				return out, nil, err
			}
			matugenRaw = raw
			effectiveMode = mode
			autoMeta = meta
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
		StyleFamily:    cfg.StyleFamily,
		Profile:        cfg.StyleFamily, // backwards compatibility in theme.Compute
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
	for role, hex := range out.Roles {
		role = strings.TrimSpace(role)
		hex = strings.TrimSpace(hex)
		if role == "" || hex == "" {
			continue
		}
		out.Meta["effective_role_"+role] = strings.ToLower(hex)
	}
	out.Meta["status_palette"] = cfg.StatusPalette
	out.Meta["status_theme"] = cfg.StatusTheme
	out.Meta["status_position"] = cfg.StatusPosition
	out.Meta["status_layout"] = cfg.StatusLayout
	out.Meta["status_separator"] = cfg.StatusSeparator
	out.Meta["widget_battery"] = onOffFlag(cfg.WidgetBattery)
	out.Meta["widget_cpu"] = onOffFlag(cfg.WidgetCPU)
	out.Meta["widget_ram"] = onOffFlag(cfg.WidgetRAM)
	out.Meta["widget_weather"] = onOffFlag(cfg.WidgetWeather)
	for k, v := range autoMeta {
		out.Meta[k] = v
	}
	out.Status.Separator = computed.Status.Separator
	out.Status.Weather = computed.Status.Weather
	out.Status.Charging = computed.Status.Charging
	out.Status.Battery = append([]string{}, computed.Status.Battery[:]...)
	out.Status.CPU = append([]string{}, computed.Status.CPU[:]...)
	out.Status.RAM = append([]string{}, computed.Status.RAM[:]...)
	return out, matugenRaw, nil
}

func generateMatugenJSON(cfg themeApplyConfig, wallpaper, workDir string) ([]byte, string, map[string]string, error) {
	_ = writeApplyProgress("Generating dynamic palette", 0.14)
	meta := map[string]string{}
	mode := canonicalMode(cfg.Mode)
	metrics := analyzeWallpaperLuma(wallpaper)
	candidates, err := collectMatugenCandidates(cfg, wallpaper, mode, metrics)
	if err != nil {
		return nil, "", nil, err
	}
	if len(candidates) == 0 {
		return nil, "", nil, fmt.Errorf("no matugen candidates evaluated")
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Score > best.Score {
			best = c
		}
	}

	meta["auto_mean_luma"] = fmt.Sprintf("%.4f", metrics.MeanLuma)
	meta["auto_p10"] = fmt.Sprintf("%.4f", metrics.P10)
	meta["auto_p50"] = fmt.Sprintf("%.4f", metrics.P50)
	meta["auto_p90"] = fmt.Sprintf("%.4f", metrics.P90)
	meta["auto_dark_ratio"] = fmt.Sprintf("%.4f", metrics.DarkPixelRatio)
	meta["auto_bright_ratio"] = fmt.Sprintf("%.4f", metrics.BrightPixelRatio)
	meta["auto_edge_luma"] = fmt.Sprintf("%.4f", metrics.EdgeWeightedLuma)
	meta["auto_mean_sat"] = fmt.Sprintf("%.4f", metrics.MeanSat)
	meta["auto_p90_sat"] = fmt.Sprintf("%.4f", metrics.P90Sat)
	meta["auto_hue_strength"] = fmt.Sprintf("%.4f", metrics.HueStrength)
	if metrics.DominantHue >= 0 {
		meta["auto_dominant_hue"] = fmt.Sprintf("%.2f", metrics.DominantHue)
	}
	if metrics.SecondaryHue >= 0 {
		meta["auto_secondary_hue"] = fmt.Sprintf("%.2f", metrics.SecondaryHue)
	}
	meta["auto_candidate_count"] = fmt.Sprintf("%d", len(candidates))
	sceneClass, modeGate := autoSceneDecision(metrics, mode)
	meta["auto_scene_class"] = sceneClass
	meta["auto_mode_gate"] = modeGate
	meta["auto_decision_reason"] = "deep-score"
	meta["auto_selected_scheme"] = best.Scheme
	meta["auto_selected_source_index"] = fmt.Sprintf("%d", best.SourceIndex)
	meta["auto_selected_score"] = fmt.Sprintf("%.4f", best.Score)
	meta["auto_selected_readability"] = fmt.Sprintf("%.4f", best.Readability)
	meta["auto_selected_ansi_delta"] = fmt.Sprintf("%.4f", ansiSeparationScore(best.Roles))
	meta["auto_selected_scene_fit"] = fmt.Sprintf("%.4f", sceneFitScore(best.Mode, metrics))
	meta["auto_selected_wallpaper_affinity"] = fmt.Sprintf("%.4f", wallpaperHueAffinityScore(best.Roles, metrics))

	return best.Raw, best.Mode, meta, nil
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

func maxFloat(v float64, more ...float64) float64 {
	m := v
	for _, x := range more {
		if x > m {
			m = x
		}
	}
	return m
}

type matugenCandidate struct {
	Raw         []byte
	Mode        string
	Scheme      string
	SourceIndex int
	Roles       map[string]string
	Readability float64
	Score       float64
}

func collectMatugenCandidates(cfg themeApplyConfig, wallpaper, mode string, metrics autoDecisionMetrics) ([]matugenCandidate, error) {
	schemes := pickSchemeCandidates(cfg.SchemeType)
	modes := autoCandidateModes(mode, metrics)
	indices := sourceIndexCandidates(cfg.StyleFamily)
	candidates := make([]matugenCandidate, 0, len(modes)*len(schemes)*len(indices))
	var lastErr error
	for _, m := range modes {
		for _, s := range schemes {
			for _, idx := range indices {
				raw, err := runMatugenImage(cfg.MatugenBin, wallpaper, m, s, idx)
				if err != nil {
					lastErr = err
					continue
				}
				roles, err := itheme.ParseMatugenColors(raw)
				if err != nil {
					lastErr = err
					continue
				}
				readability := readabilityScore(roles)
				c := matugenCandidate{
					Raw:         raw,
					Mode:        m,
					Scheme:      s,
					SourceIndex: idx,
					Roles:       roles,
					Readability: readability,
				}
				c.Score = scoreCandidate(c, metrics, cfg.StyleFamily)
				candidates = append(candidates, c)
			}
		}
	}
	if len(candidates) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("failed to evaluate any matugen candidates")
		}
		return nil, lastErr
	}
	return candidates, nil
}

func autoCandidateModes(mode string, metrics autoDecisionMetrics) []string {
	if mode != "auto" {
		return []string{mode}
	}
	// Readability-first policy for terminal use: auto always resolves to dark candidates.
	return []string{"dark"}
}

func autoSceneDecision(metrics autoDecisionMetrics, mode string) (string, string) {
	if mode != "auto" {
		return mode, "explicit-" + mode
	}
	if darkDominantScene(metrics) {
		return "dark", "forced-dark"
	}
	if brightDominantScene(metrics) {
		return "bright", "forced-light"
	}
	return "mixed", "dual"
}

func pickSchemeCandidates(schemeType string) []string {
	s := strings.TrimSpace(schemeType)
	if s != "" {
		return []string{s}
	}
	return []string{"scheme-expressive", "scheme-tonal-spot", "scheme-fidelity", "scheme-content"}
}

func sourceIndexCandidates(styleFamily string) []int {
	if canonicalProfile(styleFamily) == "adaptive" {
		// Keep adaptive anchored to the strongest wallpaper seed colors.
		return []int{0, 1, 2}
	}
	return []int{0, 1, 2, 3, 4}
}

func scoreCandidate(c matugenCandidate, metrics autoDecisionMetrics, styleFamily string) float64 {
	isAdaptive := canonicalProfile(styleFamily) == "adaptive"
	bg := getRoleOr(c.Roles, "background", "#1a1b26")
	fg := getRoleOr(c.Roles, "on_background", "#c0caf5")
	fgContrast := contrastRatioHex(fg, bg)
	if fgContrast < 7.0 {
		return -1000 + fgContrast
	}
	accentMin := minFloat(
		contrastRatioHex(getRoleOr(c.Roles, "primary", "#7aa2f7"), bg),
		contrastRatioHex(getRoleOr(c.Roles, "secondary", "#7dcfff"), bg),
		contrastRatioHex(getRoleOr(c.Roles, "tertiary", "#bb9af7"), bg),
		contrastRatioHex(getRoleOr(c.Roles, "error", "#ff5f5f"), bg),
	)
	if accentMin < 3.0 {
		return -800 + accentMin
	}

	sceneFit := sceneFitScore(c.Mode, metrics)
	ansiSep := ansiSeparationScore(c.Roles)
	wallpaperAffinity := wallpaperHueAffinityScore(c.Roles, metrics)
	modePenalty := 0.0
	if darkDominantScene(metrics) && c.Mode == "light" {
		modePenalty += 2.4
	}
	if brightDominantScene(metrics) && c.Mode == "dark" {
		modePenalty += 0.65
	}
	sourcePenalty := 0.0
	if c.SourceIndex > 0 {
		sourcePenalty = 0.22 * float64(c.SourceIndex)
		if isAdaptive {
			sourcePenalty = 0.42 * float64(c.SourceIndex)
		} else if metrics.P90Sat > 0.45 {
			sourcePenalty *= 0.65
		}
	}
	schemeBias := 0.0
	switch c.Scheme {
	case "scheme-expressive":
		schemeBias = 0.22 + (0.35 * clamp01(metrics.P90Sat-0.32))
		if isAdaptive {
			schemeBias -= 0.20
		}
	case "scheme-fidelity", "scheme-content":
		schemeBias = 0.16 + (0.28 * clamp01(metrics.P90Sat-0.28))
	case "scheme-tonal-spot":
		schemeBias = 0.18 + (0.25 * clamp01(0.40-metrics.P90Sat))
	}
	affinityWeight := 0.75
	if isAdaptive {
		affinityWeight = 1.45
	}
	return (c.Readability * 1.85) + (fgContrast * 0.9) + (accentMin * 0.85) + (ansiSep * 0.8) + (wallpaperAffinity * affinityWeight) + sceneFit + schemeBias - sourcePenalty - modePenalty
}

func wallpaperHueAffinityScore(roles map[string]string, metrics autoDecisionMetrics) float64 {
	if metrics.HueStrength <= 0.05 || metrics.DominantHue < 0 {
		return 0
	}
	accents := []string{
		getRoleOr(roles, "primary", "#7aa2f7"),
		getRoleOr(roles, "secondary", "#7dcfff"),
		getRoleOr(roles, "tertiary", "#bb9af7"),
		getRoleOr(roles, "error", "#f7768e"),
	}
	sum := 0.0
	for _, hex := range accents {
		h := hueFromHex(hex)
		d1 := hueDistanceDegrees(h, metrics.DominantHue)
		closeness := 1.0 - (d1 / 180.0)
		if metrics.SecondaryHue >= 0 {
			d2 := hueDistanceDegrees(h, metrics.SecondaryHue)
			c2 := 1.0 - (d2 / 180.0)
			closeness = maxFloat(closeness, c2*0.82)
		}
		sum += closeness
	}
	avg := sum / float64(len(accents))
	return (avg - 0.5) * 2.0 * clamp01(metrics.HueStrength*1.35)
}

func sceneFitScore(mode string, metrics autoDecisionMetrics) float64 {
	darkMass := 0.55*metrics.DarkPixelRatio + 0.25*clamp01(0.52-metrics.P50) + 0.20*clamp01(0.50-metrics.EdgeWeightedLuma)
	brightMass := 0.55*metrics.BrightPixelRatio + 0.25*clamp01(metrics.P50-0.48) + 0.20*clamp01(metrics.EdgeWeightedLuma-0.45)
	if mode == "dark" {
		return darkMass - (0.55 * brightMass)
	}
	return brightMass - (0.55 * darkMass)
}

func darkDominantScene(metrics autoDecisionMetrics) bool {
	return (metrics.DarkPixelRatio > 0.45 && (metrics.P50 < 0.50 || metrics.MeanLuma < 0.48)) ||
		(metrics.P50 < 0.40 && metrics.DarkPixelRatio > 0.52) ||
		(metrics.MeanLuma < 0.43 && metrics.EdgeWeightedLuma < 0.50)
}

func brightDominantScene(metrics autoDecisionMetrics) bool {
	return (metrics.BrightPixelRatio > 0.28 && (metrics.P50 > 0.56 || metrics.MeanLuma > 0.56)) ||
		(metrics.P50 > 0.60 && metrics.DarkPixelRatio < 0.36) ||
		(metrics.MeanLuma > 0.63 && metrics.BrightPixelRatio > 0.22)
}

func ansiSeparationScore(roles map[string]string) float64 {
	accents := []string{
		getRoleOr(roles, "error", "#ff5f5f"),
		getRoleOr(roles, "secondary", "#7dcfff"),
		getRoleOr(roles, "tertiary", "#bb9af7"),
		getRoleOr(roles, "primary", "#7aa2f7"),
	}
	sum := 0.0
	count := 0
	for i := 0; i < len(accents); i++ {
		ha := hueFromHex(accents[i])
		for j := i + 1; j < len(accents); j++ {
			hb := hueFromHex(accents[j])
			d := math.Abs(ha - hb)
			if d > 180 {
				d = 360 - d
			}
			sum += d / 180.0
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func analyzeWallpaperLuma(path string) autoDecisionMetrics {
	imgMetrics, ok := wallpaperImageMetrics(path)
	if ok {
		return imgMetrics
	}
	// Fallback for environments where decode fails.
	l := wallpaperLuma(path)
	if l < 0 {
		return autoDecisionMetrics{MeanLuma: 0.5, P10: 0.5, P50: 0.5, P90: 0.5, DarkPixelRatio: 0.5, BrightPixelRatio: 0.0, EdgeWeightedLuma: 0.5, MeanSat: 0.35, P90Sat: 0.55, DominantHue: -1, SecondaryHue: -1, HueStrength: 0}
	}
	return autoDecisionMetrics{MeanLuma: l, P10: l, P50: l, P90: l, DarkPixelRatio: ternf(l < 0.25, 1, 0), BrightPixelRatio: ternf(l > 0.82, 1, 0), EdgeWeightedLuma: l, MeanSat: 0.40, P90Sat: 0.60, DominantHue: -1, SecondaryHue: -1, HueStrength: 0}
}

func parseThemeApplyFlags(args []string) (themeApplyConfig, error) {
	cfg := themeApplyConfig{
		Mode:          "auto",
		SchemeType:    "scheme-tonal-spot",
		ThemeSource:   "wallpaper",
		PresetFamily:  "catppuccin",
		PresetVariant: "mocha",
		StatusPalette: "default",
		StatusTheme:   "default",
		StatusPosition: "top",
		StatusLayout:   "two-line",
		StatusSeparator: "on",
		StyleFamily:   "adaptive",
		Profile:       "adaptive",
		WidgetBattery: true,
		WidgetCPU:     true,
		WidgetRAM:     true,
		WidgetWeather: true,
	}
	widgetBattery := "on"
	widgetCPU := "on"
	widgetRAM := "on"
	widgetWeather := "on"
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
	fs.StringVar(&cfg.StatusTheme, "status-theme", cfg.StatusTheme, "")
	fs.StringVar(&cfg.StatusPosition, "status-position", cfg.StatusPosition, "")
	fs.StringVar(&cfg.StatusLayout, "status-layout", cfg.StatusLayout, "")
	fs.StringVar(&cfg.StatusSeparator, "status-separator", cfg.StatusSeparator, "")
	fs.StringVar(&cfg.StyleFamily, "style-family", cfg.StyleFamily, "")
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "")
	fs.BoolVar(&cfg.PreviewOnly, "preview-only", false, "")
	fs.StringVar(&cfg.ReuseBackupID, "reuse-backup", "", "")
	fs.StringVar(&cfg.AnsiRed, "ansi-red", "", "")
	fs.StringVar(&cfg.AnsiGreen, "ansi-green", "", "")
	fs.StringVar(&cfg.AnsiYellow, "ansi-yellow", "", "")
	fs.StringVar(&cfg.AnsiBlue, "ansi-blue", "", "")
	fs.StringVar(&cfg.AnsiMagenta, "ansi-magenta", "", "")
	fs.StringVar(&cfg.AnsiCyan, "ansi-cyan", "", "")
	fs.StringVar(&widgetBattery, "widget-battery", widgetBattery, "")
	fs.StringVar(&widgetCPU, "widget-cpu", widgetCPU, "")
	fs.StringVar(&widgetRAM, "widget-ram", widgetRAM, "")
	fs.StringVar(&widgetWeather, "widget-weather", widgetWeather, "")
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
	cfg.StatusTheme = normalizeStatusTheme(cfg.StatusTheme)
	if cfg.StatusTheme == "" {
		return cfg, fmt.Errorf("invalid status theme")
	}
	cfg.StatusPosition = normalizeStatusPosition(cfg.StatusPosition)
	cfg.StatusLayout = normalizeStatusLayout(cfg.StatusLayout)
	cfg.StatusSeparator = normalizeSeparatorMode(cfg.StatusSeparator)
	if cfg.StatusLayout == "single-line" {
		cfg.StatusSeparator = "off"
	}
	styleFamilySet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "style-family" {
			styleFamilySet = true
		}
	})
	if !styleFamilySet || strings.TrimSpace(cfg.StyleFamily) == "" {
		cfg.StyleFamily = cfg.Profile
	}
	cfg.StyleFamily = canonicalProfile(cfg.StyleFamily)
	cfg.Profile = cfg.StyleFamily // keep both fields normalized the same
	if !contains(profilePresets, cfg.StyleFamily) {
		return cfg, fmt.Errorf("invalid style family: %s", cfg.StyleFamily)
	}
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
	var err error
	cfg.WidgetBattery, err = parseOnOffValue(widgetBattery)
	if err != nil {
		return cfg, fmt.Errorf("invalid --widget-battery value: %s", widgetBattery)
	}
	cfg.WidgetCPU, err = parseOnOffValue(widgetCPU)
	if err != nil {
		return cfg, fmt.Errorf("invalid --widget-cpu value: %s", widgetCPU)
	}
	cfg.WidgetRAM, err = parseOnOffValue(widgetRAM)
	if err != nil {
		return cfg, fmt.Errorf("invalid --widget-ram value: %s", widgetRAM)
	}
	cfg.WidgetWeather, err = parseOnOffValue(widgetWeather)
	if err != nil {
		return cfg, fmt.Errorf("invalid --widget-weather value: %s", widgetWeather)
	}
	return cfg, nil
}

func parseOnOffValue(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on", "enabled":
		return true, nil
	case "0", "false", "no", "off", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("expected one of on/off/true/false/1/0")
	}
}

func normalizeStatusTheme(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default":
		return "default"
	case "rounded":
		return "rounded"
	case "rectangle", "rect":
		return "rectangle"
	default:
		return ""
	}
}

func normalizeStatusPosition(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "bottom":
		return "bottom"
	default:
		return "top"
	}
}

func normalizeStatusLayout(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "single", "single-line", "single_line":
		return "single-line"
	default:
		return "two-line"
	}
}

func normalizeSeparatorMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "off", "none", "false", "0":
		return "off"
	default:
		return "on"
	}
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
	tmuxRamp := tmuxGradientRamp(payload)
	sessionBG := nonBlackStatusColor(
		avoidRedHue(
			blendHexColor(getRoleOr(payload.Roles, "tertiary", tmuxRamp[16]), "#b889ff", 0.55),
			tmuxRamp[16],
			"#a678ff",
			payload.Foreground,
		),
		payload.Foreground,
	)
	prefixBG := nonBlackStatusColor(
		avoidRedHue(
			blendHexColor(getRoleOr(payload.Roles, "error", payload.Colors[1]), "#ffb347", 0.55),
			"#ff6f61",
			"#ffb347",
			payload.Foreground,
		),
		payload.Foreground,
	)
	copyBG := nonBlackStatusColor(
		avoidRedHue(
			blendHexColor(getRoleOr(payload.Roles, "secondary", tmuxRamp[10]), "#00d5ff", 0.42),
			"#00c96b",
			"#2dd4ff",
			payload.Foreground,
		),
		payload.Foreground,
	)
	modeBG := nonBlackStatusColor(blendHexColor(copyBG, payload.Background, 0.15), copyBG)
	modeFG := ensureReadableTextColor(modeBG, payload.Background, payload.Foreground)
	matchBG := nonBlackStatusColor(blendHexColor(tmuxRamp[12], payload.Background, 0.25), modeBG)
	matchFG := ensureReadableTextColor(matchBG, payload.Background, payload.Foreground)
	currentMatchBG := nonBlackStatusColor(tmuxRamp[15], matchBG)
	currentMatchFG := ensureReadableTextColor(currentMatchBG, payload.Background, payload.Foreground)
	windowInactiveBG := blendHexColor(tmuxRamp[4], payload.Background, 0.90)
	windowInactiveFG := ensureReadableTextColor(windowInactiveBG, getRoleOr(payload.Roles, "on_surface_variant", payload.Foreground), payload.Foreground)
	windowActiveBG := nonBlackStatusColor(
		avoidRedHue(
			blendHexColor(getRoleOr(payload.Roles, "primary", tmuxRamp[8]), getRoleOr(payload.Roles, "secondary", tmuxRamp[10]), 0.28),
			tmuxRamp[9],
			tmuxRamp[7],
			payload.Foreground,
		),
		payload.Foreground,
	)
	windowActiveFG := nonBlackStatusColor(ensureReadableTextColor(windowActiveBG, payload.Foreground, "#ffffff"), payload.Foreground)
	attentionBG := nonBlackStatusColor(
		avoidRedHue(
			blendHexColor(getRoleOr(payload.Roles, "error", payload.Colors[1]), "#ffd166", 0.35),
			"#ff7a59",
			"#ffcc4d",
			payload.Foreground,
		),
		payload.Foreground,
	)
	attentionFG := bestTextColorForBackgrounds(
		ensureReadableTextColor(attentionBG, payload.Background, payload.Foreground),
		payload.Foreground,
		4.5,
		payload.Background,
		windowInactiveBG,
		windowActiveBG,
	)
	windowAccentFG := ensureReadableTextColor(windowActiveBG, payload.Background, payload.Foreground)
	paneBorderColor := ensureReadableTextColor(payload.Background, blendHexColor(payload.Foreground, payload.Background, 0.62), payload.Foreground)
	paneActiveBorderColor := nonBlackStatusColor(
		ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "primary", tmuxRamp[8]), payload.Foreground),
		payload.Foreground,
	)
	statusPalette := strings.TrimSpace(payload.Meta["status_palette"])
	if statusPalette == "" {
		statusPalette = "default"
	}
	statusTheme := normalizeStatusTheme(payload.Meta["status_theme"])
	if statusTheme == "" {
		statusTheme = "default"
	}
	statusPosition := normalizeStatusPosition(payload.Meta["status_position"])
	statusLayout := normalizeStatusLayout(payload.Meta["status_layout"])
	statusSeparator := normalizeSeparatorMode(payload.Meta["status_separator"])
	if statusLayout == "single-line" {
		statusSeparator = "off"
	}
	widgetBattery := onOffFlag(parseOnOffDefault(payload.Meta["widget_battery"], true))
	widgetCPU := onOffFlag(parseOnOffDefault(payload.Meta["widget_cpu"], true))
	widgetRAM := onOffFlag(parseOnOffDefault(payload.Meta["widget_ram"], true))
	widgetWeather := onOffFlag(parseOnOffDefault(payload.Meta["widget_weather"], true))
	surfaceHigh := nonBlackStatusColor(getRoleOr(payload.Roles, "surface_container_high", windowInactiveBG), payload.Foreground)
	surfaceHighest := nonBlackStatusColor(getRoleOr(payload.Roles, "surface_container_highest", blendHexColor(surfaceHigh, payload.Foreground, 0.10)), payload.Foreground)
	primaryBase := nonBlackStatusColor(getRoleOr(payload.Roles, "primary", tmuxRamp[8]), payload.Foreground)
	primaryFixed := nonBlackStatusColor(getRoleOr(payload.Roles, "primary_fixed", primaryBase), payload.Foreground)
	primaryFixedDim := nonBlackStatusColor(getRoleOr(payload.Roles, "primary_fixed_dim", primaryFixed), payload.Foreground)
	primaryContainer := nonBlackStatusColor(getRoleOr(payload.Roles, "primary_container", windowActiveBG), payload.Foreground)
	secondaryBase := nonBlackStatusColor(getRoleOr(payload.Roles, "secondary", tmuxRamp[10]), payload.Foreground)
	secondaryFixed := nonBlackStatusColor(getRoleOr(payload.Roles, "secondary_fixed", secondaryBase), payload.Foreground)
	secondaryFixedDim := nonBlackStatusColor(getRoleOr(payload.Roles, "secondary_fixed_dim", secondaryFixed), payload.Foreground)
	secondaryContainer := nonBlackStatusColor(getRoleOr(payload.Roles, "secondary_container", copyBG), payload.Foreground)
	tertiaryBase := nonBlackStatusColor(getRoleOr(payload.Roles, "tertiary", tmuxRamp[17]), payload.Foreground)
	tertiaryFixed := nonBlackStatusColor(getRoleOr(payload.Roles, "tertiary_fixed", tertiaryBase), payload.Foreground)
	tertiaryFixedDim := nonBlackStatusColor(getRoleOr(payload.Roles, "tertiary_fixed_dim", tertiaryFixed), payload.Foreground)
	tertiaryContainer := nonBlackStatusColor(getRoleOr(payload.Roles, "tertiary_container", sessionBG), payload.Foreground)
	weatherColor := bestTextColorForBackgrounds(tertiaryFixedDim, payload.Foreground, 4.5, payload.Background)
	separatorColor := ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "outline_variant", blendHexColor(payload.Foreground, payload.Background, 0.48)), payload.Foreground)
	ruleBaseColor := ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "outline_variant", separatorColor), payload.Foreground)
	rulePrefixColor := ensureReadableTextColor(payload.Background, blendHexColor(prefixBG, getRoleOr(payload.Roles, "primary", prefixBG), 0.42), payload.Foreground)
	ruleCopyColor := ensureReadableTextColor(payload.Background, blendHexColor(copyBG, getRoleOr(payload.Roles, "secondary", copyBG), 0.40), payload.Foreground)
	chargingColor := bestTextColorForBackgrounds(secondaryFixed, payload.Foreground, 4.5, payload.Background)
	batteryRamp := []string{
		"#ff6b6b", // red
		"#ff8f5a", // orange-red
		"#ffb347", // orange
		"#ffd166", // yellow
		"#b7df6a", // yellow-green
		"#49d17d", // green
	}
	batteryColors := make([]string, len(batteryRamp))
	for i, c := range batteryRamp {
		batteryColors[i] = bestTextColorForBackgrounds(saturateHexColor(c, 0.16), payload.Foreground, 4.5, payload.Background)
	}
	batteryFullColors := [4]string{}
	batteryHalfColors := [4]string{}
	batteryEmptyColors := [4]string{}
	batAnchors := []string{
		batteryColors[0],
		batteryColors[1],
		batteryColors[3],
		batteryColors[5],
	}
	for i := 0; i < 4; i++ {
		fullT := 0.06 + (0.88 * float64(i) / 3.0)
		halfT := 0.18 + (0.88 * float64(i) / 3.0)
		full := bestTextColorForBackgrounds(saturateHexColor(sampleGradientColor(batAnchors, clamp01(fullT)), 0.26), payload.Foreground, 4.5, payload.Background)
		half := bestTextColorForBackgrounds(saturateHexColor(sampleGradientColor(batAnchors, clamp01(halfT)), 0.30), payload.Foreground, 4.5, payload.Background)
		empty := bestTextColorForBackgrounds(saturateHexColor(blendHexColor(full, surfaceHighest, 0.18), 0.14), payload.Foreground, 4.5, payload.Background)
		batteryFullColors[i] = full
		batteryHalfColors[i] = half
		batteryEmptyColors[i] = empty
	}
	cpuColors := []string{
		bestTextColorForBackgrounds(primaryBase, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(primaryFixed, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(secondaryBase, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(secondaryFixed, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(tertiaryBase, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(tertiaryFixed, payload.Foreground, 4.5, payload.Background),
	}
	ramColors := []string{
		bestTextColorForBackgrounds(tertiaryBase, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(tertiaryFixed, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(primaryFixedDim, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(primaryFixed, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(secondaryFixedDim, payload.Foreground, 4.5, payload.Background),
		bestTextColorForBackgrounds(secondaryFixed, payload.Foreground, 4.5, payload.Background),
	}
	batteryBG := normalizeHexColor(blendHexColor(getRoleOr(payload.Roles, "surface_dim", payload.Background), "#ffffff", ternf(payload.EffectiveMode == "dark", 0.18, 0.24)))
	chargingBG := nonBlackStatusColor(blendHexColor(blendHexColor(secondaryBase, primaryBase, 0.35), tertiaryBase, 0.18), payload.Foreground)
	cpuBG := nonBlackStatusColor(blendHexColor(secondaryContainer, surfaceHighest, 0.20), payload.Foreground)
	ramBG := nonBlackStatusColor(blendHexColor(primaryContainer, surfaceHighest, 0.22), payload.Foreground)
	weatherBG := nonBlackStatusColor(blendHexColor(tertiaryContainer, surfaceHighest, 0.18), payload.Foreground)
	weatherColor = bestTextColorForBackgrounds(weatherColor, payload.Foreground, 4.5, payload.Background, weatherBG)
	chargingColor = bestTextColorForBackgrounds(chargingColor, payload.Foreground, 4.5, payload.Background, chargingBG)
	for i := range batteryColors {
		batteryColors[i] = bestTextColorForBackgrounds(batteryColors[i], payload.Foreground, 4.5, payload.Background, batteryBG)
	}
	for i := range batteryFullColors {
		batteryFullColors[i] = bestTextColorForBackgrounds(batteryFullColors[i], payload.Foreground, 4.5, payload.Background, batteryBG)
		batteryHalfColors[i] = bestTextColorForBackgrounds(batteryHalfColors[i], payload.Foreground, 4.5, payload.Background, batteryBG)
		batteryEmptyColors[i] = bestTextColorForBackgrounds(batteryEmptyColors[i], payload.Foreground, 4.5, payload.Background, batteryBG)
	}
	for i := range cpuColors {
		cpuColors[i] = bestTextColorForBackgrounds(cpuColors[i], payload.Foreground, 4.5, payload.Background, cpuBG)
	}
	for i := range ramColors {
		ramColors[i] = bestTextColorForBackgrounds(ramColors[i], payload.Foreground, 4.5, payload.Background, ramBG)
	}
	widgetAccentFG := bestTextColorForBackgrounds(payload.Foreground, payload.Background, 6.0, payload.Background)
	sessionBG = ensureBackgroundContrastForText(sessionBG, widgetAccentFG, 3.8)
	prefixBG = ensureBackgroundContrastForText(prefixBG, widgetAccentFG, 3.8)
	copyBG = ensureBackgroundContrastForText(copyBG, widgetAccentFG, 3.8)
	batteryBG = ensureBackgroundContrastForText(batteryBG, widgetAccentFG, 3.8)
	chargingBG = ensureBackgroundContrastForText(chargingBG, widgetAccentFG, 3.8)
	cpuBG = ensureBackgroundContrastForText(cpuBG, widgetAccentFG, 3.8)
	ramBG = ensureBackgroundContrastForText(ramBG, widgetAccentFG, 3.8)
	weatherBG = ensureBackgroundContrastForText(weatherBG, widgetAccentFG, 3.8)
	batteryBGSetting := batteryBG
	if statusTheme == "default" {
		batteryBGSetting = "default"
	}
	edgeStyle := "rounded"
	leftEdgeStyle := edgeStyle
	bgRight := "on"
	leftGap := " "
	leftSessionPad := "none"
	rightGap := "space"
	windowStatusFormat := fmt.Sprintf(`#[fg=%s,bg=%s,nobold,noitalics,nounderscore] #I `, windowInactiveFG, windowInactiveBG)
	windowStatusCurrentFormat := fmt.Sprintf(`#[fg=%s,bg=%s,bold,noitalics,nounderscore] #W `, windowActiveFG, windowActiveBG)
	switch statusTheme {
	case "rounded":
		// One outer rounded capsule for the full window list, with active window inset as its own pill.
		rightGap = "space"
		windowStatusFormat = fmt.Sprintf(`#{?window_start_flag,#[fg=%s]#[bg=default],}#[fg=%s,bg=%s,nobold,noitalics,nounderscore]#{?window_start_flag,#I ,#I }#{?window_end_flag,#[fg=%s]#[bg=default],}`, windowInactiveBG, windowInactiveFG, windowInactiveBG, windowInactiveBG)
		windowStatusCurrentFormat = fmt.Sprintf(`#{?window_start_flag,#[fg=%s]#[bg=default],}#{?window_start_flag,,#[fg=%s]#[bg=%s]}#[fg=%s,bg=%s,bold,noitalics,nounderscore]#W#{?window_end_flag,#[fg=%s]#[bg=default],}#{?window_end_flag,,#[fg=%s]#[bg=%s]}#{?window_end_flag,,#[fg=%s]#[bg=%s] }`, windowActiveBG, windowActiveBG, windowInactiveBG, windowAccentFG, windowActiveBG, windowActiveBG, windowActiveBG, windowInactiveBG, windowInactiveFG, windowInactiveBG)
	case "rectangle":
		edgeStyle = "flat"
		leftEdgeStyle = "flat"
		leftGap = " "
		rightGap = "none"
		leftSessionPad = "space"
	default:
		leftEdgeStyle = "flat"
		bgRight = "off"
		leftSessionPad = "space"
	}
	statusRuleThin := strings.Repeat("─", 260)
	statusRuleThick := strings.Repeat("━", 260)
	statusRuleExpr := fmt.Sprintf(`#{?client_prefix,#[fg=%s]%s,#{?pane_in_mode,#[fg=%s]%s,#[fg=%s]%s}}`, rulePrefixColor, statusRuleThick, ruleCopyColor, statusRuleThick, ruleBaseColor, statusRuleThin)
	statusRows := 2
	statusFormatCommands := "set -gu status-format[0]\nset -gu status-format[1]"
	if statusLayout == "single-line" {
		statusRows = 1
	} else if statusSeparator == "on" {
		if statusPosition == "bottom" {
			statusFormatCommands = fmt.Sprintf("set -g status-format[0] %q\nset -gu status-format[1]", statusRuleExpr)
		} else {
			statusFormatCommands = fmt.Sprintf("set -gu status-format[0]\nset -g status-format[1] %q", statusRuleExpr)
		}
	}
	return fmt.Sprintf(`# >>> MATUGEN THEME START >>>
# Generated by %s/theme apply
set -g status-style "bg=default,fg=%s"
set -g status-position "%s"
set -g status %d
set -g @status-tmux-edge-style "%s"
set -g @status-tmux-left-edge-style "%s"
set -g @status-tmux-bg-left "on"
set -g @status-tmux-bg-right "%s"
set -g @status-tmux-left-bg-session "%s"
set -g @status-tmux-left-bg-prefix "%s"
set -g @status-tmux-left-bg-copy "%s"
set -g @status-tmux-left-session-pad "%s"
set -g status-left "#(\$HOME/.config/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}')%s"
set -g status-right "#(\$HOME/.config/tmux/run-system-widget all)#(\$HOME/.config/tmux/widget-weather)"
%s
set -g window-status-separator ""
set -g window-status-format "%s"
set -g window-status-current-format "%s"
set -g window-status-activity-style "fg=%s,bold"
set -g window-status-bell-style "fg=%s,bold"
setw -g monitor-activity on
set -g visual-activity on
set -g pane-border-style "fg=%s"
set -g pane-active-border-style "fg=%s"
set -g message-style "bg=default,fg=%s"
set -g message-command-style "bg=default,fg=%s"
set -g mode-style "bg=%s,fg=%s"
set -g copy-mode-match-style "bg=%s,fg=%s"
set -g copy-mode-current-match-style "bg=%s,fg=%s,bold"
setw -g clock-mode-colour "%s"
set -g @status-tmux-palette "%s"
set -g @status-tmux-widget-battery "%s"
set -g @status-tmux-widget-cpu "%s"
set -g @status-tmux-widget-ram "%s"
set -g @status-tmux-widget-weather "%s"
set -g @status-tmux-widget-gap-right "%s"
set -g @status-tmux-color-separator "%s"
set -g @status-tmux-color-weather "%s"
set -g @status-tmux-color-charging "%s"
set -g @status-tmux-color-battery-1 "%s"
set -g @status-tmux-color-battery-2 "%s"
set -g @status-tmux-color-battery-3 "%s"
set -g @status-tmux-color-battery-4 "%s"
set -g @status-tmux-color-battery-5 "%s"
set -g @status-tmux-color-battery-6 "%s"
set -g @status-tmux-color-battery-full-1 "%s"
set -g @status-tmux-color-battery-full-2 "%s"
set -g @status-tmux-color-battery-full-3 "%s"
set -g @status-tmux-color-battery-full-4 "%s"
set -g @status-tmux-color-battery-half-1 "%s"
set -g @status-tmux-color-battery-half-2 "%s"
set -g @status-tmux-color-battery-half-3 "%s"
set -g @status-tmux-color-battery-half-4 "%s"
set -g @status-tmux-color-battery-empty-1 "%s"
set -g @status-tmux-color-battery-empty-2 "%s"
set -g @status-tmux-color-battery-empty-3 "%s"
set -g @status-tmux-color-battery-empty-4 "%s"
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
set -g @status-tmux-bg-battery "%s"
set -g @status-tmux-bg-charging "%s"
set -g @status-tmux-bg-cpu "%s"
set -g @status-tmux-bg-ram "%s"
set -g @status-tmux-bg-weather "%s"
set -g @status-tmux-fg-on-accent "%s"
# <<< MATUGEN THEME END <<<
`, tooieConfigDir,
		payload.Foreground,
		statusPosition,
		statusRows,
		edgeStyle,
		leftEdgeStyle,
		bgRight,
		sessionBG,
		prefixBG,
		copyBG,
		leftSessionPad,
		leftGap,
		statusFormatCommands,
		windowStatusFormat,
		windowStatusCurrentFormat,
		attentionFG, attentionFG,
		paneBorderColor, paneActiveBorderColor, payload.Foreground, payload.Foreground,
		modeBG, modeFG,
		matchBG, matchFG,
		currentMatchBG, currentMatchFG,
		payload.Roles["secondary"],
		statusPalette,
		widgetBattery, widgetCPU, widgetRAM, widgetWeather,
		rightGap,
		separatorColor, weatherColor, chargingColor,
		batteryColors[0], batteryColors[1], batteryColors[2], batteryColors[3], batteryColors[4], batteryColors[5],
		batteryFullColors[0], batteryFullColors[1], batteryFullColors[2], batteryFullColors[3],
		batteryHalfColors[0], batteryHalfColors[1], batteryHalfColors[2], batteryHalfColors[3],
		batteryEmptyColors[0], batteryEmptyColors[1], batteryEmptyColors[2], batteryEmptyColors[3],
		cpuColors[0], cpuColors[1], cpuColors[2], cpuColors[3], cpuColors[4], cpuColors[5],
		ramColors[0], ramColors[1], ramColors[2], ramColors[3], ramColors[4], ramColors[5],
		batteryBGSetting,
		chargingBG,
		cpuBG,
		ramBG,
		weatherBG,
		widgetAccentFG,
	)
}

func tmuxGradientRamp(payload computedPayload) []string {
	anchors := []string{
		getRoleOr(payload.Roles, "error", payload.Colors[1]),
		getRoleOr(payload.Roles, "secondary", payload.Colors[6]),
		getRoleOr(payload.Roles, "primary", payload.Colors[4]),
		getRoleOr(payload.Roles, "tertiary", payload.Colors[5]),
		getRoleOr(payload.Roles, "secondary_fixed", payload.Colors[16]),
	}
	ramp := make([]string, 21)
	for i := range ramp {
		p := 0.0
		if len(ramp) > 1 {
			p = float64(i) / float64(len(ramp)-1)
		}
		raw := sampleGradientColor(anchors, p)
		ramp[i] = nonBlackStatusColor(ensureReadableTextColor(payload.Background, raw, payload.Foreground), payload.Foreground)
	}
	return ramp
}

func nonBlackStatusColor(hex, fallback string) string {
	hex = normalizeHexColor(hex)
	if !itheme.IsHexColor(hex) {
		return normalizeHexColor(fallback)
	}
	r, g, b := parseHexColor(hex)
	if maxInt(r, maxInt(g, b)) < 96 {
		// Avoid near-black status text on transparent bars.
		return normalizeHexColor(blendHexColor(fallback, "#ffffff", 0.22))
	}
	return hex
}

func bestTextColorForBackgrounds(preferred, fallback string, minContrast float64, backgrounds ...string) string {
	candidates := []string{
		preferred,
		fallback,
		"#ffffff",
		"#f5f7ff",
		"#111111",
		"#000000",
	}
	for _, bg := range backgrounds {
		candidates = append(candidates, ensureReadableTextColor(bg, preferred, fallback))
		candidates = append(candidates, ensureReadableTextColor(bg, "#ffffff", "#111111"))
	}
	best := normalizeHexColor(fallback)
	bestMin := -1.0
	for _, cand := range candidates {
		cand = normalizeHexColor(cand)
		if !itheme.IsHexColor(cand) {
			continue
		}
		minSeen := 99.0
		validBG := 0
		for _, bg := range backgrounds {
			bg = normalizeHexColor(bg)
			if !itheme.IsHexColor(bg) {
				continue
			}
			validBG++
			r := contrastRatioHex(cand, bg)
			if r < minSeen {
				minSeen = r
			}
		}
		if validBG == 0 {
			return cand
		}
		if minSeen > bestMin {
			bestMin = minSeen
			best = cand
		}
		if minSeen >= minContrast {
			return cand
		}
	}
	return best
}

func ensureBackgroundContrastForText(bg, fg string, minContrast float64) string {
	bg = normalizeHexColor(bg)
	fg = normalizeHexColor(fg)
	if !itheme.IsHexColor(bg) || !itheme.IsHexColor(fg) {
		return bg
	}
	if contrastRatioHex(bg, fg) >= minContrast {
		return bg
	}
	target := "#000000"
	if relativeLuminanceHex(fg) < 0.5 {
		target = "#ffffff"
	}
	best := bg
	bestRatio := contrastRatioHex(bg, fg)
	for i := 1; i <= 14; i++ {
		t := 0.06 * float64(i)
		if t > 0.92 {
			t = 0.92
		}
		cand := normalizeHexColor(blendHexColor(bg, target, t))
		r := contrastRatioHex(cand, fg)
		if r > bestRatio {
			best = cand
			bestRatio = r
		}
		if r >= minContrast {
			return cand
		}
	}
	return best
}

func avoidRedHue(hex string, fallbacks ...string) string {
	hex = normalizeHexColor(hex)
	if itheme.IsHexColor(hex) {
		h := hueFromHex(hex)
		if !(h >= 345 || h < 20) {
			return hex
		}
	}
	for _, fb := range fallbacks {
		fb = normalizeHexColor(fb)
		if !itheme.IsHexColor(fb) {
			continue
		}
		h := hueFromHex(fb)
		if !(h >= 345 || h < 20) {
			return fb
		}
	}
	// Last resort: push toward a cool non-red accent.
	return normalizeHexColor(blendHexColor("#56c8ff", "#9b7dff", 0.35))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func runMatugenImage(bin, wallpaper, mode, schemeType string, sourceColorIndex int) ([]byte, error) {
	args := []string{"image", wallpaper, "-m", mode, "-t", schemeType, "--source-color-index", fmt.Sprintf("%d", sourceColorIndex), "-j", "hex", "--dry-run"}
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("matugen failed for mode=%s scheme=%s idx=%d: %v (%s)", mode, schemeType, sourceColorIndex, err, strings.TrimSpace(string(out)))
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

func wallpaperImageMetrics(path string) (autoDecisionMetrics, bool) {
	f, err := os.Open(path)
	if err != nil {
		return autoDecisionMetrics{}, false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return autoDecisionMetrics{}, false
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return autoDecisionMetrics{}, false
	}
	const target = 64
	stepX := float64(w) / float64(target)
	stepY := float64(h) / float64(target)
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}

	lumas := make([]float64, 0, target*target)
	sats := make([]float64, 0, target*target)
	const hueBins = 24
	hist := make([]float64, hueBins)
	histTotal := 0.0
	grid := make([][]float64, target)
	for y := 0; y < target; y++ {
		grid[y] = make([]float64, target)
		sy := b.Min.Y + int((float64(y)+0.5)*stepY)
		if sy >= b.Max.Y {
			sy = b.Max.Y - 1
		}
		for x := 0; x < target; x++ {
			sx := b.Min.X + int((float64(x)+0.5)*stepX)
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			r, g, bb, _ := img.At(sx, sy).RGBA()
			// RGBA() returns 16-bit.
			r8 := float64(r>>8) / 255.0
			g8 := float64(g>>8) / 255.0
			b8 := float64(bb>>8) / 255.0
			l := 0.2126*r8 + 0.7152*g8 + 0.0722*b8
			maxv := math.Max(r8, math.Max(g8, b8))
			minv := math.Min(r8, math.Min(g8, b8))
			sat := 0.0
			if maxv > 0 {
				sat = (maxv - minv) / maxv
			}
			if sat > 0.12 && l > 0.06 && l < 0.94 {
				h := hueFromRGB(r8, g8, b8)
				bin := int(math.Round((h / 360.0) * float64(hueBins-1)))
				if bin < 0 {
					bin = 0
				}
				if bin >= hueBins {
					bin = hueBins - 1
				}
				weight := sat * (0.55 + 0.45*(1.0-math.Abs(l-0.5)*2.0))
				if weight > 0 {
					hist[bin] += weight
					histTotal += weight
				}
			}
			lumas = append(lumas, l)
			sats = append(sats, sat)
			grid[y][x] = l
		}
	}
	if len(lumas) == 0 {
		return autoDecisionMetrics{}, false
	}
	sum := 0.0
	dark := 0
	bright := 0
	for _, l := range lumas {
		sum += l
		if l < 0.25 {
			dark++
		}
		if l > 0.82 {
			bright++
		}
	}
	sort.Float64s(lumas)
	sort.Float64s(sats)
	mean := sum / float64(len(lumas))
	satSum := 0.0
	for _, s := range sats {
		satSum += s
	}
	meanSat := satSum / float64(len(sats))
	p10 := percentileFromSorted(lumas, 0.10)
	p50 := percentileFromSorted(lumas, 0.50)
	p90 := percentileFromSorted(lumas, 0.90)
	p90Sat := percentileFromSorted(sats, 0.90)

	edgeWeight := 0.0
	edgeSum := 0.0
	for y := 1; y < target-1; y++ {
		for x := 1; x < target-1; x++ {
			gx := math.Abs(grid[y][x+1] - grid[y][x-1])
			gy := math.Abs(grid[y+1][x] - grid[y-1][x])
			grad := gx + gy
			edgeWeight += grad
			edgeSum += grad * grid[y][x]
		}
	}
	edgeLuma := mean
	if edgeWeight > 0 {
		edgeLuma = edgeSum / edgeWeight
	}
	dominantHue := -1.0
	secondaryHue := -1.0
	hueStrength := 0.0
	if histTotal > 0 {
		top1, top2 := 0, 0
		for i := 1; i < len(hist); i++ {
			if hist[i] > hist[top1] {
				top2 = top1
				top1 = i
			} else if i != top1 && hist[i] > hist[top2] {
				top2 = i
			}
		}
		dominantHue = (float64(top1) + 0.5) * (360.0 / float64(hueBins))
		if hist[top2] > 0 && top2 != top1 {
			secondaryHue = (float64(top2) + 0.5) * (360.0 / float64(hueBins))
		}
		hueStrength = clamp01(hist[top1] / histTotal)
	}
	return autoDecisionMetrics{
		MeanLuma:         mean,
		P10:              p10,
		P50:              p50,
		P90:              p90,
		DarkPixelRatio:   float64(dark) / float64(len(lumas)),
		BrightPixelRatio: float64(bright) / float64(len(lumas)),
		EdgeWeightedLuma: edgeLuma,
		MeanSat:          meanSat,
		P90Sat:           p90Sat,
		DominantHue:      dominantHue,
		SecondaryHue:     secondaryHue,
		HueStrength:      hueStrength,
	}, true
}

func percentileFromSorted(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	i := int(math.Floor(pos))
	f := pos - float64(i)
	if i >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	return sorted[i]*(1-f) + sorted[i+1]*f
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

func ternf(ok bool, a, b float64) float64 {
	if ok {
		return a
	}
	return b
}

func hueFromHex(hex string) float64 {
	r, g, b := parseHexColor(hex)
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	return hueFromRGB(rf, gf, bf)
}

func hueFromRGB(rf, gf, bf float64) float64 {
	maxv := math.Max(rf, math.Max(gf, bf))
	minv := math.Min(rf, math.Min(gf, bf))
	delta := maxv - minv
	if delta == 0 {
		return 0
	}
	var h float64
	switch maxv {
	case rf:
		h = math.Mod(((gf - bf) / delta), 6.0)
	case gf:
		h = ((bf-rf)/delta + 2.0)
	default:
		h = ((rf-gf)/delta + 4.0)
	}
	h *= 60.0
	if h < 0 {
		h += 360.0
	}
	return h
}

func hueDistanceDegrees(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func saturateHexColor(hex string, boost float64) string {
	r, g, b := parseHexColor(hex)
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	maxv := math.Max(rf, math.Max(gf, bf))
	minv := math.Min(rf, math.Min(gf, bf))
	light := (maxv + minv) / 2.0
	if maxv == minv {
		return normalizeHexColor(hex)
	}
	var sat float64
	delta := maxv - minv
	if light > 0.5 {
		sat = delta / (2.0 - maxv - minv)
	} else {
		sat = delta / (maxv + minv)
	}
	h := hueFromHex(hex)
	sat = clamp01(sat + boost*(1.0-sat))
	return hslToHex(h, sat, light)
}

func hslToHex(h, s, l float64) string {
	h = math.Mod(h, 360.0)
	if h < 0 {
		h += 360.0
	}
	h /= 360.0
	if s <= 0 {
		v := int(math.Round(l * 255.0))
		return fmt.Sprintf("#%02x%02x%02x", clampInt(v, 0, 255), clampInt(v, 0, 255), clampInt(v, 0, 255))
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r := hue2rgb(p, q, h+1.0/3.0)
	g := hue2rgb(p, q, h)
	b := hue2rgb(p, q, h-1.0/3.0)
	return fmt.Sprintf("#%02x%02x%02x", clampInt(int(math.Round(r*255.0)), 0, 255), clampInt(int(math.Round(g*255.0)), 0, 255), clampInt(int(math.Round(b*255.0)), 0, 255))
}

func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
