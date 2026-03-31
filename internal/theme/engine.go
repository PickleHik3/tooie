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
	StyleFamily    string
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
	if mode == "" {
		mode = "dark"
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
		"style_family":              "native",
		"style_family_version":      "2",
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

func canonicalStyleFamily(v string) string {
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

func canonicalProfile(v string) string {
	return canonicalStyleFamily(v)
}

func applyStyleFamily(in map[string]string, styleFamily, mode string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = NormalizeHex(v)
	}
	p := role(out, "primary", "#7aa2f7")
	s := role(out, "secondary", "#7dcfff")
	t := role(out, "tertiary", "#bb9af7")
	e := role(out, "error", "#ff5f5f")
	bg := role(out, "background", tern(mode == "dark", "#1a1b26", "#eff1f5"))
	surface := role(out, "surface", blendHex(bg, tern(mode == "dark", "#000000", "#ffffff"), 0.10))
	outline := role(out, "outline", blendHex(tern(mode == "dark", "#d8daf0", "#2a2c3d"), bg, 0.68))
	fgAnchor := tern(mode == "dark", "#f5f5f8", "#111118")
	switch styleFamily {
	case "soft-pastel":
		p = blendHex(p, "#c6b6ff", 0.66)
		s = blendHex(s, "#a7d9ff", 0.64)
		t = blendHex(t, "#f0b8d8", 0.64)
		e = blendHex(e, "#ff9db8", 0.58)
		bg = blendHex(bg, tern(mode == "dark", "#2b2233", "#f8f1ff"), 0.58)
		surface = blendHex(surface, tern(mode == "dark", "#34293f", "#f2e9fb"), 0.56)
		outline = blendHex(outline, tern(mode == "dark", "#b8a9d0", "#8c79a8"), 0.52)
	case "studio-dark":
		p = blendHex(p, "#7ea8d9", 0.58)
		s = blendHex(s, "#78b7c8", 0.56)
		t = blendHex(t, "#a58fc0", 0.54)
		e = blendHex(e, "#d07d86", 0.56)
		bg = blendHex(bg, tern(mode == "dark", "#1d232e", "#eef2f7"), 0.62)
		surface = blendHex(surface, tern(mode == "dark", "#252d3a", "#e6ecf4"), 0.58)
		outline = blendHex(outline, tern(mode == "dark", "#7f8fa6", "#5e6f87"), 0.54)
	case "neon-night":
		p = blendHex(p, "#5ea2ff", 0.72)
		s = blendHex(s, "#38e3ff", 0.70)
		t = blendHex(t, "#d180ff", 0.70)
		e = blendHex(e, "#ff6d93", 0.66)
		bg = blendHex(bg, tern(mode == "dark", "#101226", "#e9ecff"), 0.68)
		surface = blendHex(surface, tern(mode == "dark", "#171a31", "#dde3ff"), 0.64)
		outline = blendHex(outline, tern(mode == "dark", "#6a79b8", "#5f6bb5"), 0.60)
	case "warm-retro":
		p = blendHex(p, "#b98f5a", 0.68)
		s = blendHex(s, "#8ea467", 0.66)
		t = blendHex(t, "#c17374", 0.64)
		e = blendHex(e, "#dc6e4e", 0.66)
		bg = blendHex(bg, tern(mode == "dark", "#2b1f1a", "#f7e7cf"), 0.64)
		surface = blendHex(surface, tern(mode == "dark", "#362821", "#ecdac1"), 0.60)
		outline = blendHex(outline, tern(mode == "dark", "#b79b79", "#8f6f4f"), 0.56)
	case "vivid-noir":
		p = blendHex(p, "#b58bff", 0.74)
		s = blendHex(s, "#68e1ff", 0.72)
		t = blendHex(t, "#ff69c7", 0.72)
		e = blendHex(e, "#ff6262", 0.70)
		bg = blendHex(bg, tern(mode == "dark", "#141119", "#fff6eb"), 0.70)
		surface = blendHex(surface, tern(mode == "dark", "#1c1623", "#f6ecdf"), 0.66)
		outline = blendHex(outline, tern(mode == "dark", "#8d78a9", "#8f6aa5"), 0.60)
	case "arctic-calm":
		p = blendHex(p, "#8ea5d6", 0.62)
		s = blendHex(s, "#7dc0d2", 0.60)
		t = blendHex(t, "#9ab2cb", 0.62)
		e = blendHex(e, "#d08086", 0.58)
		bg = blendHex(bg, tern(mode == "dark", "#1a2431", "#eaf2f8"), 0.64)
		surface = blendHex(surface, tern(mode == "dark", "#223041", "#e1ebf4"), 0.62)
		outline = blendHex(outline, tern(mode == "dark", "#8ea2bb", "#6f879f"), 0.58)
	default: // adaptive
		p = blendHex(p, "#4f8dff", 0.22)
		s = blendHex(s, "#31c6c9", 0.24)
		t = blendHex(t, "#cf63ff", 0.22)
		e = blendHex(e, "#ff5f5f", 0.26)
		surface = blendHex(surface, tern(mode == "dark", "#1f2230", "#f2f4f8"), 0.18)
		outline = blendHex(outline, tern(mode == "dark", "#8a8fb0", "#6b718f"), 0.20)
	}
	// Keep accents from flattening against surfaces.
	accentLift := 0.06
	accentSink := 0.04
	if styleFamily != "adaptive" {
		accentLift = 0.03
		accentSink = 0.01
	}
	p = blendHex(blendHex(p, fgAnchor, accentLift), bg, accentSink)
	s = blendHex(blendHex(s, fgAnchor, accentLift), bg, accentSink)
	t = blendHex(blendHex(t, fgAnchor, accentLift), bg, accentSink)
	e = blendHex(e, bg, 0.01)
	surfaceContainer := blendHex(surface, bg, 0.22)
	surfaceContainerHigh := blendHex(surface, fgAnchor, ternf(mode == "dark", 0.12, 0.08))
	surfaceVariant := blendHex(surface, outline, 0.30)
	outlineVariant := blendHex(outline, surface, 0.34)
	out["surface"] = NormalizeHex(surface)
	out["surface_container"] = NormalizeHex(surfaceContainer)
	out["surface_container_high"] = NormalizeHex(surfaceContainerHigh)
	out["surface_variant"] = NormalizeHex(surfaceVariant)
	out["outline"] = NormalizeHex(outline)
	out["outline_variant"] = NormalizeHex(outlineVariant)
	out["primary_fixed"] = NormalizeHex(blendHex(p, fgAnchor, 0.16))
	out["secondary_fixed"] = NormalizeHex(blendHex(s, fgAnchor, 0.14))
	out["tertiary_fixed"] = NormalizeHex(blendHex(t, fgAnchor, 0.14))
	out["primary_fixed_dim"] = NormalizeHex(blendHex(p, bg, 0.08))
	out["secondary_fixed_dim"] = NormalizeHex(blendHex(s, bg, 0.08))
	out["tertiary_fixed_dim"] = NormalizeHex(blendHex(t, bg, 0.08))
	out["primary_container"] = NormalizeHex(blendHex(p, surface, 0.48))
	out["secondary_container"] = NormalizeHex(blendHex(s, surface, 0.50))
	out["tertiary_container"] = NormalizeHex(blendHex(t, surface, 0.52))
	out["error_container"] = NormalizeHex(blendHex(e, surface, 0.54))
	out["background"] = NormalizeHex(bg)
	out["primary"] = NormalizeHex(p)
	out["secondary"] = NormalizeHex(s)
	out["tertiary"] = NormalizeHex(t)
	out["error"] = NormalizeHex(e)
	return out
}

func applyProfile(in map[string]string, profile, mode string) map[string]string {
	return applyStyleFamily(in, profile, mode)
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
