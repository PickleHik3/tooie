package theme

import "testing"

func TestBuildPresetRoles(t *testing.T) {
	roles, mode, err := BuildPresetRoles("catppuccin", "mocha")
	if err != nil {
		t.Fatalf("BuildPresetRoles() error: %v", err)
	}
	if mode != "dark" {
		t.Fatalf("expected dark mode, got %q", mode)
	}
	if roles["background"] == "" || roles["primary"] == "" {
		t.Fatalf("expected core roles to be set: %#v", roles)
	}
}

func TestComputeContrastInvariants(t *testing.T) {
	roles, _, err := BuildPresetRoles("gruvbox", "dark")
	if err != nil {
		t.Fatalf("BuildPresetRoles() error: %v", err)
	}
	res, err := Compute(roles, Options{Mode: "dark", StatusPalette: "default", Profile: "adaptive"})
	if err != nil {
		t.Fatalf("Compute() error: %v", err)
	}
	if contrastRatio(res.Foreground, res.Background) < 7.0 {
		t.Fatalf("foreground contrast below 7.0: fg=%s bg=%s", res.Foreground, res.Background)
	}
	if contrastRatio(res.Cursor, res.Background) < 4.5 {
		t.Fatalf("cursor contrast below 4.5: cursor=%s bg=%s", res.Cursor, res.Background)
	}
	for i := 0; i <= 21; i++ {
		if _, ok := res.Colors[i]; !ok {
			t.Fatalf("missing terminal color index %d", i)
		}
	}
}

func TestComputeDarkModeForcesLightForegroundPolarity(t *testing.T) {
	roles := map[string]string{
		"background":    "#090a0c",
		"on_background": "#101214",
		"primary":       "#4f8dff",
		"secondary":     "#31c6c9",
		"tertiary":      "#cf63ff",
		"error":         "#ef4444",
	}
	res, err := Compute(roles, Options{Mode: "dark", StatusPalette: "default", Profile: "adaptive"})
	if err != nil {
		t.Fatalf("Compute() error: %v", err)
	}
	if relLuma(res.Foreground) <= relLuma(res.Background) {
		t.Fatalf("expected light foreground on dark mode; fg=%s bg=%s", res.Foreground, res.Background)
	}
	if contrastRatio(res.Foreground, res.Background) < 7.0 {
		t.Fatalf("foreground contrast below 7.0 after polarity guard: fg=%s bg=%s", res.Foreground, res.Background)
	}
}

func TestParseMatugenColorsForModeTemplateShape(t *testing.T) {
	raw := []byte(`{
		"colors": {
			"dark": {
				"background": "#101010",
				"primary": "#3366ff",
				"on_background": "#f0f0f0"
			},
			"light": {
				"background": "#fafafa",
				"primary": "#0044cc",
				"on_background": "#121212"
			}
		}
	}`)
	dark, err := ParseMatugenColorsForMode(raw, "dark")
	if err != nil {
		t.Fatalf("ParseMatugenColorsForMode(dark) error: %v", err)
	}
	if dark["background"] != "#101010" || dark["primary"] != "#3366ff" {
		t.Fatalf("unexpected dark roles: %#v", dark)
	}
	light, err := ParseMatugenColorsForMode(raw, "light")
	if err != nil {
		t.Fatalf("ParseMatugenColorsForMode(light) error: %v", err)
	}
	if light["background"] != "#fafafa" || light["primary"] != "#0044cc" {
		t.Fatalf("unexpected light roles: %#v", light)
	}
}

func TestComputeAddsTextSafeRoles(t *testing.T) {
	roles := map[string]string{
		"background": "#4f5062",
		"primary":    "#7b8ad9",
		"secondary":  "#5c7547",
		"tertiary":   "#9f80ba",
		"error":      "#cc6d72",
	}
	res, err := Compute(roles, Options{Source: SourceWallpaper, Mode: "dark", StatusPalette: "default"})
	if err != nil {
		t.Fatalf("Compute() error: %v", err)
	}
	for _, key := range []string{
		"text_primary",
		"text_muted",
		"text_accent_primary",
		"text_accent_secondary",
		"text_info_bg",
		"text_info_fg",
		"text_action_bg",
		"text_action_fg",
	} {
		if !IsHexColor(res.Roles[key]) {
			t.Fatalf("expected %s role to be populated, got %q", key, res.Roles[key])
		}
	}
	if contrastRatio(res.Roles["text_primary"], res.Background) < 4.5 {
		t.Fatalf("text_primary contrast too low: %s on %s", res.Roles["text_primary"], res.Background)
	}
}

func TestComputeGuardsGreenAnsiReadability(t *testing.T) {
	roles := map[string]string{
		"background": "#4f5062",
		"primary":    "#7080d1",
		"secondary":  "#4e6d3f",
		"tertiary":   "#9f7fc0",
		"error":      "#cc6d72",
	}
	res, err := Compute(roles, Options{Source: SourceWallpaper, Mode: "dark", StatusPalette: "default"})
	if err != nil {
		t.Fatalf("Compute() error: %v", err)
	}
	if contrastRatio(res.Colors[2], res.Background) < 4.4 {
		t.Fatalf("green ansi contrast too low: c2=%s bg=%s", res.Colors[2], res.Background)
	}
}
