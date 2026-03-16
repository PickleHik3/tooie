package main

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPageNavigationThreePages(t *testing.T) {
	m := model{page: pageHome}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m1 := next.(model)
	if m1.page != pageSettings {
		t.Fatalf("page after right from home = %d, want %d", m1.page, pageSettings)
	}

	next, _ = m1.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m2 := next.(model)
	if m2.page != pageTheme {
		t.Fatalf("page after right from settings = %d, want %d", m2.page, pageTheme)
	}

	next, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m3 := next.(model)
	if m3.page != pageHome {
		t.Fatalf("page after right from theme = %d, want %d", m3.page, pageHome)
	}

	next, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m4 := next.(model)
	if m4.page != pageTheme {
		t.Fatalf("page after left from home = %d, want %d", m4.page, pageTheme)
	}
}

func TestPageLabelIncludesSettings(t *testing.T) {
	cases := []struct {
		page int
		want string
	}{
		{page: pageHome, want: "Tooie"},
		{page: pageTheme, want: "Theme"},
		{page: pageSettings, want: "Settings"},
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
