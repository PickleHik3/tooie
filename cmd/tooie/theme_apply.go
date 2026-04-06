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
	"sync"
	"time"

	itheme "github.com/PickleHik3/tooie/internal/theme"
)

const (
	themeBackupKeep = 5
)

var matugenResultCache = struct {
	mu sync.Mutex
	m  map[string][]byte
}{
	m: map[string][]byte{},
}

const matugenRolesTemplate = `{
  "colors": {
    "dark": {
      "background": "{{colors.background.dark.hex}}",
      "surface": "{{colors.surface.dark.hex}}",
      "surface_container": "{{colors.surface_container.dark.hex}}",
      "surface_container_high": "{{colors.surface_container_high.dark.hex}}",
      "surface_dim": "{{colors.surface_dim.dark.hex}}",
      "surface_bright": "{{colors.surface_bright.dark.hex}}",
      "surface_variant": "{{colors.surface_variant.dark.hex}}",
      "on_background": "{{colors.on_background.dark.hex}}",
      "on_surface": "{{colors.on_surface.dark.hex}}",
      "on_surface_variant": "{{colors.on_surface_variant.dark.hex}}",
      "outline": "{{colors.outline.dark.hex}}",
      "outline_variant": "{{colors.outline_variant.dark.hex}}",
      "primary": "{{colors.primary.dark.hex}}",
      "secondary": "{{colors.secondary.dark.hex}}",
      "tertiary": "{{colors.tertiary.dark.hex}}",
      "error": "{{colors.error.dark.hex}}",
      "secondary_fixed": "{{colors.secondary_fixed.dark.hex}}",
      "tertiary_fixed": "{{colors.tertiary_fixed.dark.hex}}"
    },
    "light": {
      "background": "{{colors.background.light.hex}}",
      "surface": "{{colors.surface.light.hex}}",
      "surface_container": "{{colors.surface_container.light.hex}}",
      "surface_container_high": "{{colors.surface_container_high.light.hex}}",
      "surface_dim": "{{colors.surface_dim.light.hex}}",
      "surface_bright": "{{colors.surface_bright.light.hex}}",
      "surface_variant": "{{colors.surface_variant.light.hex}}",
      "on_background": "{{colors.on_background.light.hex}}",
      "on_surface": "{{colors.on_surface.light.hex}}",
      "on_surface_variant": "{{colors.on_surface_variant.light.hex}}",
      "outline": "{{colors.outline.light.hex}}",
      "outline_variant": "{{colors.outline_variant.light.hex}}",
      "primary": "{{colors.primary.light.hex}}",
      "secondary": "{{colors.secondary.light.hex}}",
      "tertiary": "{{colors.tertiary.light.hex}}",
      "error": "{{colors.error.light.hex}}",
      "secondary_fixed": "{{colors.secondary_fixed.light.hex}}",
      "tertiary_fixed": "{{colors.tertiary_fixed.light.hex}}"
    }
  }
}
`

type themeApplyConfig struct {
	Mode                 string
	SchemeType           string
	ThemeSource          string
	StarshipPrompt       string
	PresetFamily         string
	PresetVariant        string
	MatugenBin           string
	StatusPalette        string
	StatusTheme          string
	StatusPosition       string
	StatusLayout         string
	StatusSeparator      string
	StyleFamily          string
	Profile              string
	ExtractSourceIndex   int
	ExtractPrefer        string
	ExtractFallbackColor string
	ExtractResizeFilter  string
	TextColor            string
	CursorColor          string
	PreviewOnly          bool
	ReuseBackupID        string
	AnsiRed              string
	AnsiGreen            string
	AnsiYellow           string
	AnsiBlue             string
	AnsiMagenta          string
	AnsiCyan             string
	WidgetBattery        bool
	WidgetCPU            bool
	WidgetRAM            bool
	WidgetWeather        bool
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
	meta["type"] = schemeTypeLabel(cfg.SchemeType)
	meta["matugen_bin"] = cfg.MatugenBin
	meta["text_color_override"] = strings.TrimSpace(cfg.TextColor)
	meta["cursor_color_override"] = strings.TrimSpace(cfg.CursorColor)
	meta["status_palette"] = cfg.StatusPalette
	meta["status_theme"] = cfg.StatusTheme
	meta["starship_prompt"] = normalizeStarshipPrompt(cfg.StarshipPrompt)
	meta["status_position"] = cfg.StatusPosition
	meta["status_layout"] = cfg.StatusLayout
	meta["status_separator"] = cfg.StatusSeparator
	meta["extract_source_index"] = fmt.Sprintf("%d", cfg.ExtractSourceIndex)
	meta["extract_prefer"] = strings.TrimSpace(cfg.ExtractPrefer)
	meta["extract_fallback_color"] = strings.TrimSpace(cfg.ExtractFallbackColor)
	meta["extract_resize_filter"] = strings.TrimSpace(cfg.ExtractResizeFilter)
	meta["widget_battery"] = onOffFlag(cfg.WidgetBattery)
	meta["widget_cpu"] = onOffFlag(cfg.WidgetCPU)
	meta["widget_ram"] = onOffFlag(cfg.WidgetRAM)
	meta["widget_weather"] = onOffFlag(cfg.WidgetWeather)
	if cfg.ThemeSource != "preset" {
		family := canonicalProfile(cfg.StyleFamily)
		meta["style_family"] = family // backward compatibility for existing readers
		meta["style_family_version"] = "2"
		meta["profile"] = family // backward compatibility for existing readers
		meta["extraction_preset"] = family
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
		parsed, err := itheme.ParseMatugenColorsForMode(matugenRaw, effectiveMode)
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
	settings, _ := loadTooieSettings()
	platformProfile := normalizePlatformProfile(settings.Platform.Profile)
	computed, err := itheme.Compute(roles, itheme.Options{
		Source:         itheme.Source(cfg.ThemeSource),
		Mode:           effectiveMode,
		Platform:       platformProfile,
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
	out.Meta["type"] = schemeTypeLabel(cfg.SchemeType)
	out.Meta["scheme_type"] = normalizeSchemeType(cfg.SchemeType)
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
		fallback := normalizeSchemeType(defaultPaletteType)
		if normalizeSchemeType(cfg.SchemeType) != fallback {
			cfg2 := cfg
			cfg2.SchemeType = fallback
			candidates, err = collectMatugenCandidates(cfg2, wallpaper, mode, metrics)
			if err == nil {
				meta["palette_fallback"] = schemeTypeLabel(fallback)
				meta["palette_requested"] = schemeTypeLabel(cfg.SchemeType)
			}
		}
		if err != nil {
			return nil, "", nil, err
		}
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
	meta["auto_selected_scheme"] = schemeTypeLabel(best.Scheme)
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
	indices := sourceIndexCandidates(cfg)
	type combo struct {
		mode   string
		scheme string
		index  int
	}
	jobs := make([]combo, 0, len(modes)*len(schemes)*len(indices))
	for _, m := range modes {
		for _, s := range schemes {
			for _, idx := range indices {
				jobs = append(jobs, combo{mode: m, scheme: s, index: idx})
			}
		}
	}
	candidates := make([]matugenCandidate, 0, len(jobs))
	var lastErr error
	var mu sync.Mutex
	var wg sync.WaitGroup
	parallel := 4
	if len(jobs) < parallel {
		parallel = len(jobs)
	}
	if parallel <= 0 {
		return nil, fmt.Errorf("no matugen candidates requested")
	}
	sem := make(chan struct{}, parallel)
	for _, job := range jobs {
		job := job
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			raw, err := runMatugenImage(cfg.MatugenBin, wallpaper, job.mode, job.scheme, job.index, cfg.ExtractPrefer, cfg.ExtractFallbackColor, cfg.ExtractResizeFilter)
			if err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
				return
			}
			roles, err := itheme.ParseMatugenColorsForMode(raw, job.mode)
			if err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
				return
			}
			readability := readabilityScore(roles)
			c := matugenCandidate{
				Raw:         raw,
				Mode:        job.mode,
				Scheme:      job.scheme,
				SourceIndex: job.index,
				Roles:       roles,
				Readability: readability,
			}
			c.Score = scoreCandidate(c, metrics)
			mu.Lock()
			candidates = append(candidates, c)
			mu.Unlock()
		}()
	}
	wg.Wait()
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
	sceneMode, _ := autoSceneDecision(metrics, mode)
	return []string{sceneMode}
}

func autoSceneDecision(metrics autoDecisionMetrics, mode string) (string, string) {
	if mode != "auto" {
		return mode, "explicit-" + mode
	}
	if darkDominantScene(metrics) {
		return "dark", "forced-dark"
	}
	if brightDominantScene(metrics) {
		return "light", "forced-light"
	}
	return "dark", "fallback-dark"
}

func pickSchemeCandidates(schemeType string) []string {
	s := normalizeSchemeType(schemeType)
	if s != "" {
		return []string{s}
	}
	return []string{
		"scheme-tonal-spot",
		"scheme-expressive",
		"scheme-fidelity",
		"scheme-content",
		"scheme-vibrant",
		"scheme-neutral",
		"scheme-rainbow",
		"scheme-fruit-salad",
		"scheme-cmf",
	}
}

