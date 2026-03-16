package main

import (
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseLauncherComponentLines(t *testing.T) {
	raw := "\ncom.termux/.app.TermuxActivity\njunk line\npriority 0 com.android.chrome/com.google.android.apps.chrome.Main\n\n"
	got := parseLauncherComponentLines(raw)
	want := []string{
		"com.termux/.app.TermuxActivity",
		"com.android.chrome/com.google.android.apps.chrome.Main",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLauncherComponentLines() = %#v, want %#v", got, want)
	}
}

func TestSplitComponent(t *testing.T) {
	pkg, activity, ok := splitComponent("com.termux/.app.TermuxActivity")
	if !ok {
		t.Fatal("splitComponent returned ok=false")
	}
	if pkg != "com.termux" {
		t.Fatalf("package = %q, want %q", pkg, "com.termux")
	}
	if activity != ".app.TermuxActivity" {
		t.Fatalf("activity = %q, want %q", activity, ".app.TermuxActivity")
	}
}

func TestParseResolvedComponent(t *testing.T) {
	raw := "priority=0 preferredOrder=0 match=0x108000 specificIndex=-1 isDefault=true\ncom.android.chrome/com.google.android.apps.chrome.Main\n"
	got := parseResolvedComponent(raw)
	want := "com.android.chrome/com.google.android.apps.chrome.Main"
	if got != want {
		t.Fatalf("parseResolvedComponent() = %q, want %q", got, want)
	}
}

func TestDecodeDataURLImage(t *testing.T) {
	rawPNG := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(rawPNG)
	gotRaw, gotType, ok := decodeDataURLImage(dataURL)
	if !ok {
		t.Fatal("decodeDataURLImage returned ok=false")
	}
	if gotType != "image/png" {
		t.Fatalf("contentType = %q, want image/png", gotType)
	}
	if !reflect.DeepEqual(gotRaw, rawPNG) {
		t.Fatalf("raw = %#v, want %#v", gotRaw, rawPNG)
	}
}

func TestImageExtensionByMagic(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	if got := imageExtension("", png); got != ".png" {
		t.Fatalf("imageExtension(png) = %q, want .png", got)
	}
	webp := []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P'}
	if got := imageExtension("", webp); got != ".webp" {
		t.Fatalf("imageExtension(webp) = %q, want .webp", got)
	}
	jpg := []byte{0xff, 0xd8, 0xff, 0x00}
	if got := imageExtension("", jpg); got != ".jpg" {
		t.Fatalf("imageExtension(jpg) = %q, want .jpg", got)
	}
}

func TestIconEndpointCandidates(t *testing.T) {
	got := iconEndpointCandidates("com.termux")
	want := []string{
		"/v1/apps/icon/com.termux",
		"/v1/apps/com.termux/icon",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("iconEndpointCandidates() = %#v, want %#v", got, want)
	}
}

func TestIsRestartCLIArgs(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"--restart"}, want: true},
		{args: []string{"-restart"}, want: true},
		{args: []string{"restart"}, want: false},
		{args: []string{"--restart", "extra"}, want: false},
		{args: nil, want: false},
	}
	for _, tt := range tests {
		if got := isRestartCLIArgs(tt.args); got != tt.want {
			t.Fatalf("isRestartCLIArgs(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestTrimTransparentImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 6, 6))
	img.Set(2, 1, color.RGBA{255, 0, 0, 255})
	img.Set(3, 4, color.RGBA{0, 255, 0, 255})

	trimmed := trimTransparentImage(img)
	if trimmed.Bounds().Dx() != 2 || trimmed.Bounds().Dy() != 4 {
		t.Fatalf("trimmed size = %dx%d, want 2x4", trimmed.Bounds().Dx(), trimmed.Bounds().Dy())
	}
}

func TestRenderTinyImageANSI(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(1, 0, color.RGBA{0, 255, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})
	img.Set(1, 1, color.RGBA{255, 255, 0, 255})

	rendered := renderTinyImageANSI(img, 2, 1)
	if !strings.Contains(rendered, "▀") {
		t.Fatalf("renderTinyImageANSI() missing block rune: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[38;2;255;0;0m") {
		t.Fatalf("renderTinyImageANSI() missing top ANSI color: %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[48;2;0;0;255m") {
		t.Fatalf("renderTinyImageANSI() missing bottom ANSI color: %q", rendered)
	}
}

func TestRenderPinnedAppIconCachesRenderedValue(t *testing.T) {
	tmpDir := t.TempDir()
	iconPath := filepath.Join(tmpDir, "termux.png")

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	img.Set(1, 0, color.RGBA{255, 255, 255, 255})
	img.Set(0, 1, color.RGBA{0, 0, 0, 255})
	img.Set(1, 1, color.RGBA{0, 0, 0, 255})

	f, err := os.Create(iconPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	app := launchableApp{
		PackageName:   "com.termux",
		Label:         "Termux",
		IconCachePath: iconPath,
	}
	first := renderPinnedAppIcon(app, 2, 1)
	second := renderPinnedAppIcon(app, 2, 1)
	if first != second {
		t.Fatalf("renderPinnedAppIcon() not stable across cache reads")
	}
	if !strings.Contains(first, "▀") {
		t.Fatalf("renderPinnedAppIcon() expected rendered icon, got %q", first)
	}
}

func TestRenderPinnedAppIconBadgeFallback(t *testing.T) {
	app := launchableApp{
		PackageName: "com.termux",
		Label:       "Termux",
	}
	got := renderPinnedAppIcon(app, 4, 1)
	if !strings.Contains(got, "TE") {
		t.Fatalf("badge fallback missing text: %q", got)
	}
}

func TestApplyArgsPresetOmitsMode(t *testing.T) {
	m := model{
		themeSource:   "preset",
		mode:          "mocha",
		presetFamily:  "catppuccin",
		presetVariant: "mocha",
		palette:       "default",
		widgetBattery: true,
		widgetCPU:     true,
		widgetRAM:     true,
		widgetWeather: true,
	}

	args := m.applyArgs(false)
	for i := range args {
		if args[i] == "-m" || args[i] == "--mode" {
			t.Fatalf("applyArgs() included wallpaper mode for preset source: %v", args)
		}
	}
	if !reflect.DeepEqual(args, []string{
		"--theme-source", "preset",
		"--status-palette", "default",
		"--preset-family", "catppuccin",
		"--preset-variant", "mocha",
		"--widget-battery", "on",
		"--widget-cpu", "on",
		"--widget-ram", "on",
		"--widget-weather", "on",
	}) {
		t.Fatalf("applyArgs() = %v", args)
	}
}

func TestNormalizeThemeSelectionResetsInvalidMode(t *testing.T) {
	m := model{
		themeSource:   "preset",
		mode:          "mocha",
		presetFamily:  "catppuccin",
		presetVariant: "mocha",
		profile:       "adaptive",
	}

	m.normalizeThemeSelection()

	if m.mode != defaultMode {
		t.Fatalf("mode = %q, want %q", m.mode, defaultMode)
	}
}

func TestApplyArgsWallpaperUsesProfile(t *testing.T) {
	m := model{
		themeSource:   "wallpaper",
		mode:          "auto",
		profile:       "neon-night",
		palette:       "vibrant",
		widgetBattery: false,
		widgetCPU:     true,
		widgetRAM:     false,
		widgetWeather: true,
	}
	args := m.applyArgs(false)
	want := []string{
		"--theme-source", "wallpaper",
		"--status-palette", "vibrant",
		"-m", "auto",
		"--profile", "neon-night",
		"--widget-battery", "off",
		"--widget-cpu", "on",
		"--widget-ram", "off",
		"--widget-weather", "on",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("applyArgs() = %v, want %v", args, want)
	}
}

func TestANSIColorOptionsPreferSemanticChannel(t *testing.T) {
	m := model{
		selectedHexes: map[string]string{
			"background":          "#101218",
			"error":               "#ff5f5f",
			"error_container":     "#93000a",
			"secondary":           "#46d37b",
			"secondary_container": "#1b6a3b",
			"primary":             "#4f8dff",
			"tertiary":            "#cf63ff",
		},
	}
	opts := m.colorPickerOptions("ansi_red")
	if len(opts) < 2 {
		t.Fatalf("expected ansi red options beyond auto, got %v", opts)
	}
	if strings.ToLower(strings.TrimSpace(opts[1].Hex)) != "#ff5f5f" {
		t.Fatalf("expected first semantic red option to be error color, got %#v", opts[1])
	}
}
