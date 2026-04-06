package main

import "testing"

func TestApplyInstallPlanTermuxNoneKeepsWidgetsEnabled(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: true}
	plan := setupInstallPlan{Platform: "termux", Backend: "none", ThemeItems: "tmux"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if settings.Platform.Profile != "termux" {
		t.Fatalf("profile = %q, want termux", settings.Platform.Profile)
	}
	if settings.Modules.BtopHelper {
		t.Fatalf("btop helper should be off for backend none")
	}
	if !settings.Widgets.WidgetBattery || !settings.Widgets.WidgetCPU || !settings.Widgets.WidgetRAM || !settings.Widgets.WidgetWeather {
		t.Fatalf("termux backend none should keep all widgets enabled for unprivileged local metrics")
	}
	if !settings.Modules.TmuxTheme {
		t.Fatalf("tmux theme should be enabled for tmux item")
	}
	if settings.Modules.FishBootstrap || settings.Modules.StarshipMode != "off" || settings.Modules.TermuxAppearance {
		t.Fatalf("only tmux target should be enabled")
	}
}

func TestApplyInstallPlanTermuxRootShell(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: true}
	plan := setupInstallPlan{Platform: "termux", Backend: "root", ThemeItems: "shell"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if settings.Platform.Profile != "termux-root" {
		t.Fatalf("profile = %q, want termux-root", settings.Platform.Profile)
	}
	if settings.Privileged.Runner != "root" {
		t.Fatalf("runner = %q, want root", settings.Privileged.Runner)
	}
	if !settings.Modules.BtopHelper || !settings.Widgets.WidgetCPU {
		t.Fatalf("root backend should enable helper and cpu widget")
	}
	if settings.Modules.TmuxTheme || settings.Modules.TermuxAppearance {
		t.Fatalf("shell item should not enable tmux/termux targets")
	}
	if !settings.Modules.FishBootstrap || settings.Modules.StarshipMode != "themed" || !settings.Modules.PeaclockTheme {
		t.Fatalf("shell item should enable fish + themed starship + peaclock targets")
	}
}

func TestApplyInstallPlanLinuxRejectsTermuxItem(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: false}
	plan := setupInstallPlan{Platform: "linux", Backend: "none", ThemeItems: "termux"}
	if err := applyInstallPlan(&settings, env, plan); err == nil {
		t.Fatalf("expected linux + termux item to fail")
	}
}

func TestApplyInstallPlanLinuxNoneKeepsCPUWidgetOn(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: false}
	plan := setupInstallPlan{Platform: "linux", Backend: "none", ThemeItems: "all"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if !settings.Widgets.WidgetCPU {
		t.Fatalf("linux + backend none should keep cpu widget on")
	}
}

func TestApplyInstallPlanTermuxCombinationTargets(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: true}
	plan := setupInstallPlan{Platform: "termux", Backend: "none", ThemeItems: "tmux,starship"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if !settings.Modules.TmuxTheme {
		t.Fatalf("tmux theme should be enabled for tmux selection")
	}
	if settings.Modules.TermuxAppearance {
		t.Fatalf("termux appearance should be off when not selected")
	}
	if settings.Modules.StarshipMode != "themed" || !settings.Modules.PeaclockTheme || !settings.Modules.FishBootstrap {
		t.Fatalf("starship selection should enable themed starship + peaclock + fish defaults")
	}
}

func TestApplyInstallPlanTermuxRishBtopOverrideOff(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: true}
	plan := setupInstallPlan{Platform: "termux", Backend: "rish", ThemeItems: "tmux", Btop: "off"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if settings.Modules.BtopHelper {
		t.Fatalf("btop helper should be off when explicitly disabled")
	}
	if settings.Privileged.Runner != "rish" {
		t.Fatalf("runner should remain rish for termux-rish profile, got %q", settings.Privileged.Runner)
	}
}

func TestApplyInstallPlanTermuxShizukuBtopOverrideOnForcesRishRunner(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: true}
	plan := setupInstallPlan{Platform: "termux", Backend: "shizuku", ThemeItems: "tmux", Btop: "on"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if !settings.Modules.BtopHelper {
		t.Fatalf("btop helper should be enabled when explicitly requested")
	}
	if settings.Privileged.Runner != "rish" {
		t.Fatalf("runner should be forced to rish for shizuku+rish btop flow, got %q", settings.Privileged.Runner)
	}
}

func TestApplyInstallPlanLinuxAllSkipsTermuxAppearance(t *testing.T) {
	settings := defaultTooieSettings()
	env := setupEnv{IsTermux: false}
	plan := setupInstallPlan{Platform: "linux", Backend: "none", ThemeItems: "all"}
	if err := applyInstallPlan(&settings, env, plan); err != nil {
		t.Fatalf("applyInstallPlan() error: %v", err)
	}
	if settings.Modules.TermuxAppearance {
		t.Fatalf("linux all selection should not enable termux appearance")
	}
}