func sourceIndexCandidates(cfg themeApplyConfig) []int {
	if cfg.ExtractSourceIndex >= 0 {
		return []int{cfg.ExtractSourceIndex}
	}
	if strings.TrimSpace(cfg.ExtractPrefer) != "" {
		return []int{-1}
	}
	return []int{0, 1, 2, 3, 4}
}

func scoreCandidate(c matugenCandidate, metrics autoDecisionMetrics) float64 {
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
	mutedContrast := contrastRatioHex(getRoleOr(c.Roles, "on_surface_variant", fg), bg)
	if mutedContrast < 3.2 {
		return -700 + mutedContrast
	}
	outlineContrast := contrastRatioHex(getRoleOr(c.Roles, "outline", fg), bg)
	if outlineContrast < 2.8 {
		return -650 + outlineContrast
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
		if metrics.P90Sat > 0.45 {
			sourcePenalty *= 0.65
		}
	}
	schemeBias := 0.0
	switch c.Scheme {
	case "scheme-expressive":
		schemeBias = 0.22 + (0.35 * clamp01(metrics.P90Sat-0.32))
	case "scheme-fidelity", "scheme-content":
		schemeBias = 0.16 + (0.28 * clamp01(metrics.P90Sat-0.28))
	case "scheme-vibrant":
		schemeBias = 0.25 + (0.24 * clamp01(metrics.P90Sat-0.30))
	case "scheme-neutral":
		schemeBias = 0.14 + (0.12 * clamp01(0.42-metrics.P90Sat))
	case "scheme-rainbow":
		schemeBias = 0.20 + (0.20 * clamp01(metrics.P90Sat-0.24))
	case "scheme-fruit-salad":
		schemeBias = 0.18 + (0.20 * clamp01(metrics.P90Sat-0.24))
	case "scheme-cmf":
		schemeBias = 0.22 + (0.22 * clamp01(metrics.P90Sat-0.20))
	case "scheme-tonal-spot":
		schemeBias = 0.18 + (0.25 * clamp01(0.40-metrics.P90Sat))
	}
	affinityWeight := 1.0
	return (c.Readability * 2.05) + (fgContrast * 1.05) + (accentMin * 1.00) + (mutedContrast * 0.70) + (outlineContrast * 0.45) + (ansiSep * 0.95) + (wallpaperAffinity * affinityWeight) + sceneFit + schemeBias - sourcePenalty - modePenalty
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
	return metrics.DarkPixelRatio >= 0.45 ||
		metrics.P50 <= 0.38 ||
		(metrics.MeanLuma <= 0.35 && metrics.BrightPixelRatio < 0.08)
}

