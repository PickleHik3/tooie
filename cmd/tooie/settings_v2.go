package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const tooieSettingsVersion = 2

type tmuxSetupOptions struct {
	Mode          string `json:"mode"`
	StatusPosition string `json:"status_position"`
	StatusLayout   string `json:"status_layout"`
	StatusSeparator string `json:"status_separator"`
}

type setupModules struct {
	TermuxAppearance bool `json:"termux_appearance"`
	ShellTheme       bool `json:"shell_theme"`
	PeaclockTheme    bool `json:"peaclock_theme"`
	BtopHelper       bool `json:"btop_helper"`
}

type privilegedOptions struct {
	Runner string `json:"runner"`
}

type tooieSettings struct {
	Version    int                   `json:"version"`
	Tmux       tmuxSetupOptions      `json:"tmux"`
	Widgets    persistedShellSettings `json:"widgets"`
	Modules    setupModules          `json:"modules"`
	Privileged privilegedOptions     `json:"privileged"`
}

func defaultTooieSettings() tooieSettings {
	return tooieSettings{
		Version: tooieSettingsVersion,
		Tmux: tmuxSetupOptions{
			Mode:            "full",
			StatusPosition:  "top",
			StatusLayout:    "two-line",
			StatusSeparator: "on",
		},
		Widgets: defaultShellSettings(),
		Modules: setupModules{
			TermuxAppearance: true,
			ShellTheme:       true,
			PeaclockTheme:    true,
			BtopHelper:       false,
		},
		Privileged: privilegedOptions{Runner: "auto"},
	}
}

func tooieSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "tooie-settings.json"
	}
	return filepath.Join(home, ".config", "tooie", "settings.json")
}

func normalizeTmuxMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "status", "status-only", "status_only":
		return "status-only"
	default:
		return "full"
	}
}

func normalizeRunner(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "rish":
		return "rish"
	case "su":
		return "su"
	case "tsu":
		return "tsu"
	default:
		return "auto"
	}
}

func normalizeTooieSettings(s *tooieSettings) {
	if s.Version <= 0 {
		s.Version = tooieSettingsVersion
	}
	s.Tmux.Mode = normalizeTmuxMode(s.Tmux.Mode)
	s.Tmux.StatusPosition = normalizeStatusPosition(s.Tmux.StatusPosition)
	s.Tmux.StatusLayout = normalizeStatusLayout(s.Tmux.StatusLayout)
	s.Tmux.StatusSeparator = normalizeSeparatorMode(s.Tmux.StatusSeparator)
	s.Privileged.Runner = normalizeRunner(s.Privileged.Runner)
	if s.Tmux.StatusLayout == "single-line" {
		s.Tmux.StatusSeparator = "off"
	}
}

func loadTooieSettings() (tooieSettings, bool) {
	out := defaultTooieSettings()
	raw, err := os.ReadFile(tooieSettingsPath())
	if err != nil {
		return out, false
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return defaultTooieSettings(), false
	}
	normalizeTooieSettings(&out)
	return out, true
}

func saveTooieSettings(s tooieSettings) error {
	normalizeTooieSettings(&s)
	path := tooieSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func migrateLegacyIntoTooieSettings() tooieSettings {
	settings, ok := loadTooieSettings()
	if ok {
		return settings
	}
	settings = defaultTooieSettings()
	legacy, ok := loadLegacyShellSettings()
	if ok {
		settings.Widgets = legacy
	}
	return settings
}
