package theme

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type Source string

const (
	SourceWallpaper Source = "wallpaper"
	SourcePreset    Source = "preset"
)

type Options struct {
	Source         Source
	Mode           string
	Profile        string
	StatusPalette  string
	TextOverride   string
	CursorOverride string
	AnsiOverrides  map[string]string
}

type Result struct {
	EffectiveMode string
	Roles         map[string]string
	Foreground    string
	Background    string
	Cursor        string
	Colors        map[int]string
	Status        StatusColors
	Meta          map[string]string
}

type StatusColors struct {
	Separator string
	Weather   string
	Charging  string
	Battery   [6]string
	CPU       [6]string
	RAM       [6]string
}

type matugenJSON struct {
	Colors map[string]struct {
		Default struct {
			Color string `json:"color"`
		} `json:"default"`
	} `json:"colors"`
}

func ParseMatugenColors(raw []byte) (map[string]string, error) {
	var data matugenJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range data.Colors {
		c := strings.TrimSpace(v.Default.Color)
		if IsHexColor(c) {
			out[k] = NormalizeHex(c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("matugen output had no usable colors")
	}
	return out, nil
}

func BuildPresetRoles(family, variant string) (map[string]string, string, error) {
	preset, ok := presetLookup(strings.TrimSpace(strings.ToLower(family)), strings.TrimSpace(strings.ToLower(variant)))
	if !ok {
		return nil, "", fmt.Errorf("invalid preset selection: %s:%s", family, variant)
	}
	roles := map[string]string{
		"background":             preset.BG,
		"surface_container":      preset.Surface,
		"surface_container_high": blendHex(preset.Surface, "#ffffff", 0.10),
		"surface_variant":        blendHex(preset.Surface, preset.BG, 0.28),
		"surface_dim":            blendHex(preset.BG, "#000000", 0.08),
		"surface_bright":         blendHex(preset.BG, "#ffffff", 0.10),
		"on_background":          preset.FG,
		"on_surface":             preset.FG,
		"on_surface_variant":     preset.Outline,
		"outline":                preset.Outline,
		"outline_variant":        blendHex(preset.Outline, preset.Surface, 0.3),
		"primary":                preset.Primary,
		"secondary":              preset.Secondary,
		"tertiary":               preset.Tertiary,
		"error":                  preset.Error,
		"secondary_fixed":        preset.Secondary,
		"tertiary_fixed":         preset.Tertiary,
	}
	return normalizeRoleMap(roles), preset.Mode, nil
}

func BuildRolesJSON(roles map[string]string) ([]byte, error) {
	type colorDef struct {
		Default struct {
			Color string `json:"color"`
		} `json:"default"`
	}
	obj := struct {
		Colors map[string]colorDef `json:"colors"`
	}{Colors: map[string]colorDef{}}
	keys := make([]string, 0, len(roles))
	for k := range roles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := colorDef{}
		c.Default.Color = NormalizeHex(roles[k])
		obj.Colors[k] = c
	}
	return json.MarshalIndent(obj, "", "  ")
}

func Compute(rolesInput map[string]string, opts Options) (Result, error) {
	roles := normalizeRoleMap(rolesInput)
	mode := canonicalMode(opts.Mode)
	profile := canonicalProfile(opts.Profile)
	if mode == "" {
		mode = "dark"
	}
	if opts.Source == SourceWallpaper {
		roles = applyProfile(roles, profile, mode)
	}

	bg := role(roles, "background", tern(mode == "dark", "#1a1b26", "#eff1f5"))
	surface := role(roles, "surface_container", blendHex(bg, tern(mode == "dark", "#000000", "#ffffff"), 0.12))
	primary := ensureContrast(role(roles, "primary", "#7aa2f7"), bg, 4.0)
	secondary := ensureContrast(role(roles, "secondary", "#7dcfff"), bg, 4.0)
	tertiary := ensureContrast(role(roles, "tertiary", "#bb9af7"), bg, 4.0)
	errorC := ensureContrast(role(roles, "error", "#ff5f5f"), bg, 4.0)
	fg := ensureContrast(role(roles, "on_background", role(roles, "on_surface", tern(mode == "dark", "#c0caf5", "#4c4f69"))), bg, 7.0)
	muted := ensureContrast(role(roles, "on_surface_variant", blendHex(fg, bg, 0.38)), bg, 4.5)
	outline := ensureContrast(role(roles, "outline", blendHex(fg, bg, 0.54)), bg, 3.2)
	cursor := ensureContrast(primary, bg, 4.5)

	guard := role(roles, tern(mode == "dark", "surface_bright", "surface_dim"), blendHex(bg, tern(mode == "dark", "#ffffff", "#000000"), 0.18))
	fg = ensureDualContrast(fg, bg, guard, 7.0)
	cursor = ensureDualContrast(cursor, bg, guard, 4.5)

	if IsHexColor(opts.TextOverride) {
		fg = ensureDualContrast(opts.TextOverride, bg, guard, 7.0)
	}
	if IsHexColor(opts.CursorOverride) {
		cursor = ensureDualContrast(opts.CursorOverride, bg, guard, 4.5)
	}
	// Polarity guard: avoid dark-on-dark or light-on-light text in explicit mode.
	bgLuma := relLuma(bg)
	fgLuma := relLuma(fg)
	if mode == "dark" && fgLuma <= bgLuma {
		fg = ensureDualContrast("#f2f2f8", bg, guard, 7.0)
	}
	if mode == "light" && fgLuma >= bgLuma {
		fg = ensureDualContrast("#111118", bg, guard, 7.0)
	}

	c := map[int]string{}
	c[0] = NormalizeHex(surface)
	c[1] = ensureContrast(errorC, bg, 3.6)
	c[2] = ensureContrast(blendHex(secondary, "#46d37b", 0.45), bg, 3.6)
	c[3] = ensureContrast(blendHex(tertiary, "#ffd75f", 0.55), bg, 3.6)
	c[4] = ensureContrast(primary, bg, 3.6)
	c[5] = ensureContrast(tertiary, bg, 3.6)
	c[6] = ensureContrast(secondary, bg, 3.6)
	c[7] = ensureContrast(role(roles, "on_surface", fg), bg, 6.0)

	brightMix := "#ffffff"
	brightT := 0.24
	if mode == "light" {
		brightMix = "#000000"
		brightT = 0.22
	}
	c[8] = ensureContrast(role(roles, "surface_container_high", blendHex(c[0], brightMix, 0.14)), bg, 1.3)
	for i := 1; i <= 5; i++ {
		c[8+i] = ensureContrast(blendHex(c[i], brightMix, brightT), bg, 4.0)
	}
	c[14] = outline
	c[15] = muted
	c[16] = ensureContrast(role(roles, "secondary_fixed", blendHex(c[2], c[6], 0.50)), bg, 3.2)
	c[17] = ensureContrast(role(roles, "tertiary_fixed", blendHex(c[5], c[4], 0.40)), bg, 3.2)
	c[18] = role(roles, "surface_dim", blendHex(bg, "#000000", ternf(mode == "dark", 0.10, 0.06)))
	c[19] = role(roles, "surface_bright", blendHex(bg, "#ffffff", ternf(mode == "dark", 0.12, 0.06)))
	c[20] = role(roles, "surface_variant", blendHex(c[0], c[15], 0.26))
	c[21] = role(roles, "outline_variant", blendHex(c[14], c[0], 0.30))

	if opts.AnsiOverrides != nil {
		if v := opts.AnsiOverrides["red"]; IsHexColor(v) {
			c[1], c[9] = NormalizeHex(v), NormalizeHex(v)
		}
		if v := opts.AnsiOverrides["green"]; IsHexColor(v) {
			c[2], c[10] = NormalizeHex(v), NormalizeHex(v)
		}
		if v := opts.AnsiOverrides["yellow"]; IsHexColor(v) {
			c[3], c[11] = NormalizeHex(v), NormalizeHex(v)
		}
		if v := opts.AnsiOverrides["blue"]; IsHexColor(v) {
			c[4], c[12] = NormalizeHex(v), NormalizeHex(v)
		}
		if v := opts.AnsiOverrides["magenta"]; IsHexColor(v) {
			c[5], c[13] = NormalizeHex(v), NormalizeHex(v)
		}
		if v := opts.AnsiOverrides["cyan"]; IsHexColor(v) {
			c[6], c[14] = NormalizeHex(v), NormalizeHex(v)
		}
	}

	stateRed := ensureContrast(c[1], bg, 3.6)
	stateOrange := ensureContrast(blendHex(c[1], c[3], 0.58), bg, 3.6)
	stateYellow := ensureContrast(c[3], bg, 3.6)
	stateGreen := ensureContrast(c[2], bg, 3.6)
	stateMint := ensureContrast(blendHex(c[2], c[6], 0.42), bg, 3.6)
	stateBlue := ensureContrast(c[4], bg, 3.6)
	stateTeal := ensureContrast(c[6], bg, 3.6)
	stateViolet := ensureContrast(blendHex(c[4], c[5], 0.55), bg, 3.6)
	stateMagenta := ensureContrast(c[5], bg, 3.6)

	status := StatusColors{}
	if strings.TrimSpace(opts.StatusPalette) == "vibrant" {
		status.Separator = ensureContrast(blendHex(c[14], c[12], 0.55), bg, 3.2)
		status.Weather = ensureContrast(blendHex(stateBlue, stateTeal, 0.45), bg, 3.6)
		status.Charging = ensureContrast(blendHex(stateGreen, stateMint, 0.40), bg, 3.6)
	} else {
		status.Separator = ensureContrast(c[14], bg, 3.0)
		status.Weather = ensureContrast(stateBlue, bg, 3.4)
		status.Charging = ensureContrast(stateGreen, bg, 3.4)
	}
	status.Battery = [6]string{stateRed, stateOrange, stateYellow, ensureContrast(blendHex(stateYellow, stateGreen, 0.42), bg, 3.6), stateMint, stateGreen}
	status.CPU = [6]string{stateGreen, stateTeal, stateBlue, stateViolet, stateOrange, stateRed}
	status.RAM = [6]string{stateBlue, ensureContrast(blendHex(stateBlue, stateTeal, 0.55), bg, 3.6), stateTeal, stateViolet, stateMagenta, stateRed}

	roles["background"] = NormalizeHex(bg)
	roles["surface_container"] = NormalizeHex(surface)
	roles["on_surface"] = NormalizeHex(fg)
	roles["outline"] = NormalizeHex(outline)
	roles["primary"] = NormalizeHex(primary)
	roles["secondary"] = NormalizeHex(secondary)
	roles["tertiary"] = NormalizeHex(tertiary)
	roles["error"] = NormalizeHex(errorC)

	meta := map[string]string{
		"effective_background":      roles["background"],
		"effective_surface":         roles["surface_container"],
		"effective_on_surface":      roles["on_surface"],
		"effective_outline":         roles["outline"],
		"effective_primary":         roles["primary"],
		"effective_secondary":       roles["secondary"],
		"effective_tertiary":        roles["tertiary"],
		"effective_error":           roles["error"],
		"contrast_guard_background": NormalizeHex(guard),
	}

	return Result{
		EffectiveMode: mode,
		Roles:         roles,
		Foreground:    NormalizeHex(fg),
		Background:    NormalizeHex(bg),
		Cursor:        NormalizeHex(cursor),
		Colors:        c,
		Status:        status,
		Meta:          meta,
	}, nil
}

func canonicalMode(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return "dark"
	}
}