func brightDominantScene(metrics autoDecisionMetrics) bool {
	return metrics.BrightPixelRatio >= 0.28 && metrics.P50 >= 0.52
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
		Mode:               "dark",
		SchemeType:         "tonal-spot",
		ThemeSource:        "wallpaper",
		PresetFamily:       "tokyo-night",
		PresetVariant:      "night",
		StatusPalette:      "default",
		StatusTheme:        "default",
		StarshipPrompt:     defaultStarship,
		StatusPosition:     "top",
		StatusLayout:       "two-line",
		StatusSeparator:    "on",
		StyleFamily:        "auto",
		Profile:            "auto",
		ExtractSourceIndex: -1,
		WidgetBattery:      true,
		WidgetCPU:          true,
		WidgetRAM:          true,
		WidgetWeather:      true,
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
	fs.StringVar(&cfg.StarshipPrompt, "starship-prompt", cfg.StarshipPrompt, "")
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
	fs.IntVar(&cfg.ExtractSourceIndex, "source-color-index", cfg.ExtractSourceIndex, "")
	fs.StringVar(&cfg.ExtractPrefer, "prefer", cfg.ExtractPrefer, "")
	fs.StringVar(&cfg.ExtractFallbackColor, "fallback-color", cfg.ExtractFallbackColor, "")
	fs.StringVar(&cfg.ExtractResizeFilter, "resize-filter", cfg.ExtractResizeFilter, "")
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
	rawMode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if rawMode == "auto" {
		return cfg, fmt.Errorf("invalid --mode value: auto (only dark/light are supported)")
	}
	cfg.Mode = canonicalMode(cfg.Mode)
	cfg.ThemeSource = strings.TrimSpace(strings.ToLower(cfg.ThemeSource))
	if cfg.ThemeSource != "wallpaper" && cfg.ThemeSource != "preset" {
		return cfg, fmt.Errorf("invalid theme source: %s", cfg.ThemeSource)
	}
	cfg.StarshipPrompt = normalizeStarshipPrompt(cfg.StarshipPrompt)
	rawSchemeType := strings.TrimSpace(cfg.SchemeType)
	cfg.SchemeType = normalizeSchemeType(cfg.SchemeType)
	if cfg.SchemeType == "" {
		return cfg, fmt.Errorf("invalid --type value: %s", rawSchemeType)
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
	sourceIndexSet := false
	preferSet := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "style-family":
			styleFamilySet = true
		case "source-color-index":
			sourceIndexSet = true
		case "prefer":
			preferSet = true
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
	if cfg.ExtractSourceIndex < -1 || cfg.ExtractSourceIndex > 4 {
		return cfg, fmt.Errorf("invalid --source-color-index value: %d (expected -1..4)", cfg.ExtractSourceIndex)
	}
	rawPrefer := strings.TrimSpace(cfg.ExtractPrefer)
	cfg.ExtractPrefer = normalizeMatugenPrefer(cfg.ExtractPrefer)
	if rawPrefer != "" && cfg.ExtractPrefer == "" {
		return cfg, fmt.Errorf("invalid --prefer value: %s", rawPrefer)
	}
	rawFilter := strings.TrimSpace(cfg.ExtractResizeFilter)
	cfg.ExtractResizeFilter = normalizeResizeFilter(cfg.ExtractResizeFilter)
	if rawFilter != "" && cfg.ExtractResizeFilter == "" {
		return cfg, fmt.Errorf("invalid --resize-filter value: %s", rawFilter)
	}
	cfg.ExtractFallbackColor = strings.TrimSpace(cfg.ExtractFallbackColor)
	if cfg.ExtractFallbackColor != "" && !itheme.IsHexColor(cfg.ExtractFallbackColor) {
		return cfg, fmt.Errorf("invalid --fallback-color value: %s (expected #rrggbb)", cfg.ExtractFallbackColor)
	}
	if !sourceIndexSet && !preferSet {
		presetIndex, presetPrefer := extractionPresetSelection(cfg.StyleFamily)
		cfg.ExtractSourceIndex = presetIndex
		cfg.ExtractPrefer = presetPrefer
	}
	if cfg.ExtractPrefer == "closest-to-fallback" && cfg.ExtractFallbackColor == "" {
		cfg.ExtractFallbackColor = "#6750A4"
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

func normalizeSchemeType(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.TrimPrefix(v, "scheme-")
	if v == "monochrome" {
		v = "neutral"
	}
	switch v {
	case "tonal-spot", "expressive", "fidelity", "content", "vibrant", "neutral", "rainbow", "fruit-salad", "cmf":
		return "scheme-" + v
	default:
		return ""
	}
}

func schemeTypeLabel(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	return strings.TrimPrefix(v, "scheme-")
}

type statusSchemeTuning struct {
	Lift           float64
	AccentSat      float64
	ChipMixA       float64
	ChipMixB       float64
	ChipMixC       float64
	WidgetMix      float64
	WindowActive   float64
	WindowInactive float64
	HueShiftA      float64
	HueShiftB      float64
	HueShiftC      float64
	RoleDesat      float64
}

func statusSchemeTuningFor(scheme, mode string) statusSchemeTuning {
	t := statusSchemeTuning{
		Lift:           0.24,
		AccentSat:      0.18,
		ChipMixA:       0.62,
		ChipMixB:       0.58,
		ChipMixC:       0.60,
		WidgetMix:      0.64,
		WindowActive:   0.36,
		WindowInactive: 0.62,
		HueShiftA:      0,
		HueShiftB:      0,
		HueShiftC:      0,
		RoleDesat:      0,
	}
	if mode == "light" {
		t.Lift = 0.14
		t.AccentSat = 0.14
	}
	switch normalizeSchemeType(scheme) {
	case "scheme-tonal-spot":
		t.AccentSat -= 0.06
		t.ChipMixA -= 0.10
		t.ChipMixB -= 0.10
		t.ChipMixC -= 0.10
		t.WidgetMix -= 0.08
		t.WindowInactive += 0.08
		t.RoleDesat += 0.14
	case "scheme-expressive":
		t.AccentSat += 0.02
		t.ChipMixA += 0.06
		t.ChipMixB += 0.04
		t.ChipMixC += 0.07
		t.WidgetMix += 0.06
		t.WindowActive += 0.05
		t.HueShiftA = 26
		t.HueShiftB = -18
		t.HueShiftC = 36
	case "scheme-rainbow":
		t.AccentSat += 0.10
		t.ChipMixA += 0.12
		t.ChipMixB += 0.12
		t.ChipMixC += 0.12
		t.WidgetMix += 0.10
		t.WindowActive += 0.08
		t.HueShiftA = 120
		t.HueShiftB = 205
		t.HueShiftC = 285
	case "scheme-fruit-salad":
		t.AccentSat += 0.08
		t.ChipMixA += 0.10
		t.ChipMixB += 0.10
		t.ChipMixC += 0.10
		t.WidgetMix += 0.08
		t.WindowActive += 0.08
		t.HueShiftA = 84
		t.HueShiftB = 164
		t.HueShiftC = 244
	case "scheme-vibrant":
		t.AccentSat += 0.16
		t.ChipMixA += 0.12
		t.ChipMixB += 0.12
		t.ChipMixC += 0.12
		t.WidgetMix += 0.10
		t.WindowActive += 0.08
	case "scheme-fidelity", "scheme-content":
		t.AccentSat += 0.02
		t.ChipMixA -= 0.01
		t.ChipMixB -= 0.01
		t.ChipMixC -= 0.01
		t.WidgetMix += 0.02
	case "scheme-cmf":
		t.AccentSat += 0.08
		t.ChipMixA += 0.05
		t.ChipMixB += 0.03
		t.ChipMixC += 0.10
		t.WidgetMix += 0.07
		t.WindowActive += 0.06
		t.HueShiftC = 20
	case "scheme-neutral":
		t.AccentSat -= 0.16
		t.ChipMixA -= 0.14
		t.ChipMixB -= 0.14
		t.ChipMixC -= 0.14
		t.WidgetMix -= 0.14
		t.WindowActive -= 0.10
		t.WindowInactive += 0.12
		t.RoleDesat = 0.74
	}
	t.Lift = clamp01(t.Lift)
	t.AccentSat = clamp01(t.AccentSat)
	t.ChipMixA = clamp01(t.ChipMixA)
	t.ChipMixB = clamp01(t.ChipMixB)
	t.ChipMixC = clamp01(t.ChipMixC)
	t.WidgetMix = clamp01(t.WidgetMix)
	t.WindowActive = clamp01(t.WindowActive)
	t.WindowInactive = clamp01(t.WindowInactive)
	t.RoleDesat = clamp01(t.RoleDesat)
	return t
}

func normalizeMatugenPrefer(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "none":
		return ""
	case "darkness":
		return "darkness"
	case "lightness":
		return "lightness"
	case "saturation":
		return "saturation"
	case "less-saturation", "less_saturation":
		return "less-saturation"
	case "value":
		return "value"
	case "closest-to-fallback", "closest_to_fallback":
		return "closest-to-fallback"
	default:
		return ""
	}
}

func normalizeResizeFilter(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "default":
		return ""
	case "nearest":
		return "nearest"
	case "triangle":
		return "triangle"
	case "catmull-rom", "catmull_rom":
		return "catmull-rom"
	case "gaussian":
		return "gaussian"
	case "lanczos3":
		return "lanczos3"
	default:
		return ""
	}
}

func extractionPresetSelection(preset string) (int, string) {
	switch canonicalProfile(preset) {
	case "source-0":
		return 0, ""
	case "source-1":
		return 1, ""
	case "source-2":
		return 2, ""
	case "source-3":
		return 3, ""
	case "source-4":
		return 4, ""
	case "prefer-darkness":
		return -1, "darkness"
	case "prefer-lightness":
		return -1, "lightness"
	case "prefer-saturation":
		return -1, "saturation"
	case "prefer-less-saturation":
		return -1, "less-saturation"
	case "prefer-value":
		return -1, "value"
	case "prefer-closest-fallback":
		return -1, "closest-to-fallback"
	default:
		return 0, ""
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
	tmuxConf := managedTmuxConfPath()
	peaclockCfg := managedPeaclockPath()
	starshipCfg := managedStarshipPath()
	settings, _ := loadTooieSettings()
	profile := normalizePlatformProfile(settings.Platform.Profile)

	if profile == "linux" {
		_ = writeApplyProgress("Writing Ghostty theme", 0.42)
		if err := applyLinuxGhosttyTheme(payload, backupDir); err != nil {
			return err
		}
	} else {
		_ = writeApplyProgress("Writing Termux colors", 0.42)
		if err := applyTermuxColors(payload, backupDir, settings.Modules.TermuxAppearance); err != nil {
			return err
		}
	}

	_ = writeApplyProgress("Writing tmux theme", 0.56)
	_ = syncManagedTmuxRuntimeFilesFromRepo()
	_ = syncManagedFishConfigFromRepo()
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

	if settings.Modules.StarshipMode == "themed" {
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
	}

	metaPath := filepath.Join(backupDir, "meta.env")
	f, err := os.OpenFile(metaPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		_, _ = f.WriteString("peaclock_themed=true\n")
		if settings.Modules.StarshipMode == "themed" {
			_, _ = f.WriteString("starship_themed=true\n")
		}
		_ = f.Close()
	}

	_ = writeApplyProgress("Reloading shell surfaces", 0.94)
	if profile != "linux" {
		if _, err := exec.LookPath("termux-reload-settings"); err == nil {
			_ = exec.Command("termux-reload-settings").Run()
		}
	}
	if _, err := exec.LookPath("tmux"); err == nil {
		_ = exec.Command("tmux", "source-file", filepath.Join(homeDir, ".tmux.conf")).Run()
		_ = exec.Command("tmux", "set-option", "-g", "base-index", "1").Run()
		_ = exec.Command("tmux", "set-window-option", "-g", "pane-base-index", "1").Run()
		_ = exec.Command("tmux", "set-option", "-g", "renumber-windows", "on").Run()
	}
	return nil
}

func applyTermuxColors(payload computedPayload, backupDir string, syncLive bool) error {
	termuxColors := managedTermuxFilePath("colors.properties")
	if err := os.MkdirAll(filepath.Dir(termuxColors), 0o755); err != nil {
		return err
	}
	if raw, err := os.ReadFile(termuxColors); err == nil {
		_ = os.WriteFile(filepath.Join(backupDir, "colors.properties.bak"), raw, 0o644)
	}
	rendered := []byte(renderColorsProperties(payload))
	if err := os.WriteFile(termuxColors, rendered, 0o644); err != nil {
		return err
	}
	if !syncLive {
		return nil
	}
	for _, dst := range []string{
		filepath.Join(homeDir, ".termux", "colors.properties"),
		filepath.Join(homeDir, ".config", "termux", "colors.properties"),
	} {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, rendered, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func applyLinuxGhosttyTheme(payload computedPayload, backupDir string) error {
	managedTheme := managedGhosttyThemePath()
	if err := os.MkdirAll(filepath.Dir(managedTheme), 0o755); err != nil {
		return err
	}
	if raw, err := os.ReadFile(managedTheme); err == nil {
		_ = os.WriteFile(filepath.Join(backupDir, "ghostty.theme.bak"), raw, 0o644)
	}
	if err := os.WriteFile(managedTheme, []byte(renderGhosttyTheme(payload)), 0o644); err != nil {
		return err
	}

	userGhosttyConfig := filepath.Join(homeDir, ".config", "ghostty", "config")
	if err := ensureFileWithDirs(userGhosttyConfig); err != nil {
		return err
	}
	if err := backupIfExists(userGhosttyConfig, filepath.Join(backupDir, "ghostty.config.bak")); err != nil {
		return err
	}
	block := fmt.Sprintf(`# >>> TOOIE GHOSTTY THEME START >>>
config-file = %s
# <<< TOOIE GHOSTTY THEME END <<<`, managedTheme)
	return replaceBlock(
		userGhosttyConfig,
		"# >>> TOOIE GHOSTTY THEME START >>>",
		"# <<< TOOIE GHOSTTY THEME END <<<",
		block,
	)
}

func managedGhosttyThemePath() string {
	return filepath.Join(managedConfigsDir(), "ghostty", "dank-theme.conf")
}

func renderGhosttyTheme(payload computedPayload) string {
	selectionBG := getRoleOr(payload.Roles, "primary_container", blendHexColor(payload.Background, payload.Foreground, 0.20))
	selectionFG := getRoleOr(payload.Roles, "on_surface", payload.Foreground)
	cursor := getRoleOr(payload.Roles, "primary", payload.Cursor)
	return fmt.Sprintf(`# Generated by %s/theme apply
background = %s
foreground = %s
cursor-color = %s
selection-background = %s
selection-foreground = %s

palette = 0=%s
palette = 1=%s
palette = 2=%s
palette = 3=%s
palette = 4=%s
palette = 5=%s
palette = 6=%s
palette = 7=%s
palette = 8=%s
palette = 9=%s
palette = 10=%s
palette = 11=%s
palette = 12=%s
palette = 13=%s
palette = 14=%s
palette = 15=%s
`, tooieConfigDir,
		payload.Background,
		payload.Foreground,
		cursor,
		selectionBG,
		selectionFG,
		colorSlot(payload.Colors, 0, "#1a1b26"),
		colorSlot(payload.Colors, 1, "#f7768e"),
		colorSlot(payload.Colors, 2, "#73daca"),
		colorSlot(payload.Colors, 3, "#e0af68"),
		colorSlot(payload.Colors, 4, "#7aa2f7"),
		colorSlot(payload.Colors, 5, "#bb9af7"),
		colorSlot(payload.Colors, 6, "#7dcfff"),
		colorSlot(payload.Colors, 7, "#c0caf5"),
		colorSlot(payload.Colors, 8, "#414868"),
		colorSlot(payload.Colors, 9, "#f7768e"),
		colorSlot(payload.Colors, 10, "#73daca"),
		colorSlot(payload.Colors, 11, "#e0af68"),
		colorSlot(payload.Colors, 12, "#7aa2f7"),
		colorSlot(payload.Colors, 13, "#bb9af7"),
		colorSlot(payload.Colors, 14, "#7dcfff"),
		colorSlot(payload.Colors, 15, "#c0caf5"),
	)
}

func colorSlot(colors map[int]string, idx int, fallback string) string {
	if v := strings.TrimSpace(colors[idx]); v != "" {
		return v
	}
	return fallback
}

func syncManagedTmuxRuntimeFilesFromRepo() error {
	if err := os.MkdirAll(managedTmuxDir(), 0o755); err != nil {
		return err
	}
	files := []string{
		"helpers.sh",
		"run-system-widget",
		"system-widgets",
		"widget-left",
		"widget-weather",
		"widget-cpu",
		"widget-ram",
		"widget-battery",
	}
	for _, name := range files {
		src, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "tmux", name))
		if err != nil {
			// Applying a theme should still succeed from installed-only environments.
			continue
		}
		dst := filepath.Join(managedTmuxDir(), name)
		if err := copyFile(src, dst, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func syncManagedFishConfigFromRepo() error {
	src, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "fish", "config.fish"))
	if err != nil {
		// Applying a theme should still succeed from installed-only environments.
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(managedFishConfigPath()), 0o755); err != nil {
		return err
	}
	return copyFile(src, managedFishConfigPath(), 0o644)
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
	schemeKey := normalizeSchemeType(strings.TrimSpace(payload.Meta["scheme_type"]))
	if schemeKey == "" {
		schemeKey = normalizeSchemeType(strings.TrimSpace(payload.Meta["type"]))
	}
	if schemeKey == "" {
		schemeKey = normalizeSchemeType(strings.TrimSpace(payload.Meta["auto_selected_scheme"]))
	}
	primaryRole := getRoleOr(payload.Roles, "primary", tmuxRamp[8])
	secondaryRole := getRoleOr(payload.Roles, "secondary", tmuxRamp[10])
	tertiaryRole := getRoleOr(payload.Roles, "tertiary", tmuxRamp[16])
	schemeName := strings.TrimSpace(payload.Meta["scheme_type"])
	if schemeName == "" {
		schemeName = strings.TrimSpace(payload.Meta["type"])
	}
	if schemeName == "" {
		schemeName = strings.TrimSpace(payload.Meta["auto_selected_scheme"])
	}
	tuning := statusSchemeTuningFor(schemeName, payload.EffectiveMode)
	seedHue := hueFromHex(primaryRole)
	if tuning.HueShiftA != 0 {
		primaryRole = shiftHexHue(primaryRole, tuning.HueShiftA)
	}
	if tuning.HueShiftB != 0 {
		secondaryRole = shiftHexHue(secondaryRole, tuning.HueShiftB)
	}
	if tuning.HueShiftC != 0 {
		tertiaryRole = shiftHexHue(tertiaryRole, tuning.HueShiftC)
	}
	if schemeKey == "scheme-rainbow" || schemeKey == "scheme-fruit-salad" {
		primaryRole = pushHueAway(primaryRole, seedHue, 44, 72)
		secondaryRole = pushHueAway(secondaryRole, seedHue, 44, 72)
		tertiaryRole = pushHueAway(tertiaryRole, seedHue, 44, 72)
	}
	if tuning.RoleDesat > 0 {
		primaryRole = saturateHexColor(primaryRole, -tuning.RoleDesat)
		secondaryRole = saturateHexColor(secondaryRole, -tuning.RoleDesat)
		tertiaryRole = saturateHexColor(tertiaryRole, -tuning.RoleDesat)
	}
	primaryContainerRole := getRoleOr(payload.Roles, "primary_container", blendHexColor(primaryRole, payload.Background, 0.35))
	secondaryContainerRole := getRoleOr(payload.Roles, "secondary_container", blendHexColor(secondaryRole, payload.Background, 0.35))
	tertiaryContainerRole := getRoleOr(payload.Roles, "tertiary_container", blendHexColor(tertiaryRole, payload.Background, 0.35))
	primaryContainerRole = saturateHexColor(blendHexColor(primaryContainerRole, primaryRole, 0.24), tuning.AccentSat*0.5)
	secondaryContainerRole = saturateHexColor(blendHexColor(secondaryContainerRole, secondaryRole, 0.24), tuning.AccentSat*0.5)
	tertiaryContainerRole = saturateHexColor(blendHexColor(tertiaryContainerRole, tertiaryRole, 0.24), tuning.AccentSat*0.5)
	statusBaseBG := nonBlackStatusColor(getRoleOr(payload.Roles, "surface_container", blendHexColor(payload.Background, payload.Foreground, 0.12)), payload.Foreground)
	statusElevatedBG := nonBlackStatusColor(getRoleOr(payload.Roles, "surface_container_high", blendHexColor(statusBaseBG, payload.Foreground, 0.10)), payload.Foreground)
	statusHighestBG := nonBlackStatusColor(getRoleOr(payload.Roles, "surface_container_highest", blendHexColor(statusElevatedBG, payload.Foreground, 0.12)), payload.Foreground)
	statusLift := tuning.Lift
	statusAccentA := nonBlackStatusColor(saturateHexColor(blendHexColor(primaryContainerRole, statusElevatedBG, 0.18), tuning.AccentSat), payload.Foreground)
	statusAccentB := nonBlackStatusColor(saturateHexColor(blendHexColor(secondaryContainerRole, statusElevatedBG, 0.20), tuning.AccentSat), payload.Foreground)
	statusAccentC := nonBlackStatusColor(saturateHexColor(blendHexColor(tertiaryContainerRole, statusElevatedBG, 0.22), tuning.AccentSat), payload.Foreground)
	statusAccentD := nonBlackStatusColor(saturateHexColor(blendHexColor(statusAccentA, statusAccentC, 0.52), 0.14), payload.Foreground)
	sessionBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(primaryContainerRole, statusElevatedBG, 0.34), tuning.AccentSat+0.14), statusLift*1.05), payload.Foreground)
	prefixBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(secondaryContainerRole, statusElevatedBG, 0.34), tuning.AccentSat+0.14), statusLift*1.05), payload.Foreground)
	copyBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(tertiaryContainerRole, statusElevatedBG, 0.34), tuning.AccentSat+0.14), statusLift*1.05), payload.Foreground)
	leftChipBGs := diversifyAdjacentStatusColors([]string{sessionBG, prefixBG, copyBG}, payload.Background, 24.0, 3.6)
	sessionBG, prefixBG, copyBG = leftChipBGs[0], leftChipBGs[1], leftChipBGs[2]
	modeBG := nonBlackStatusColor(blendHexColor(statusHighestBG, payload.Background, 0.35), statusElevatedBG)
	modeFG := ensureReadableTextColor(modeBG, payload.Background, payload.Foreground)
	matchBG := nonBlackStatusColor(blendHexColor(statusAccentA, statusHighestBG, 0.18), modeBG)
	matchFG := ensureReadableTextColor(matchBG, payload.Background, payload.Foreground)
	currentMatchBG := nonBlackStatusColor(blendHexColor(statusAccentB, statusHighestBG, 0.16), matchBG)
	currentMatchFG := ensureReadableTextColor(currentMatchBG, payload.Background, payload.Foreground)
	windowInactiveSeed := blendHexColor(
		getRoleOr(payload.Roles, "surface_variant", statusBaseBG),
		blendHexColor(tertiaryContainerRole, primaryContainerRole, 0.58),
		0.62,
	)
	windowInactiveBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(statusBaseBG, windowInactiveSeed, 0.54), -0.34), statusLift*0.24), statusBaseBG)
	windowInactiveBG = nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(windowInactiveBG, statusBaseBG, tuning.WindowInactive), -0.28), statusLift*0.14), statusBaseBG)
	inactiveInkBase := saturateHexColor(
		blendHexColor(
			blendHexColor(getRoleOr(payload.Roles, "tertiary_fixed_dim", getRoleOr(payload.Roles, "text_muted", payload.Foreground)), getRoleOr(payload.Roles, "primary_fixed_dim", getRoleOr(payload.Roles, "text_muted", payload.Foreground)), 0.56),
			"#e4d8f7",
			0.22,
		),
		-0.10,
	)
	inactiveInkSeed := saturateHexColor(
		blendHexColor(
			inactiveInkBase,
			getRoleOr(payload.Roles, "text_muted", payload.Foreground),
			0.54,
		),
		-0.18,
	)
	windowInactiveFG := nonBlackStatusColorForBG(
		preferChromaticTextColor(
			ensureReadableTextColor(windowInactiveBG, inactiveInkSeed, inactiveInkBase),
			inactiveInkBase,
			windowInactiveBG,
			3.3,
		),
		inactiveInkBase,
		windowInactiveBG,
		3.3,
	)
	windowActiveBG := nonBlackStatusColor(
		avoidRedHue(
			blendHexColor(blendHexColor(statusAccentA, statusAccentD, tuning.WindowActive), windowInactiveBG, 0.22),
			tmuxRamp[9],
			tmuxRamp[7],
			payload.Foreground,
		),
		payload.Foreground,
	)
	leftBandBGs := diversifyAdjacentStatusColors([]string{copyBG, windowInactiveBG, windowActiveBG}, payload.Background, 24.0, 3.6)
	copyBG, windowInactiveBG, windowActiveBG = leftBandBGs[0], leftBandBGs[1], leftBandBGs[2]
	activeInkSeed := saturateHexColor(
		blendHexColor(
			blendHexColor(getRoleOr(payload.Roles, "primary", payload.Foreground), getRoleOr(payload.Roles, "tertiary", payload.Foreground), 0.44),
			"#111118",
			0.38,
		),
		0.24,
	)
	windowActiveFG := nonBlackStatusColorForBG(
		preferChromaticTextColor(ensureReadableTextColor(windowActiveBG, activeInkSeed, getRoleOr(payload.Roles, "on_primary", payload.Foreground)), activeInkSeed, windowActiveBG, 4.6),
		activeInkSeed,
		windowActiveBG,
		4.6,
	)
	alertColor := ensureReadableTextColor(payload.Background, "#ffd75f", payload.Foreground)
	windowAccentFG := nonBlackStatusColorForBG(ensureReadableTextColor(windowActiveBG, blendHexColor(windowActiveFG, payload.Background, 0.36), payload.Foreground), windowActiveFG, windowActiveBG, 4.2)
	windowInactiveFG = avoidExtremeTextInk(windowInactiveBG, windowInactiveFG, blendHexColor(payload.Foreground, payload.Background, 0.30))
	windowActiveFG = avoidExtremeTextInk(windowActiveBG, windowActiveFG, blendHexColor(primaryRole, tertiaryRole, 0.50))
	windowAccentFG = avoidExtremeTextInk(windowActiveBG, windowAccentFG, blendHexColor(windowActiveFG, primaryRole, 0.35))
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
	widgetBattery := onOffFlag(parseOnOffDefault(payload.Meta["widget_battery"], true))
	widgetCPU := onOffFlag(parseOnOffDefault(payload.Meta["widget_cpu"], true))
	widgetRAM := onOffFlag(parseOnOffDefault(payload.Meta["widget_ram"], true))
	widgetWeather := onOffFlag(parseOnOffDefault(payload.Meta["widget_weather"], true))
	surfaceHighest := statusHighestBG
	primaryBase := nonBlackStatusColor(primaryRole, payload.Foreground)
	primaryFixed := nonBlackStatusColor(getRoleOr(payload.Roles, "primary_fixed", primaryBase), payload.Foreground)
	primaryFixedDim := nonBlackStatusColor(getRoleOr(payload.Roles, "primary_fixed_dim", primaryFixed), payload.Foreground)
	secondaryBase := nonBlackStatusColor(secondaryRole, payload.Foreground)
	secondaryFixed := nonBlackStatusColor(getRoleOr(payload.Roles, "secondary_fixed", secondaryBase), payload.Foreground)
	secondaryFixedDim := nonBlackStatusColor(getRoleOr(payload.Roles, "secondary_fixed_dim", secondaryFixed), payload.Foreground)
	tertiaryBase := nonBlackStatusColor(tertiaryRole, payload.Foreground)
	tertiaryFixed := nonBlackStatusColor(getRoleOr(payload.Roles, "tertiary_fixed", tertiaryBase), payload.Foreground)
	tertiaryFixedDim := nonBlackStatusColor(getRoleOr(payload.Roles, "tertiary_fixed_dim", tertiaryFixed), payload.Foreground)
	weatherColor := nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "text_accent_primary", tertiaryFixedDim), payload.Foreground), tertiaryFixedDim, payload.Background, 4.2)
	separatorBase := getRoleOr(payload.Roles, "outline_variant", blendHexColor(payload.Foreground, payload.Background, 0.48))
	separatorColor := nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, separatorBase, payload.Foreground), separatorBase, payload.Background, 3.2)
	ruleBaseColor := ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "outline_variant", separatorColor), payload.Foreground)
	rulePrefixColor := ensureReadableTextColor(payload.Background, blendHexColor(prefixBG, getRoleOr(payload.Roles, "primary", prefixBG), 0.42), payload.Foreground)
	ruleCopyColor := ensureReadableTextColor(payload.Background, blendHexColor(copyBG, getRoleOr(payload.Roles, "secondary", copyBG), 0.40), payload.Foreground)
	chargingColor := nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "text_accent_secondary", secondaryFixed), payload.Foreground), secondaryFixed, payload.Background, 4.2)
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
		base := saturateHexColor(c, 0.16)
		batteryColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, base, payload.Foreground), base, payload.Background, 3.6)
	}
	batteryLevelStops := []string{
		getRoleOr(payload.Roles, "error", batteryColors[0]),
		blendHexColor(getRoleOr(payload.Roles, "error", batteryColors[0]), getRoleOr(payload.Roles, "tertiary", batteryColors[2]), 0.45),
		getRoleOr(payload.Roles, "tertiary", batteryColors[2]),
		blendHexColor(getRoleOr(payload.Roles, "tertiary", batteryColors[2]), getRoleOr(payload.Roles, "secondary", batteryColors[4]), 0.50),
		getRoleOr(payload.Roles, "secondary", batteryColors[4]),
		blendHexColor(getRoleOr(payload.Roles, "secondary", batteryColors[4]), getRoleOr(payload.Roles, "primary", batteryColors[5]), 0.50),
		getRoleOr(payload.Roles, "primary", batteryColors[5]),
		blendHexColor(getRoleOr(payload.Roles, "primary", batteryColors[5]), getRoleOr(payload.Roles, "secondary_fixed", batteryColors[5]), 0.45),
		getRoleOr(payload.Roles, "secondary_fixed", batteryColors[5]),
		blendHexColor(getRoleOr(payload.Roles, "secondary_fixed", batteryColors[5]), getRoleOr(payload.Roles, "text_accent_secondary", batteryColors[5]), 0.40),
	}
	batteryLevelColors := make([]string, 10)
	for i := range batteryLevelColors {
		t := float64(i) / float64(len(batteryLevelColors)-1)
		raw := saturateHexColor(sampleGradientColor(batteryLevelStops, t), 0.12)
		fallbackIdx := i / 2
		if fallbackIdx >= len(batteryColors) {
			fallbackIdx = len(batteryColors) - 1
		}
		batteryLevelColors[i] = nonBlackStatusColor(raw, batteryColors[fallbackIdx])
	}
	batteryEmptyMuted := saturateHexColor(blendHexColor(surfaceHighest, getRoleOr(payload.Roles, "text_muted", surfaceHighest), 0.36), 0.10)
	batteryFullColors := [4]string{}
	batteryHalfColors := [4]string{}
	batteryEmptyColors := [4]string{}
	for i := 0; i < 4; i++ {
		fullIdx := clampInt(2+(i*2), 1, 10) - 1
		halfIdx := clampInt(1+(i*2), 1, 10) - 1
		batteryFullColors[i] = batteryLevelColors[fullIdx]
		batteryHalfColors[i] = batteryLevelColors[halfIdx]
		batteryEmptyColors[i] = batteryEmptyMuted
	}
	cpuColors := []string{
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, primaryBase, payload.Foreground), primaryBase, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, primaryFixed, payload.Foreground), primaryFixed, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, secondaryBase, payload.Foreground), secondaryBase, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, secondaryFixed, payload.Foreground), secondaryFixed, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, tertiaryBase, payload.Foreground), tertiaryBase, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, tertiaryFixed, payload.Foreground), tertiaryFixed, payload.Background, 3.6),
	}
	ramColors := []string{
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, tertiaryBase, payload.Foreground), tertiaryBase, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, tertiaryFixed, payload.Foreground), tertiaryFixed, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, primaryFixedDim, payload.Foreground), primaryFixedDim, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, primaryFixed, payload.Foreground), primaryFixed, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, secondaryFixedDim, payload.Foreground), secondaryFixedDim, payload.Background, 3.6),
		nonBlackStatusColorForBG(ensureReadableTextColor(payload.Background, secondaryFixed, payload.Foreground), secondaryFixed, payload.Background, 3.6),
	}
	widgetBandBase := nonBlackStatusColor(
		lightenHexColor(
			saturateHexColor(
				blendHexColor(statusElevatedBG, statusBaseBG, 0.50),
				0.06,
			),
			statusLift*0.76,
		),
		payload.Foreground,
	)
	batteryBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(tertiaryContainerRole, widgetBandBase, tuning.WidgetMix*0.52), 0.14), statusLift*0.28), payload.Foreground)
	chargingBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(primaryContainerRole, widgetBandBase, tuning.WidgetMix*0.50), 0.14), statusLift*0.28), payload.Foreground)
	cpuBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(secondaryContainerRole, widgetBandBase, tuning.WidgetMix*0.52), 0.14), statusLift*0.28), payload.Foreground)
	ramBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(blendHexColor(primaryContainerRole, tertiaryContainerRole, 0.44), widgetBandBase, tuning.WidgetMix*0.50), 0.14), statusLift*0.28), payload.Foreground)
	weatherBG := nonBlackStatusColor(lightenHexColor(saturateHexColor(blendHexColor(blendHexColor(secondaryContainerRole, tertiaryContainerRole, 0.48), widgetBandBase, tuning.WidgetMix*0.50), 0.14), statusLift*0.28), payload.Foreground)
	accentFGA := getRoleOr(payload.Roles, "text_accent_primary", primaryBase)
	accentFGB := getRoleOr(payload.Roles, "text_accent_secondary", secondaryBase)
	widgetAccentFG := bestTextColorForBackgrounds(
		accentFGA,
		ensureReadableTextColor(payload.Background, payload.Foreground, accentFGB),
		3.8,
		payload.Background, sessionBG, prefixBG, copyBG, batteryBG, chargingBG, cpuBG, ramBG, weatherBG,
	)
	sessionBG = ensureBackgroundContrastForText(sessionBG, widgetAccentFG, 3.8)
	prefixBG = ensureBackgroundContrastForText(prefixBG, widgetAccentFG, 3.8)
	copyBG = ensureBackgroundContrastForText(copyBG, widgetAccentFG, 3.8)
	batteryBG = ensureBackgroundContrastForText(batteryBG, widgetAccentFG, 3.8)
	chargingBG = ensureBackgroundContrastForText(chargingBG, widgetAccentFG, 3.8)
	cpuBG = ensureBackgroundContrastForText(cpuBG, widgetAccentFG, 3.8)
	ramBG = ensureBackgroundContrastForText(ramBG, widgetAccentFG, 3.8)
	weatherBG = ensureBackgroundContrastForText(weatherBG, widgetAccentFG, 3.8)
	sessionFG := nonBlackStatusColorForBG(
		preferChromaticTextColor(
			bestTextColorForBackgrounds(getRoleOr(payload.Roles, "on_tertiary_container", widgetAccentFG), widgetAccentFG, 4.5, sessionBG),
			widgetAccentFG,
			sessionBG,
			4.5,
		),
		blendHexColor(widgetAccentFG, getRoleOr(payload.Roles, "primary", widgetAccentFG), 0.34),
		sessionBG,
		4.5,
	)
	prefixFG := nonBlackStatusColorForBG(
		preferChromaticTextColor(
			bestTextColorForBackgrounds(getRoleOr(payload.Roles, "on_error_container", widgetAccentFG), widgetAccentFG, 4.5, prefixBG),
			widgetAccentFG,
			prefixBG,
			4.5,
		),
		blendHexColor(widgetAccentFG, getRoleOr(payload.Roles, "secondary", widgetAccentFG), 0.34),
		prefixBG,
		4.5,
	)
	copyFG := nonBlackStatusColorForBG(
		preferChromaticTextColor(
			bestTextColorForBackgrounds(getRoleOr(payload.Roles, "on_secondary_container", widgetAccentFG), widgetAccentFG, 4.5, copyBG),
			widgetAccentFG,
			copyBG,
			4.5,
		),
		blendHexColor(widgetAccentFG, getRoleOr(payload.Roles, "tertiary", widgetAccentFG), 0.34),
		copyBG,
		4.5,
	)
	weatherColor = ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "text_accent_primary", weatherColor), payload.Foreground)
	weatherColor = ensureReadableTextColor(weatherBG, weatherColor, payload.Foreground)
	weatherColor = nonBlackStatusColorForBG(weatherColor, payload.Foreground, payload.Background, 4.2)
	chargingColor = ensureReadableTextColor(payload.Background, getRoleOr(payload.Roles, "text_accent_secondary", chargingColor), payload.Foreground)
	chargingColor = ensureReadableTextColor(chargingBG, chargingColor, payload.Foreground)
	chargingColor = nonBlackStatusColorForBG(chargingColor, payload.Foreground, payload.Background, 4.2)
	for i, c := range batteryLevelColors {
		batteryLevelColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(batteryBG, c, sessionFG), c, batteryBG, 3.3)
	}
	batteryEmptyMuted = nonBlackStatusColorForBG(ensureReadableTextColor(batteryBG, batteryEmptyMuted, sessionFG), batteryEmptyMuted, batteryBG, 3.1)
	for i, c := range batteryColors {
		batteryColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(batteryBG, c, sessionFG), c, batteryBG, 3.4)
	}
	for i, c := range batteryFullColors {
		batteryFullColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(batteryBG, c, sessionFG), c, batteryBG, 3.4)
	}
	for i, c := range batteryHalfColors {
		batteryHalfColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(batteryBG, c, sessionFG), c, batteryBG, 3.4)
	}
	for i, c := range batteryEmptyColors {
		batteryEmptyColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(batteryBG, c, sessionFG), c, batteryBG, 3.1)
	}
	for i, c := range cpuColors {
		cpuColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(cpuBG, c, sessionFG), c, cpuBG, 3.4)
	}
	for i, c := range ramColors {
		ramColors[i] = nonBlackStatusColorForBG(ensureReadableTextColor(ramBG, c, sessionFG), c, ramBG, 3.4)
	}
	rightChipBGs := diversifyAdjacentStatusColors([]string{batteryBG, chargingBG, cpuBG, ramBG, weatherBG}, payload.Background, 22.0, 3.6)
	batteryBG, chargingBG, cpuBG, ramBG, weatherBG = rightChipBGs[0], rightChipBGs[1], rightChipBGs[2], rightChipBGs[3], rightChipBGs[4]
	batteryBG = ensureBackgroundContrastForText(batteryBG, widgetAccentFG, 3.8)
	chargingBG = ensureBackgroundContrastForText(chargingBG, widgetAccentFG, 3.8)
	cpuBG = ensureBackgroundContrastForText(cpuBG, widgetAccentFG, 3.8)
	ramBG = ensureBackgroundContrastForText(ramBG, widgetAccentFG, 3.8)
	weatherBG = ensureBackgroundContrastForText(weatherBG, widgetAccentFG, 3.8)
	batteryColors = diversifyAdjacentStatusColors(batteryColors, batteryBG, 12.0, 3.3)
	cpuColors = diversifyAdjacentStatusColors(cpuColors, cpuBG, 14.0, 3.4)
	ramColors = diversifyAdjacentStatusColors(ramColors, ramBG, 14.0, 3.4)
	batteryBGSetting := batteryBG
	if statusTheme == "default" {
		batteryBGSetting = "default"
	}
	edgeStyle := "rounded"
	leftEdgeStyle := edgeStyle
	bgRight := "on"
	leftGap := ""
	leftSessionPad := "none"
	rightGap := "space"
	windowStatusFormat := fmt.Sprintf(`#[fg=%s,bg=%s,nobold,noitalics,nounderscore] #I `, windowInactiveFG, windowInactiveBG)
	windowStatusCurrentFormat := fmt.Sprintf(`#[fg=%s,bg=%s,bold,noitalics,nounderscore] #W `, windowActiveFG, windowActiveBG)
	switch statusTheme {
	case "rounded":
		// One outer rounded capsule for the full window list, with active window inset as its own pill.
		leftGap = " "
		rightGap = "space"
		windowStatusFormat = fmt.Sprintf(`#{?window_start_flag,#[fg=%s]#[bg=default],}#[fg=%s,bg=%s,nobold,noitalics,nounderscore]#{?window_start_flag,#I ,#{?window_end_flag,#I,#I }}#{?window_end_flag,#[fg=%s]#[bg=default],}`, windowInactiveBG, windowInactiveFG, windowInactiveBG, windowInactiveBG)
		windowStatusCurrentFormat = fmt.Sprintf(`#{?window_start_flag,#[fg=%s]#[bg=default],}#{?window_start_flag,,#[fg=%s]#[bg=%s]}#[fg=%s,bg=%s,bold,noitalics,nounderscore]#W#{?window_end_flag,#[fg=%s]#[bg=default],}#{?window_end_flag,,#[fg=%s]#[bg=%s]}#{?window_end_flag,,#[fg=%s]#[bg=%s] }`, windowActiveBG, windowActiveBG, windowInactiveBG, windowAccentFG, windowActiveBG, windowActiveBG, windowActiveBG, windowInactiveBG, windowInactiveFG, windowInactiveBG)
	case "rectangle":
		edgeStyle = "flat"
		leftEdgeStyle = "flat"
		leftGap = ""
		rightGap = "none"
		leftSessionPad = "space"
	default:
		leftEdgeStyle = "flat"
		bgRight = "off"
		leftSessionPad = "space"
	}
	statusRuleThin := strings.Repeat("─", 260)
	statusRuleThick := strings.Repeat("━", 260)
	statusRuleExpr := fmt.Sprintf(`#{?client_prefix,#[fg=%s]%s,#{?pane_in_mode,#[fg=%s]%s,#{?session_alerts,#[fg=%s,bold]%s,#[fg=%s]%s}}}`, rulePrefixColor, statusRuleThick, ruleCopyColor, statusRuleThick, alertColor, statusRuleThick, ruleBaseColor, statusRuleThin)
	statusRows := 2
	statusFormatCommands := "set -g status-format[1] \"\"\nset -gu status-format[2]"
	if statusLayout == "single-line" {
		statusRows = 1
		statusFormatCommands = "set -gu status-format[1]\nset -gu status-format[2]"
	} else {
		statusFormatCommands = fmt.Sprintf("set -g status-format[1] %q\nset -gu status-format[2]", statusRuleExpr)
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
set -g @status-tmux-left-fg-session "%s"
set -g @status-tmux-left-fg-prefix "%s"
set -g @status-tmux-left-fg-copy "%s"
set -g @status-tmux-left-session-pad "%s"
set -g status-left "#(\$HOME/.config/tooie/configs/tmux/widget-left '#{session_name}' '#{client_prefix}' '#{pane_in_mode}')%s"
set -g status-right "#(\$HOME/.config/tooie/configs/tmux/run-system-widget all)#(\$HOME/.config/tooie/configs/tmux/widget-weather)"
%s
set -g window-status-separator ""
set -g mouse on
set -g base-index 1
setw -g pane-base-index 1
set -g renumber-windows on
set -g window-status-format "%s"
set -g window-status-current-format "%s"
set -g window-status-activity-style "fg=%s,bold"
set -g window-status-bell-style "fg=%s,bold"
setw -g monitor-activity off
set -g visual-activity off
setw -g monitor-bell on
set -g visual-bell off
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
set -g @status-tmux-battery-segments "%d"
set -g @status-tmux-color-battery-empty-muted "%s"
set -g @status-tmux-color-battery-level-1 "%s"
set -g @status-tmux-color-battery-level-2 "%s"
set -g @status-tmux-color-battery-level-3 "%s"
set -g @status-tmux-color-battery-level-4 "%s"
set -g @status-tmux-color-battery-level-5 "%s"
set -g @status-tmux-color-battery-level-6 "%s"
set -g @status-tmux-color-battery-level-7 "%s"
set -g @status-tmux-color-battery-level-8 "%s"
set -g @status-tmux-color-battery-level-9 "%s"
set -g @status-tmux-color-battery-level-10 "%s"
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
		sessionFG,
		prefixFG,
		copyFG,
		leftSessionPad,
		leftGap,
		statusFormatCommands,
		windowStatusFormat,
		windowStatusCurrentFormat,
		alertColor, alertColor,
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
		5,
		batteryEmptyMuted,
		batteryLevelColors[0], batteryLevelColors[1], batteryLevelColors[2], batteryLevelColors[3], batteryLevelColors[4],
		batteryLevelColors[5], batteryLevelColors[6], batteryLevelColors[7], batteryLevelColors[8], batteryLevelColors[9],
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
		getRoleOr(payload.Roles, "primary", payload.Colors[4]),
		getRoleOr(payload.Roles, "secondary", payload.Colors[6]),
		getRoleOr(payload.Roles, "tertiary", payload.Colors[5]),
		getRoleOr(payload.Roles, "primary_fixed", payload.Colors[12]),
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

func nonBlackStatusColorForBG(hex, fallback, bg string, minContrast float64) string {
	hex = normalizeHexColor(hex)
	fallback = normalizeHexColor(fallback)
	bg = normalizeHexColor(bg)
	if !itheme.IsHexColor(bg) {
		return nonBlackStatusColor(hex, fallback)
	}
	cand := nonBlackStatusColor(hex, fallback)
	if contrastRatioHex(cand, bg) >= minContrast {
		return cand
	}
	if contrastRatioHex(fallback, bg) >= minContrast {
		return fallback
	}
	seed := fallback
	if !itheme.IsHexColor(seed) {
		seed = cand
	}
	best := seed
	bestRatio := contrastRatioHex(seed, bg)
	for i := 1; i <= 16; i++ {
		t := float64(i) / 16.0
		lighter := normalizeHexColor(blendHexColor(seed, "#ffffff", t))
		lr := contrastRatioHex(lighter, bg)
		if lr > bestRatio {
			best = lighter
			bestRatio = lr
		}
		if lr >= minContrast {
			return lighter
		}
		darker := normalizeHexColor(blendHexColor(seed, "#000000", t))
		dr := contrastRatioHex(darker, bg)
		if dr > bestRatio {
			best = darker
			bestRatio = dr
		}
		if dr >= minContrast {
			return darker
		}
	}
	return best
}

func avoidExtremeTextInk(bg, fg, fallback string) string {
	fg = normalizeHexColor(fg)
	if !itheme.IsHexColor(fg) {
		return normalizeHexColor(fallback)
	}
	l := relativeLuminanceHex(fg)
	if l > 0.95 || l < 0.05 {
		seed := normalizeHexColor(blendHexColor(fg, fallback, 0.26))
		return ensureReadableTextColor(bg, seed, fg)
	}
	return fg
}

func bestTextColorForBackgrounds(preferred, fallback string, minContrast float64, backgrounds ...string) string {
	candidates := []string{
		preferred,
		fallback,
		blendHexColor(preferred, fallback, 0.35),
		blendHexColor(preferred, fallback, 0.65),
	}
	for _, bg := range backgrounds {
		candidates = append(candidates, ensureReadableTextColor(bg, preferred, fallback))
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
			if !isExtremeNeutral(cand) {
				return cand
			}
		}
	}
	if bestMin >= minContrast {
		return best
	}
	// Last resort only: allow pure neutral colors if chromatic options cannot satisfy contrast.
	for _, cand := range []string{"#f5f7ff", "#ffffff", "#111111", "#000000"} {
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
		if validBG == 0 || minSeen >= minContrast {
			return cand
		}
		if minSeen > bestMin {
			bestMin = minSeen
			best = cand
		}
	}
	return best
}

