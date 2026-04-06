package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func withTempTooieSettingsHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	oldHome, oldCfg := homeDir, tooieConfigDir
	homeDir = tmp
	tooieConfigDir = filepath.Join(tmp, ".config", "tooie")
	t.Cleanup(func() {
		homeDir = oldHome
		tooieConfigDir = oldCfg
	})
}

func TestPageNavigationTwoPages(t *testing.T) {
	m := model{page: pageHome}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m1 := next.(model)
	if m1.page != pageTheme {
		t.Fatalf("page after right from home = %d, want %d", m1.page, pageTheme)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m2 := next.(model)
	if m2.page != pageHome {
		t.Fatalf("page after right from theme = %d, want %d", m2.page, pageHome)
	}

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m3 := next.(model)
	if m3.page != pageTheme {
		t.Fatalf("page after left from home = %d, want %d", m3.page, pageTheme)
	}
}

func TestPageLabelIncludesTheme(t *testing.T) {
	cases := []struct {
		page int
		want string
	}{
		{page: pageHome, want: "Tooie"},
		{page: pageTheme, want: "Theme"},
	}
	for _, tc := range cases {
		m := model{page: tc.page}
		if got := m.pageLabel(); got != tc.want {
			t.Fatalf("pageLabel(%d) = %q, want %q", tc.page, got, tc.want)
		}
	}
}

func TestLoadShellSettingsFromBackups(t *testing.T) {
	settings := loadShellSettingsFromBackups([]backup{{Meta: map[string]string{
		"widget_battery": "off",
		"widget_cpu":     "on",
		"widget_ram":     "0",
		"widget_weather": "1",
	}}})
	want := persistedShellSettings{
		WidgetBattery: false,
		WidgetCPU:     true,
		WidgetRAM:     false,
		WidgetWeather: true,
	}
	if !reflect.DeepEqual(settings, want) {
		t.Fatalf("loadShellSettingsFromBackups() = %#v, want %#v", settings, want)
	}
}

func TestPersistedShellSettingsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	in := persistedShellSettings{
		WidgetBattery: false,
		WidgetCPU:     true,
		WidgetRAM:     true,
		WidgetWeather: false,
	}
	if err := savePersistedShellSettings(in); err != nil {
		t.Fatalf("savePersistedShellSettings() error: %v", err)
	}
	out, ok := loadPersistedShellSettings()
	if !ok {
		t.Fatalf("loadPersistedShellSettings() expected ok")
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip = %#v, want %#v", out, in)
	}
}

func TestStarshipPromptRowShowsNAWhenStarshipIsOff(t *testing.T) {
	withTempTooieSettingsHome(t)
	settings := defaultTooieSettings()
	settings.Modules.StarshipMode = "off"
	if err := saveTooieSettings(settings); err != nil {
		t.Fatalf("saveTooieSettings() error: %v", err)
	}

	m := model{starshipPrompt: defaultStarship}
	_, state, _, _ := m.settingsRowView("starship_prompt")
	if state != "N/A" {
		t.Fatalf("starship prompt state = %q, want N/A", state)
	}
	if got := m.settingMenuChoices("starship_prompt"); len(got) != 0 {
		t.Fatalf("starship prompt choices should be empty when starship is off")
	}
}

func TestStarshipPromptRowListsHotSwapOptions(t *testing.T) {
	withTempTooieSettingsHome(t)
	settings := defaultTooieSettings()
	settings.Modules.StarshipMode = "themed"
	if err := saveTooieSettings(settings); err != nil {
		t.Fatalf("saveTooieSettings() error: %v", err)
	}

	m := model{starshipPrompt: "jetpack"}
	label, state, _, _ := m.settingsRowView("starship_prompt")
	if !strings.Contains(label, "Starship") {
		t.Fatalf("starship label should be Starship, got %q", label)
	}
	if state != "Jetpack ▾" {
		t.Fatalf("starship prompt state = %q, want %q", state, "Jetpack ▾")
	}
}

func TestStarshipPromptMenuSelectionAutoApplies(t *testing.T) {
	m := model{
		page:              pageTheme,
		themeSource:       defaultSource,
		mode:              defaultMode,
		profile:           defaultProfile,
		statusTheme:       defaultStatusTheme,
		settingMenuTarget: "starship_prompt",
		settingMenuIndex:  1,
		lastAppliedTheme:  "stale",
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if cmd == nil {
		t.Fatalf("expected command for auto-apply after starship preset change")
	}
	if !got.applying {
		t.Fatalf("expected auto-apply state after starship preset change")
	}
}

func TestRenderThemePageShowsMergedMatrix(t *testing.T) {
	m := model{
		page:          pageTheme,
		themeSource:   defaultSource,
		mode:          defaultMode,
		profile:       defaultProfile,
		widgetBattery: true,
		widgetCPU:     true,
		widgetRAM:     false,
		widgetWeather: true,
		selectedHexes: map[string]string{
			"primary":    "#aeb1f4",
			"secondary":  "#a9cee2",
			"tertiary":   "#e1a4d3",
			"error":      "#fa8c87",
			"surface":    "#1d2024",
			"on_surface": "#e1e2e9",
		},
	}

	got := m.renderThemePage(96, 18)
	for _, want := range []string{
		"Colors",
		"Misc",
		"tmux status",
		"Update Colors",
		"Backups",
		"Apply",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderThemePage() missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Details", "Current Theme", "Setup Btop", "Reset Bootstrap"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("renderThemePage() unexpectedly contains %q in:\n%s", unwanted, got)
		}
	}
}

