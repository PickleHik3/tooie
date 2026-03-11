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