func canonicalProfile(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "", "adaptive":
		return "adaptive"
	case "soft-pastel", "studio-dark", "neon-night", "warm-retro", "vivid-noir", "arctic-calm":
		return strings.TrimSpace(strings.ToLower(v))
	case "catppuccin":
		return "soft-pastel"
	case "onedark":
		return "studio-dark"
	case "tokyonight":
		return "neon-night"
	case "gruvbox":
		return "warm-retro"
	case "dracula":
		return "vivid-noir"
	case "nord":
		return "arctic-calm"
	default:
		return "adaptive"
	}
}

func applyProfile(in map[string]string, profile, mode string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = NormalizeHex(v)
	}
	p := role(out, "primary", "#7aa2f7")
	s := role(out, "secondary", "#7dcfff")
	t := role(out, "tertiary", "#bb9af7")
	e := role(out, "error", "#ff5f5f")
	bg := role(out, "background", tern(mode == "dark", "#1a1b26", "#eff1f5"))
	fgAnchor := tern(mode == "dark", "#f5f5f8", "#111118")
	switch profile {
	case "soft-pastel":
		p = blendHex(p, "#89b4fa", 0.34)
		s = blendHex(s, "#94e2d5", 0.30)
		t = blendHex(t, "#cba6f7", 0.30)
		e = blendHex(e, "#f38ba8", 0.32)
		bg = blendHex(bg, tern(mode == "dark", "#1e1e2e", "#eff1f5"), 0.28)
	case "studio-dark":
		p = blendHex(p, "#61afef", 0.36)
		s = blendHex(s, "#56b6c2", 0.32)
		t = blendHex(t, "#c678dd", 0.34)
		e = blendHex(e, "#e06c75", 0.36)
		bg = blendHex(bg, tern(mode == "dark", "#282c34", "#fafafa"), 0.24)
	case "neon-night":
		p = blendHex(p, "#7aa2f7", 0.40)
		s = blendHex(s, "#7dcfff", 0.35)
		t = blendHex(t, "#bb9af7", 0.36)
		e = blendHex(e, "#f7768e", 0.38)
		bg = blendHex(bg, tern(mode == "dark", "#1a1b26", "#e6e7ed"), 0.26)
	case "warm-retro":
		p = blendHex(p, "#458588", 0.44)
		s = blendHex(s, "#689d6a", 0.42)
		t = blendHex(t, "#b16286", 0.40)
		e = blendHex(e, "#cc241d", 0.44)
		bg = blendHex(bg, tern(mode == "dark", "#282828", "#fbf1c7"), 0.30)
	case "vivid-noir":
		p = blendHex(p, "#bd93f9", 0.40)
		s = blendHex(s, "#8be9fd", 0.38)
		t = blendHex(t, "#ff79c6", 0.36)
		e = blendHex(e, "#ff5555", 0.40)
		bg = blendHex(bg, tern(mode == "dark", "#282a36", "#fffbeb"), 0.24)
	case "arctic-calm":
		p = blendHex(p, "#81a1c1", 0.42)
		s = blendHex(s, "#88c0d0", 0.40)
		t = blendHex(t, "#b48ead", 0.38)
		e = blendHex(e, "#bf616a", 0.42)
		bg = blendHex(bg, tern(mode == "dark", "#2e3440", "#eceff4"), 0.28)
	default: // adaptive
		p = blendHex(p, "#4f8dff", 0.22)
		s = blendHex(s, "#31c6c9", 0.24)
		t = blendHex(t, "#cf63ff", 0.22)
		e = blendHex(e, "#ff5f5f", 0.26)
	}
	// Keep accents from flattening against surfaces.
	p = blendHex(blendHex(p, fgAnchor, 0.06), bg, 0.04)
	s = blendHex(blendHex(s, fgAnchor, 0.06), bg, 0.04)
	t = blendHex(blendHex(t, fgAnchor, 0.06), bg, 0.04)
	e = blendHex(e, bg, 0.02)
	out["background"] = NormalizeHex(bg)
	out["primary"] = NormalizeHex(p)
	out["secondary"] = NormalizeHex(s)
	out["tertiary"] = NormalizeHex(t)
	out["error"] = NormalizeHex(e)
	return out
}

func normalizeRoleMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		if IsHexColor(v) {
			out[strings.TrimSpace(k)] = NormalizeHex(v)
		}
	}
	return out
}

func role(m map[string]string, key, fallback string) string {
	if v := strings.TrimSpace(m[key]); IsHexColor(v) {
		return NormalizeHex(v)
	}
	return NormalizeHex(fallback)
}

func ensureDualContrast(fg, bgA, bgB string, target float64) string {
	candA := ensureContrast(fg, bgA, target)
	candB := ensureContrast(fg, bgB, target)
	minA := minContrastSet(candA, bgA, bgB)
	minB := minContrastSet(candB, bgA, bgB)
	if minA >= minB {
		return candA
	}
	return candB
}

func minContrastSet(fg string, bgs ...string) float64 {
	if len(bgs) == 0 {
		return 0
	}
	min := math.MaxFloat64
	for _, bg := range bgs {
		r := contrastRatio(fg, bg)
		if r < min {
			min = r
		}
	}
	return min
}

func ensureContrast(fg, bg string, target float64) string {
	fg = NormalizeHex(fg)
	bg = NormalizeHex(bg)
	if contrastRatio(fg, bg) >= target {
		return fg
	}
	anchor := bestContrastColor(bg, "#f2f2f8", "#111118")
	for _, t := range []float64{0.15, 0.30, 0.45, 0.60, 0.75, 0.90, 1.00} {
		cand := blendHex(fg, anchor, t)
		if contrastRatio(cand, bg) >= target {
			return NormalizeHex(cand)
		}
	}
	return NormalizeHex(anchor)
}

