package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallManagedPathsLinuxExcludesTermuxFiles(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "tmp", "home")
	settings := defaultTooieSettings()
	settings.Modules.TermuxAppearance = false
	env := setupEnv{IsTermux: false}

	paths := installManagedPaths(home, settings, env)
	for _, p := range paths {
		if strings.Contains(p, string(filepath.Separator)+".termux"+string(filepath.Separator)) {
			t.Fatalf("linux paths should not include termux entries, got %q", p)
		}
	}
}

func TestInstallManagedPathsTermuxIncludesTermuxFiles(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "data", "data", "com.termux", "files", "home")
	settings := defaultTooieSettings()
	settings.Modules.TermuxAppearance = true
	env := setupEnv{IsTermux: true}

	paths := installManagedPaths(home, settings, env)
	var termuxCount int
	for _, p := range paths {
		if strings.Contains(p, string(filepath.Separator)+".termux"+string(filepath.Separator)) ||
			strings.Contains(p, string(filepath.Separator)+".config"+string(filepath.Separator)+"termux"+string(filepath.Separator)) {
			termuxCount++
		}
	}
	if termuxCount != 8 {
		t.Fatalf("expected 8 termux paths, got %d (%v)", termuxCount, paths)
	}
}