func preferChromaticTextColor(candidate, fallback, bg string, minContrast float64) string {
	candidate = normalizeHexColor(candidate)
	fallback = normalizeHexColor(fallback)
	bg = normalizeHexColor(bg)
	if !itheme.IsHexColor(candidate) {
		candidate = fallback
	}
	if itheme.IsHexColor(candidate) && !isExtremeNeutral(candidate) && contrastRatioHex(candidate, bg) >= minContrast {
		return candidate
	}
	alts := []string{
		fallback,
		saturateHexColor(fallback, 0.22),
		saturateHexColor(blendHexColor(fallback, candidate, 0.5), 0.18),
		blendHexColor(fallback, "#9bb8ff", 0.25),
		blendHexColor(fallback, "#8fe5d5", 0.25),
	}
	for _, alt := range alts {
		alt = normalizeHexColor(alt)
		if !itheme.IsHexColor(alt) {
			continue
		}
		if contrastRatioHex(alt, bg) >= minContrast && !isExtremeNeutral(alt) {
			return alt
		}
	}
	if contrastRatioHex(candidate, bg) >= minContrast {
		return candidate
	}
	return ensureReadableTextColor(bg, fallback, candidate)
}

func isExtremeNeutral(hex string) bool {
	hex = normalizeHexColor(hex)
	r, g, b := parseHexColor(hex)
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	maxv := math.Max(rf, math.Max(gf, bf))
	minv := math.Min(rf, math.Min(gf, bf))
	delta := maxv - minv
	luma := relativeLuminanceHex(hex)
	return delta < 0.06 || luma <= 0.06 || luma >= 0.94
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

func diversifyAdjacentStatusColors(colors []string, bg string, minHueDelta, minContrast float64) []string {
	if len(colors) <= 1 {
		return colors
	}
	out := append([]string{}, colors...)
	// Keep diversification chromatic but avoid injecting green into adjacent chips.
	seeds := []string{"#4f8dff", "#8d7dff", "#ffd166", "#ff8f5a", "#c07dff", "#2dd4ff"}
	for i := 1; i < len(out); i++ {
		prev := normalizeHexColor(out[i-1])
		cur := normalizeHexColor(out[i])
		if !itheme.IsHexColor(prev) || !itheme.IsHexColor(cur) {
			continue
		}
		hueDelta := hueDistanceDegrees(hueFromHex(prev), hueFromHex(cur))
		if hueDelta >= minHueDelta && contrastRatioHex(prev, cur) >= 1.10 {
			continue
		}
		seed := seeds[i%len(seeds)]
		cand := normalizeHexColor(blendHexColor(cur, seed, 0.26))
		cand = nonBlackStatusColorForBG(ensureReadableTextColor(bg, cand, cur), cand, bg, minContrast)
		out[i] = cand
	}
	return out
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
	prompt := normalizeStarshipPrompt(payload.Meta["starship_prompt"])
	if err := writeStarshipTemplate(path, payload.Meta["starship_prompt"]); err != nil {
		return err
	}
	if err := sanitizeStarshipConfig(path); err != nil {
		return err
	}
	c := payload.Colors
	if prompt != "gruvbox" {
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
	}
	if prompt == "gruvbox" {
		fancyInk := bestTextColorForBackgrounds(
			"#1a1a1a",
			payload.Foreground,
			4.5,
			c[5],               // color_orange
			c[15],              // color_yellow
			c[3],               // color_aqua
			c[4],               // color_blue
			c[14],              // color_bg3
			payload.Background, // color_bg1
		)
		gruvboxPalette := []struct{ key, val string }{
			{"color_fg0", fmt.Sprintf("%q", fancyInk)},
			{"color_bg1", fmt.Sprintf("%q", payload.Background)},
			{"color_bg3", fmt.Sprintf("%q", c[14])},
			{"color_blue", fmt.Sprintf("%q", c[4])},
			{"color_aqua", fmt.Sprintf("%q", c[3])},
			{"color_green", fmt.Sprintf("%q", c[2])},
			{"color_orange", fmt.Sprintf("%q", c[5])},
			{"color_purple", fmt.Sprintf("%q", c[6])},
			{"color_red", fmt.Sprintf("%q", c[1])},
			{"color_yellow", fmt.Sprintf("%q", c[15])},
		}
		if err := tomlUpsert(path, "", "palette", "\"tooie_gruvbox\""); err != nil {
			return err
		}
		for _, item := range gruvboxPalette {
			if err := tomlUpsert(path, "palettes.tooie_gruvbox", item.key, item.val); err != nil {
				return err
			}
		}
	}
	// Keep the active Starship default path in sync so themed prompts apply
	// immediately even when the shell does not export STARSHIP_CONFIG.
	userStarship := filepath.Join(homeDir, ".config", "starship.toml")
	if err := os.MkdirAll(filepath.Dir(userStarship), 0o755); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(userStarship, raw, 0o644); err != nil {
		return err
	}
	return nil
}

func writeStarshipTemplate(dstPath, prompt string) error {
	srcPath, err := managedStarshipTemplatePath(prompt)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, raw, 0o644)
}