func bestContrastColor(bg, a, b string) string {
	ra := contrastRatio(a, bg)
	rb := contrastRatio(b, bg)
	if ra >= rb {
		return NormalizeHex(a)
	}
	return NormalizeHex(b)
}

func contrastRatio(a, b string) float64 {
	la := relLuma(a)
	lb := relLuma(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relLuma(hex string) float64 {
	r, g, b := parseHex(hex)
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func lin(v int) float64 {
	x := float64(v) / 255.0
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

func blendHex(a, b string, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab := parseHex(a)
	br, bg, bb := parseHex(b)
	r := int(math.Round(float64(ar) + (float64(br-ar) * t)))
	g := int(math.Round(float64(ag) + (float64(bg-ag) * t)))
	bl := int(math.Round(float64(ab) + (float64(bb-ab) * t)))
	return fmt.Sprintf("#%02x%02x%02x", clamp255(r), clamp255(g), clamp255(bl))
}

func clamp255(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func parseHex(c string) (int, int, int) {
	c = strings.TrimSpace(strings.TrimPrefix(c, "#"))
	if len(c) == 3 {
		c = fmt.Sprintf("%c%c%c%c%c%c", c[0], c[0], c[1], c[1], c[2], c[2])
	}
	if len(c) != 6 {
		return 205, 127, 50
	}
	var r, g, b int
	if _, err := fmt.Sscanf(c, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 205, 127, 50
	}
	return r, g, b
}

func IsHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	if len(s) != 6 {
		return false
	}
	for _, ch := range s {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}

func NormalizeHex(s string) string {
	r, g, b := parseHex(s)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func tern(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}

func ternf(ok bool, a, b float64) float64 {
	if ok {
		return a
	}
	return b
}

type presetDef struct {
	Mode      string
	BG        string
	Surface   string
	FG        string
	Outline   string
	Primary   string
	Secondary string
	Tertiary  string
	Error     string
}

func presetLookup(family, variant string) (presetDef, bool) {
	s := map[string]presetDef{
		"catppuccin:latte":     {"light", "#eff1f5", "#e6e9ef", "#4c4f69", "#9ca0b0", "#1e66f5", "#179299", "#8839ef", "#d20f39"},
		"catppuccin:frappe":    {"dark", "#303446", "#292c3c", "#c6d0f5", "#737994", "#8caaee", "#81c8be", "#ca9ee6", "#e78284"},
		"catppuccin:macchiato": {"dark", "#24273a", "#1f2230", "#cad3f5", "#6e738d", "#8aadf4", "#8bd5ca", "#c6a0f6", "#ed8796"},
		"catppuccin:mocha":     {"dark", "#1e1e2e", "#181825", "#cdd6f4", "#6c7086", "#89b4fa", "#94e2d5", "#cba6f7", "#f38ba8"},
		"rose-pine:main":       {"dark", "#191724", "#1f1d2e", "#e0def4", "#6e6a86", "#c4a7e7", "#9ccfd8", "#ebbcba", "#eb6f92"},
		"rose-pine:moon":       {"dark", "#232136", "#2a273f", "#e0def4", "#6e6a86", "#c4a7e7", "#9ccfd8", "#ea9a97", "#eb6f92"},
		"rose-pine:dawn":       {"light", "#faf4ed", "#fffaf3", "#575279", "#9893a5", "#907aa9", "#56949f", "#d7827e", "#b4637a"},
		"tokyo-night:storm":    {"dark", "#24283b", "#1f2335", "#c0caf5", "#565f89", "#7aa2f7", "#7dcfff", "#bb9af7", "#f7768e"},
		"tokyo-night:moon":     {"dark", "#1e2030", "#222436", "#c8d3f5", "#636da6", "#82aaff", "#86e1fc", "#c099ff", "#ff757f"},
		"tokyo-night:night":    {"dark", "#1a1b26", "#24283b", "#c0caf5", "#565f89", "#7aa2f7", "#7dcfff", "#bb9af7", "#f7768e"},
		"tokyo-night:day":      {"light", "#e1e2e7", "#d5d6db", "#3760bf", "#848cb5", "#2e7de9", "#007197", "#9854f1", "#f52a65"},
		"synthwave-84:default": {"dark", "#241b2f", "#2c2240", "#f6f1ff", "#7f6f9b", "#f92aad", "#36f9f6", "#f4f36b", "#ff5c8a"},
		"dracula:default":      {"dark", "#282a36", "#2f3241", "#f8f8f2", "#6272a4", "#bd93f9", "#8be9fd", "#ff79c6", "#ff5555"},
		"gruvbox:dark":         {"dark", "#282828", "#32302f", "#ebdbb2", "#7c6f64", "#83a598", "#8ec07c", "#d3869b", "#fb4934"},
		"gruvbox:light":        {"light", "#fbf1c7", "#f2e5bc", "#3c3836", "#928374", "#458588", "#689d6a", "#b16286", "#cc241d"},
		"nord:default":         {"dark", "#2e3440", "#3b4252", "#eceff4", "#4c566a", "#81a1c1", "#88c0d0", "#b48ead", "#bf616a"},
	}
	p, ok := s[family+":"+variant]
	if ok {
		return p, true
	}
	if family == "synthwave-84" && variant == "" {
		return s["synthwave-84:default"], true
	}
	if family == "dracula" && variant == "" {
		return s["dracula:default"], true
	}
	if family == "nord" && variant == "" {
		return s["nord:default"], true
	}
	if family == "gruvbox" && variant == "" {
		return s["gruvbox:dark"], true
	}
	return presetDef{}, false
}
