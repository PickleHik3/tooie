package main

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
		"Status Bar",
		"Battery: on",
		"Weather: on",
		"Apply (Shift+A)",
		"#aeb1f4 primary",
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

func TestActivateSettingTogglesWidgetAndPersists(t *testing.T) {
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
	for i, item := range m.mergedPageItems() {
		if item.Target == "widget_battery" {
			m.settingIndex = i
			break
		}
	}

	next, cmd := m.activateSetting()
	got := next.(model)
	if got.widgetBattery {
		t.Fatalf("widget battery should toggle off")
	}
	if cmd == nil {
		t.Fatalf("activateSetting() should return sync command for widget toggle")
	}

	settings, ok := loadPersistedShellSettings()
	if !ok {
		t.Fatalf("loadPersistedShellSettings() expected ok after widget toggle")
	}
	if settings.WidgetBattery {
		t.Fatalf("persisted battery toggle should be off, got %#v", settings)
	}
}