func TestLowercaseAOpensApplyConfirmOnThemePage(t *testing.T) {
	m := model{
		page:        pageTheme,
		themeSource: defaultSource,
		mode:        defaultMode,
		profile:     defaultProfile,
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := next.(model)
	if !got.showApplyConfirm {
		t.Fatalf("expected lowercase a to open apply confirmation")
	}
}

func TestUppercaseAStillAppliesImmediatelyOnThemePage(t *testing.T) {
	m := model{
		page:             pageTheme,
		themeSource:      defaultSource,
		mode:             defaultMode,
		profile:          defaultProfile,
		statusTheme:      defaultStatusTheme,
		widgetBattery:    true,
		widgetCPU:        true,
		widgetRAM:        true,
		widgetWeather:    true,
		lastAppliedTheme: "stale",
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	got := next.(model)
	if cmd == nil || !got.applying {
		t.Fatalf("expected uppercase A to start immediate apply")
	}
}

func TestKeybindsCycleThemeSettings(t *testing.T) {
	m := model{
		page:        pageTheme,
		themeSource: defaultSource,
		mode:        defaultMode,
		profile:     defaultProfile,
		statusTheme: defaultStatusTheme,
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := next.(model)
	if got.themeSource == defaultSource {
		t.Fatalf("expected s to cycle source")
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	got = next.(model)
	if got.statusTheme == defaultStatusTheme {
		t.Fatalf("expected t to cycle tmux theme")
	}
}

func TestKeybindPCyclesPaletteType(t *testing.T) {
	m := model{
		page:        pageTheme,
		themeSource: defaultSource,
		mode:        defaultMode,
		profile:     defaultProfile,
		paletteType: defaultPaletteType,
		statusTheme: defaultStatusTheme,
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := next.(model)
	if got.paletteType == defaultPaletteType {
		t.Fatalf("expected p to cycle palette type")
	}
}

func TestKeybindPDoesNotCyclePaletteTypeForPreset(t *testing.T) {
	m := model{
		page:        pageTheme,
		themeSource: "preset",
		mode:        defaultMode,
		profile:     defaultProfile,
		paletteType: defaultPaletteType,
		statusTheme: defaultStatusTheme,
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	got := next.(model)
	if got.paletteType != defaultPaletteType {
		t.Fatalf("paletteType changed for preset source: %q", got.paletteType)
	}
}

func TestSourceAndModeDoNotMutateProfileViaHotkeys(t *testing.T) {
	m := model{
		page:        pageTheme,
		themeSource: defaultSource,
		mode:        defaultMode,
		profile:     "source-3",
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := next.(model)
	if got.profile != "source-3" {
		t.Fatalf("profile changed after source hotkey: got %q", got.profile)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got = next.(model)
	if got.profile != "source-3" {
		t.Fatalf("profile changed after mode hotkey: got %q", got.profile)
	}
}

func TestRequestThemeApplyNoopsWithoutChanges(t *testing.T) {
	m := model{
		themeSource:      defaultSource,
		mode:             defaultMode,
		profile:          defaultProfile,
		statusTheme:      defaultStatusTheme,
		widgetBattery:    true,
		widgetCPU:        true,
		widgetRAM:        true,
		widgetWeather:    true,
		lastAppliedTheme: "",
	}
	m.lastAppliedTheme = m.applyCacheSignature()

	next, cmd := m.requestThemeApply()
	got := next.(model)
	if cmd != nil {
		t.Fatalf("requestThemeApply() returned command for unchanged theme")
	}
	if got.lastStatus != "No theme changes to apply" {
		t.Fatalf("lastStatus = %q", got.lastStatus)
	}
}

func TestRequestThemeApplyStartsWhenChanged(t *testing.T) {
	m := model{
		themeSource:      defaultSource,
		mode:             defaultMode,
		profile:          defaultProfile,
		statusTheme:      defaultStatusTheme,
		widgetBattery:    true,
		widgetCPU:        true,
		widgetRAM:        true,
		widgetWeather:    true,
		lastAppliedTheme: "stale",
	}

	next, cmd := m.requestThemeApply()
	got := next.(model)
	if cmd == nil {
		t.Fatalf("requestThemeApply() expected command when theme changed")
	}
	if !got.applying {
		t.Fatalf("requestThemeApply() should enter applying state")
	}
}

func TestToggleSegmentPersists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := model{
		themeSource:   defaultSource,
		mode:          defaultMode,
		profile:       defaultProfile,
		statusTheme:   defaultStatusTheme,
		widgetBattery: true,
		widgetCPU:     true,
		widgetRAM:     false,
		widgetWeather: true,
	}
	m.toggleSegment("widget_battery")
	if m.widgetBattery {
		t.Fatalf("widget battery should toggle off")
	}
	if m.switchAnimTarget != "widget_battery" {
		t.Fatalf("switch animation target = %q, want widget_battery", m.switchAnimTarget)
	}
	if m.switchAnimProg != 0 {
		t.Fatalf("switch animation progress = %v, want 0", m.switchAnimProg)
	}

	settings, ok := loadPersistedShellSettings()
	if !ok {
		t.Fatalf("loadPersistedShellSettings() expected ok after widget toggle")
	}
	if settings.WidgetBattery {
		t.Fatalf("persisted battery toggle should be off, got %#v", settings)
	}
}

func TestTwoColumnWidthsBiasesLeftColumn(t *testing.T) {
	left, right := twoColumnWidths(96)
	if left <= 0 || right <= 0 {
		t.Fatalf("twoColumnWidths() returned invalid widths: left=%d right=%d", left, right)
	}
	if left <= right {
		t.Fatalf("expected left column wider than right: left=%d right=%d", left, right)
	}
	if left-right < 2 {
		t.Fatalf("expected left column to be at least 2 chars wider: left=%d right=%d", left, right)
	}
}

func TestWallpaperBlockFallbackUsesInsetSpacing(t *testing.T) {
	m := model{}
	got := m.wallpaperBlock(8, 24)
	lines := strings.Split(got, "\n")
	foundContent := false
	for i, line := range lines {
		if i < 2 {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		foundContent = true
		if !strings.HasPrefix(line, " ") {
			t.Fatalf("wallpaperBlock() content line missing left inset padding: %q", line)
		}
		break
	}
	if !foundContent {
		t.Fatalf("wallpaperBlock() did not render any content lines:\n%s", got)
	}
}

func TestAdvanceSwitchAnimationClearsTarget(t *testing.T) {
	m := model{}
	m.startSwitchAnimation("widget_cpu", false, true)
	if m.switchAnimTarget != "widget_cpu" {
		t.Fatalf("switchAnimTarget = %q", m.switchAnimTarget)
	}
	m.advanceSwitchAnimation(0.25)
	if m.switchAnimTarget != "" {
		t.Fatalf("switch animation should finish and clear target, got %q", m.switchAnimTarget)
	}
	if m.switchAnimProg != 1 {
		t.Fatalf("switchAnimProg = %v, want 1", m.switchAnimProg)
	}
}

func TestEnterOnListSettingOpensDropdown(t *testing.T) {
	m := model{
		page:        pageTheme,
		themeSource: defaultSource,
		mode:        defaultMode,
		profile:     defaultProfile,
	}
	for i, item := range m.mergedPageItems() {
		if item.Target == "theme_source" {
			m.settingIndex = i
			break
		}
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.settingMenuTarget != "theme_source" {
		t.Fatalf("settingMenuTarget = %q, want theme_source", got.settingMenuTarget)
	}
}

func TestDropdownEnterAppliesSelection(t *testing.T) {
	m := model{
		page:              pageTheme,
		themeSource:       "wallpaper",
		settingMenuTarget: "theme_source",
		settingMenuIndex:  1,
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.themeSource != "preset" {
		t.Fatalf("themeSource = %q, want preset", got.themeSource)
	}
	if got.settingMenuTarget != "" {
		t.Fatalf("expected dropdown to close after selection")
	}
}

func TestDropdownSourceAndModeDoNotMutateProfile(t *testing.T) {
	m := model{
		page:              pageTheme,
		themeSource:       "wallpaper",
		mode:              "dark",
		profile:           "source-3",
		settingMenuTarget: "theme_source",
		settingMenuIndex:  1,
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if got.themeSource != "preset" {
		t.Fatalf("themeSource = %q, want preset", got.themeSource)
	}
	if got.profile != "source-3" {
		t.Fatalf("profile changed after source dropdown: got %q", got.profile)
	}

	got.settingMenuTarget = "mode"
	got.settingMenuIndex = 0
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(model)
	if got.mode != "dark" {
		t.Fatalf("mode = %q, want dark", got.mode)
	}
	if got.profile != "source-3" {
		t.Fatalf("profile changed after mode dropdown: got %q", got.profile)
	}
}
