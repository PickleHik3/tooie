package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const tooieSettingsVersion = 5

type tmuxSetupOptions struct {
	Mode            string `json:"mode"`
	StatusPosition  string `json:"status_position"`
	StatusLayout    string `json:"status_layout"`
	StatusSeparator string `json:"status_separator"`
}

type setupModules struct {
	TmuxTheme        bool   `json:"tmux_theme"`
	TermuxAppearance bool   `json:"termux_appearance"`
	FishBootstrap    bool   `json:"fish_bootstrap"`
	StarshipMode     string `json:"starship_mode"`
	ShellTheme       bool   `json:"shell_theme"`
	PeaclockTheme    bool   `json:"peaclock_theme"`
	BtopHelper       bool   `json:"btop_helper"`
}

type setupPlatformOptions struct {
	Profile string `json:"profile"`
}

type privilegedOptions struct {
	Runner string `json:"runner"`
}

type linuxThemeOptions struct {
	TerminalTarget string `json:"terminal_target"`
}

type starshipOptions struct {
	Prompt string `json:"prompt"`
}

type tooieSettings struct {
	Version    int                    `json:"version"`
	Tmux       tmuxSetupOptions       `json:"tmux"`
	Widgets    persistedShellSettings `json:"widgets"`
	Modules    setupModules           `json:"modules"`
	Platform   setupPlatformOptions   `json:"platform"`
	Privileged privilegedOptions      `json:"privileged"`
	Linux      linuxThemeOptions      `json:"linux"`
	Starship   starshipOptions        `json:"starship"`
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
			TmuxTheme:        true,
			TermuxAppearance: true,
			FishBootstrap:    true,
			StarshipMode:     "themed",
			ShellTheme:       true,
			PeaclockTheme:    true,
			BtopHelper:       false,
		},
		Platform:   setupPlatformOptions{Profile: "termux"},
		Privileged: privilegedOptions{Runner: "auto"},
		Linux:      linuxThemeOptions{TerminalTarget: "ghostty"},
		Starship:   starshipOptions{Prompt: defaultStarship},
	}
}

func tooieSettingsPath() string {
	home := strings.TrimSpace(homeDir)
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if strings.TrimSpace(home) == "" {
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
	case "root":
		return "root"
	case "su":
		return "su"
	case "tsu":
		return "tsu"
	case "sudo":
		return "sudo"
	default:
		return "auto"
	}
}

func normalizePlatformProfile(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "termux-root", "termux_root":
		return "termux-root"
	case "termux-shizuku", "termux_shizuku":
		return "termux-shizuku"
	case "termux-rish", "termux_rish":
		return "termux-rish"
	case "linux":
		return "linux"
	default:
		return "termux"
	}
}

func normalizeLinuxTerminalTarget(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "ghostty":
		return "ghostty"
	default:
		return "ghostty"
	}
}

func normalizeStarshipInstallMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "default":
		return "default"
	case "themed":
		return "themed"
	default:
		return "off"
	}
}

func cleanupLegacyThemeArtifacts() {
	_ = os.RemoveAll(filepath.Join(tooieConfigDir, "backups"))
	_ = os.Remove(filepath.Join(tooieConfigDir, "cache", "extract-swatches.json"))
	_ = os.Remove(filepath.Join(tooieConfigDir, "cache", "theme-preview.json"))
	_ = os.Remove(filepath.Join(tooieConfigDir, "cache", "theme-apply-progress.json"))
}

func normalizeTooieSettings(s *tooieSettings) {
	if s.Version <= 0 {
		s.Version = tooieSettingsVersion
	}
	if s.Version < 3 && !s.Modules.TmuxTheme {
		s.Modules.TmuxTheme = true
	}
	if s.Version < 4 {
		cleanupLegacyThemeArtifacts()
	}
	if s.Version < 5 {
		if s.Modules.ShellTheme {
			s.Modules.FishBootstrap = true
			if normalizeStarshipInstallMode(s.Modules.StarshipMode) == "off" {
				s.Modules.StarshipMode = "themed"
			}
		}
	}
	if s.Version < tooieSettingsVersion {
		s.Version = tooieSettingsVersion
	}
	s.Tmux.Mode = normalizeTmuxMode(s.Tmux.Mode)
	s.Tmux.StatusPosition = normalizeStatusPosition(s.Tmux.StatusPosition)
	s.Tmux.StatusLayout = normalizeStatusLayout(s.Tmux.StatusLayout)
	s.Tmux.StatusSeparator = normalizeSeparatorMode(s.Tmux.StatusSeparator)
	s.Platform.Profile = normalizePlatformProfile(s.Platform.Profile)
	s.Privileged.Runner = normalizeRunner(s.Privileged.Runner)
	s.Linux.TerminalTarget = normalizeLinuxTerminalTarget(s.Linux.TerminalTarget)
	s.Modules.StarshipMode = normalizeStarshipInstallMode(s.Modules.StarshipMode)
	s.Modules.ShellTheme = s.Modules.FishBootstrap || s.Modules.StarshipMode != "off"
	s.Starship.Prompt = normalizeStarshipPrompt(s.Starship.Prompt)
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