func starshipTemplateRelativePath(prompt string) string {
	switch normalizeStarshipPrompt(prompt) {
	case "pure":
		return filepath.Join("assets", "defaults", ".config", "starship-pure.toml")
	case "gruvbox":
		return filepath.Join("assets", "defaults", ".config", "starship-gruvbox.toml")
	default:
		return filepath.Join("assets", "defaults", ".config", "starship.toml")
	}
}

func sanitizeStarshipConfig(path string) error {
	raw, _ := os.ReadFile(path)
	if len(raw) == 0 {
		return nil
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		// Remove legacy/bad root keys that can trigger parser warnings.
		if strings.HasPrefix(trim, "battery ") || strings.HasPrefix(trim, "battery=") {
			continue
		}
		out = append(out, ln)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

func tomlUpsert(path, section, key, value string) error {
	raw, _ := os.ReadFile(path)
	lines := strings.Split(string(raw), "\n")
	if strings.TrimSpace(section) == "" {
		replaced := false
		for i, ln := range lines {
			trim := strings.TrimSpace(ln)
			if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
				break
			}
			if strings.HasPrefix(trim, key+" ") || strings.HasPrefix(trim, key+"=") {
				lines[i] = fmt.Sprintf("%s = %s", key, value)
				replaced = true
				break
			}
		}
		if !replaced {
			insertAt := 0
			for i, ln := range lines {
				trim := strings.TrimSpace(ln)
				if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
					insertAt = i
					break
				}
				insertAt = i + 1
			}
			lines = append(lines[:insertAt], append([]string{fmt.Sprintf("%s = %s", key, value)}, lines[insertAt:]...)...)
		}
		return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
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
	if wall, ok := bestWallpaperPath(homeDir); ok {
		return wall, nil
	}
	return "", fmt.Errorf("wallpaper not found; set TOOIE_WALLPAPER=/path/to/image or place one in ~/Pictures")
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

func runMatugenImage(bin, wallpaper, mode, schemeType string, sourceColorIndex int, prefer, fallbackColor, resizeFilter string) ([]byte, error) {
	cacheKey := strings.Join([]string{
		bin,
		wallpaper,
		mode,
		schemeType,
		fmt.Sprintf("%d", sourceColorIndex),
		strings.TrimSpace(prefer),
		strings.TrimSpace(fallbackColor),
		strings.TrimSpace(resizeFilter),
	}, "|")
	if raw := loadMatugenCache(cacheKey); len(raw) > 0 {
		return raw, nil
	}
	raw, err := runMatugenTemplateImage(bin, wallpaper, mode, schemeType, sourceColorIndex, prefer, fallbackColor, resizeFilter)
	if err == nil && len(raw) > 0 {
		storeMatugenCache(cacheKey, raw)
		return raw, nil
	}
	legacyRaw, legacyErr := runMatugenDryRunImage(bin, wallpaper, mode, schemeType, sourceColorIndex, prefer, fallbackColor, resizeFilter)
	if legacyErr != nil {
		if err != nil {
			return nil, fmt.Errorf("matugen template path failed: %v; dry-run fallback failed: %v", err, legacyErr)
		}
		return nil, legacyErr
	}
	storeMatugenCache(cacheKey, legacyRaw)
	return legacyRaw, nil
}

func loadMatugenCache(key string) []byte {
	matugenResultCache.mu.Lock()
	defer matugenResultCache.mu.Unlock()
	raw := matugenResultCache.m[key]
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func storeMatugenCache(key string, raw []byte) {
	matugenResultCache.mu.Lock()
	defer matugenResultCache.mu.Unlock()
	cp := make([]byte, len(raw))
	copy(cp, raw)
	matugenResultCache.m[key] = cp
}

func runMatugenTemplateImage(bin, wallpaper, mode, schemeType string, sourceColorIndex int, prefer, fallbackColor, resizeFilter string) ([]byte, error) {
	cfgFile, err := os.CreateTemp("", "tooie-matugen-*.toml")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.Remove(cfgFile.Name())
	}()
	tmpDir, err := os.MkdirTemp("", "tooie-matugen-roles-*")
	if err != nil {
		_ = cfgFile.Close()
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	templatePath := filepath.Join(tmpDir, "roles.json")
	outputPath := filepath.Join(tmpDir, "roles-out.json")
	if err := os.WriteFile(templatePath, []byte(matugenRolesTemplate), 0o644); err != nil {
		_ = cfgFile.Close()
		return nil, err
	}
	config := fmt.Sprintf("[config]\n\n[templates.roles]\ninput_path = '%s'\noutput_path = '%s'\n", templatePath, outputPath)
	if _, err := cfgFile.WriteString(config); err != nil {
		_ = cfgFile.Close()
		return nil, err
	}
	if err := cfgFile.Close(); err != nil {
		return nil, err
	}
	args := []string{"image", wallpaper, "-m", mode, "-t", schemeType, "-c", cfgFile.Name()}
	if sourceColorIndex >= 0 {
		args = append(args, "--source-color-index", fmt.Sprintf("%d", sourceColorIndex))
	}
	if strings.TrimSpace(prefer) != "" {
		args = append(args, "--prefer", strings.TrimSpace(prefer))
	}
	if strings.TrimSpace(fallbackColor) != "" {
		args = append(args, "--fallback-color", strings.TrimSpace(fallbackColor))
	}
	if strings.TrimSpace(resizeFilter) != "" {
		args = append(args, "--resize-filter", strings.TrimSpace(resizeFilter))
	}
	cmd := exec.Command(bin, args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return nil, fmt.Errorf("%v (%s)", runErr, strings.TrimSpace(string(out)))
	}
	raw, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		return nil, readErr
	}
	return bytes.TrimSpace(raw), nil
}

func runMatugenDryRunImage(bin, wallpaper, mode, schemeType string, sourceColorIndex int, prefer, fallbackColor, resizeFilter string) ([]byte, error) {
	args := []string{"image", wallpaper, "-m", mode, "-t", schemeType}
	if sourceColorIndex >= 0 {
		args = append(args, "--source-color-index", fmt.Sprintf("%d", sourceColorIndex))
	}
	if strings.TrimSpace(prefer) != "" {
		args = append(args, "--prefer", strings.TrimSpace(prefer))
	}
	if strings.TrimSpace(fallbackColor) != "" {
		args = append(args, "--fallback-color", strings.TrimSpace(fallbackColor))
	}
	if strings.TrimSpace(resizeFilter) != "" {
		args = append(args, "--resize-filter", strings.TrimSpace(resizeFilter))
	}
	args = append(args, "-j", "hex", "--dry-run")
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("matugen failed for mode=%s scheme=%s idx=%d prefer=%s: %v (%s)", mode, schemeType, sourceColorIndex, strings.TrimSpace(prefer), err, strings.TrimSpace(string(out)))
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

func hexToHSL(hex string) (float64, float64, float64) {
	r, g, b := parseHexColor(hex)
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	maxv := math.Max(rf, math.Max(gf, bf))
	minv := math.Min(rf, math.Min(gf, bf))
	light := (maxv + minv) / 2.0
	if maxv == minv {
		return 0, 0, light
	}
	delta := maxv - minv
	var sat float64
	if light > 0.5 {
		sat = delta / (2.0 - maxv - minv)
	} else {
		sat = delta / (maxv + minv)
	}
	return hueFromRGB(rf, gf, bf), sat, light
}

func shiftHexHue(hex string, shift float64) string {
	h, s, l := hexToHSL(hex)
	return hslToHex(h+shift, s, l)
}

func pushHueAway(hex string, avoidHue, minDelta, fallbackShift float64) string {
	h, s, l := hexToHSL(hex)
	if hueDistanceDegrees(h, avoidHue) >= minDelta {
		return normalizeHexColor(hex)
	}
	if fallbackShift == 0 {
		fallbackShift = 60
	}
	return hslToHex(h+fallbackShift, s, l)
}

func saturateHexColor(hex string, boost float64) string {
	h, sat, light := hexToHSL(hex)
	if sat == 0 {
		return normalizeHexColor(hex)
	}
	if boost >= 0 {
		sat = clamp01(sat + boost*(1.0-sat))
	} else {
		sat = clamp01(sat * (1.0 + boost))
	}
	return hslToHex(h, sat, light)
}

func lightenHexColor(hex string, amount float64) string {
	amount = clamp01(amount)
	r, g, b := parseHexColor(hex)
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	maxv := math.Max(rf, math.Max(gf, bf))
	minv := math.Min(rf, math.Min(gf, bf))
	light := (maxv + minv) / 2.0
	if maxv == minv {
		return hslToHex(0, 0, clamp01(light+amount*(1.0-light)))
	}
	var sat float64
	delta := maxv - minv
	if light > 0.5 {
		sat = delta / (2.0 - maxv - minv)
	} else {
		sat = delta / (maxv + minv)
	}
	h := hueFromHex(hex)
	return hslToHex(h, sat, clamp01(light+amount*(1.0-light)))
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
