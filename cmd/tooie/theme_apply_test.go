package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testThemePayload() computedPayload {
	p := computedPayload{}
	p.Foreground = "#e4e3d7"
	p.Background = "#13140d"
	p.Cursor = "#c0ce7e"
	p.Roles = map[string]string{
		"primary":   "#c5c9a8",
		"secondary": "#c0ce7e",
		"tertiary":  "#a1d0c4",
		"error":     "#ffb4ab",
	}
	p.Meta = map[string]string{"status_palette": "vibrant"}
	p.Colors = map[int]string{}
	for i := 0; i <= 21; i++ {
		p.Colors[i] = "#111111"
	}
	p.Colors[1] = "#ffb4ab"
	p.Colors[2] = "#c0ce7e"
	p.Colors[3] = "#a1d0c4"
	p.Colors[4] = "#c5c9a8"
	p.Colors[5] = "#414c08"
	p.Colors[6] = "#214e45"
	p.Colors[7] = "#e4e3d7"
	p.Colors[10] = "#414c08"
	p.Colors[14] = "#919283"
	p.Colors[15] = "#c7c7b7"
	p.Status.Separator = "#919283"
	p.Status.Weather = "#c0ce7e"
	p.Status.Charging = "#c0ce7e"
	p.Status.Battery = []string{"#ffb4ab", "#a1d0c4", "#214e45", "#c5c9a8", "#45492f", "#c0ce7e"}
	p.Status.CPU = []string{"#c0ce7e", "#414c08", "#a1d0c4", "#214e45", "#93000a", "#ffb4ab"}
	p.Status.RAM = []string{"#214e45", "#45492f", "#586420", "#a1d0c4", "#93000a", "#ffb4ab"}
	return p
}

func TestRenderTmuxBlockIncludesPaletteKey(t *testing.T) {
	got := renderTmuxBlock(testThemePayload())
	if !strings.Contains(got, `set -g @status-tmux-palette "vibrant"`) {
		t.Fatalf("renderTmuxBlock() missing @status-tmux-palette: %s", got)
	}
}

func TestApplyThemeFilesCreatesBackupsAndIdempotentBlocks(t *testing.T) {
	tmp := t.TempDir()
	oldHome, oldCfg := homeDir, tooieConfigDir
	homeDir = tmp
	tooieConfigDir = filepath.Join(tmp, ".config", "tooie")
	t.Cleanup(func() {
		homeDir = oldHome
		tooieConfigDir = oldCfg
	})

	termuxColors := filepath.Join(tmp, ".termux", "colors.properties")
	tmuxConf := filepath.Join(tmp, ".tmux.conf")
	peaclockCfg := filepath.Join(tmp, ".config", "peaclock", "config")
	starshipCfg := filepath.Join(tmp, ".config", "starship.toml")

	mustWrite := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(termuxColors, "foreground=#fff\n")
	mustWrite(tmuxConf, "set -g status on\n")
	mustWrite(peaclockCfg, "style text white\n")
	mustWrite(starshipCfg, "[character]\nsuccess_symbol='x'\n")

	backupDir := filepath.Join(tmp, "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := testThemePayload()

	if err := applyThemeFiles(payload, backupDir); err != nil {
		t.Fatalf("applyThemeFiles() error: %v", err)
	}
	for _, rel := range []string{"colors.properties.bak", "tmux.conf.bak", "peaclock.config.bak", "starship.toml.bak"} {
		if _, err := os.Stat(filepath.Join(backupDir, rel)); err != nil {
			t.Fatalf("missing backup %s: %v", rel, err)
		}
	}

	if err := applyThemeFiles(payload, backupDir); err != nil {
		t.Fatalf("second applyThemeFiles() error: %v", err)
	}
	raw, err := os.ReadFile(tmuxConf)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Count(body, "# >>> MATUGEN THEME START >>>") != 1 {
		t.Fatalf("tmux block duplicated: %s", body)
	}
	if strings.Count(body, "# <<< MATUGEN THEME END <<<") != 1 {
		t.Fatalf("tmux block end duplicated: %s", body)
	}
}
