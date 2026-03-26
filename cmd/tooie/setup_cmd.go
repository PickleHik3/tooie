package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type setupEnv struct {
	IsTermux bool
	HasTmux  bool
	OS       string
}

func runSetupCommand(args []string) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	nonInteractive := fs.Bool("non-interactive", false, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie setup: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie setup: unexpected arguments")
		return 2
	}
	if !*nonInteractive {
		info, err := os.Stdin.Stat()
		if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
			fmt.Fprintln(os.Stderr, "tooie setup: interactive mode requires a TTY; rerun in a terminal or use --non-interactive")
			return 2
		}
		if !commandExists("gum") {
			fmt.Fprintln(os.Stderr, "tooie setup: gum is required for interactive setup. Install gum or use --non-interactive")
			return 2
		}
	}

	// TODO(release): remove self-rebuild from setup flow once packaging is finalized.
	if err := buildAndReplaceInstalledBinary(); err != nil {
		fmt.Fprintf(os.Stderr, "tooie setup: failed to rebuild/install binary: %v\n", err)
		return 1
	}

	env := detectSetupEnv()
	settings := migrateLegacyIntoTooieSettings()
	if !env.IsTermux {
		settings.Modules.TermuxAppearance = false
	}

	if !*nonInteractive {
		next, err := runSetupWizard(settings, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tooie setup: %v\n", err)
			return 1
		}
		settings = next
	}

	if err := saveTooieSettings(settings); err != nil {
		fmt.Fprintf(os.Stderr, "tooie setup: failed to write settings: %v\n", err)
		return 1
	}
	if err := savePersistedShellSettings(settings.Widgets); err != nil {
		fmt.Fprintf(os.Stderr, "tooie setup: failed to persist widget settings: %v\n", err)
		return 1
	}
	if err := applySetupSelection(settings, env); err != nil {
		fmt.Fprintf(os.Stderr, "tooie setup: apply failed: %v\n", err)
		return 1
	}

	fmt.Printf("Setup complete. Settings written to %s\n", tooieSettingsPath())
	return 0
}

func detectSetupEnv() setupEnv {
	home := strings.TrimSpace(os.Getenv("HOME"))
	return setupEnv{
		IsTermux: strings.Contains(home, "/data/data/com.termux") || os.Getenv("PREFIX") == "/data/data/com.termux/files/usr",
		HasTmux:  commandExists("tmux"),
		OS:       runtime.GOOS,
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runSetupWizard(cur tooieSettings, env setupEnv) (tooieSettings, error) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render("Tooie Setup")
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Gum-powered guided setup")
	fmt.Println(title)
	fmt.Println(sub)
	fmt.Println()

	var err error
	cur.Tmux.Mode, err = gumChoose("Tmux mode", []string{"full", "status-only"}, cur.Tmux.Mode)
	if err != nil {
		return cur, err
	}
	cur.Tmux.StatusPosition, err = gumChoose("Status position", []string{"top", "bottom"}, cur.Tmux.StatusPosition)
	if err != nil {
		return cur, err
	}
	cur.Tmux.StatusLayout, err = gumChoose("Status layout", []string{"two-line", "single-line"}, cur.Tmux.StatusLayout)
	if err != nil {
		return cur, err
	}
	if cur.Tmux.StatusLayout == "two-line" {
		cur.Tmux.StatusSeparator, err = gumChoose("Separator line", []string{"on", "off"}, cur.Tmux.StatusSeparator)
		if err != nil {
			return cur, err
		}
	} else {
		cur.Tmux.StatusSeparator = "off"
	}

	if env.IsTermux {
		cur.Modules.TermuxAppearance, err = gumBool("Install Termux appearance files (.termux/*)?", cur.Modules.TermuxAppearance)
		if err != nil {
			return cur, err
		}
	} else {
		cur.Modules.TermuxAppearance = false
	}
	cur.Modules.ShellTheme, err = gumBool("Install fish + starship theme files?", cur.Modules.ShellTheme)
	if err != nil {
		return cur, err
	}
	cur.Modules.PeaclockTheme, err = gumBool("Install peaclock theme file?", cur.Modules.PeaclockTheme)
	if err != nil {
		return cur, err
	}
	cur.Modules.BtopHelper, err = gumBool("Configure optional btop helper module?", cur.Modules.BtopHelper)
	if err != nil {
		return cur, err
	}
	if cur.Modules.BtopHelper {
		cur.Privileged.Runner, err = gumChoose("Privileged runner", []string{"auto", "rish", "su", "tsu"}, cur.Privileged.Runner)
		if err != nil {
			return cur, err
		}
	} else {
		cur.Privileged.Runner = "auto"
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Render("Summary"))
	fmt.Printf("  tmux mode: %s\n", cur.Tmux.Mode)
	fmt.Printf("  status: %s, %s, separator=%s\n", cur.Tmux.StatusPosition, cur.Tmux.StatusLayout, cur.Tmux.StatusSeparator)
	fmt.Printf("  modules: termux=%s shell=%s peaclock=%s btop=%s\n",
		onOffFlag(cur.Modules.TermuxAppearance),
		onOffFlag(cur.Modules.ShellTheme),
		onOffFlag(cur.Modules.PeaclockTheme),
		onOffFlag(cur.Modules.BtopHelper),
	)
	ok, err := gumConfirm("Apply this setup now?")
	if err != nil {
		return cur, err
	}
	if !ok {
		return cur, fmt.Errorf("setup cancelled")
	}
	normalizeTooieSettings(&cur)
	return cur, nil
}

func gumChoose(header string, options []string, current string) (string, error) {
	reordered := reorderWithDefault(options, current)
	args := []string{"choose", "--header", header}
	args = append(args, reordered...)
	cmd := exec.Command("gum", args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	pick := strings.TrimSpace(string(out))
	if pick == "" {
		if len(reordered) > 0 {
			return reordered[0], nil
		}
		return "", fmt.Errorf("no choice selected")
	}
	return pick, nil
}

func gumBool(prompt string, current bool) (bool, error) {
	defaultValue := "no"
	if current {
		defaultValue = "yes"
	}
	pick, err := gumChoose(prompt, []string{"yes", "no"}, defaultValue)
	if err != nil {
		return current, err
	}
	return pick == "yes", nil
}

func gumConfirm(prompt string) (bool, error) {
	cmd := exec.Command("gum", "confirm", prompt)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func reorderWithDefault(options []string, current string) []string {
	if len(options) == 0 {
		return nil
	}
	cur := strings.ToLower(strings.TrimSpace(current))
	if cur == "" {
		return options
	}
	out := make([]string, 0, len(options))
	for _, opt := range options {
		if strings.ToLower(strings.TrimSpace(opt)) == cur {
			out = append(out, opt)
			break
		}
	}
	for _, opt := range options {
		if len(out) > 0 && opt == out[0] {
			continue
		}
		out = append(out, opt)
	}
	return out
}

func resolveRepoAssetPath(rel string) (string, error) {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), rel))
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		candidates = append(candidates, filepath.Join(wd, rel))
	}
	candidates = append(candidates, filepath.Join(homeDir, "files", "tooie", rel))
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("repo asset not found: %s", rel)
}

func copyFile(src, dst string, perm os.FileMode) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, perm)
}

func copyTree(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func buildAndReplaceInstalledBinary() error {
	if !commandExists("go") {
		return fmt.Errorf("go is required")
	}
	goModPath, err := resolveRepoAssetPath("go.mod")
	if err != nil {
		return err
	}
	repoRoot := filepath.Dir(goModPath)
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return fmt.Errorf("unable to resolve home for binary install")
	}
	binPath := filepath.Join(home, ".local", "bin", "tooie")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return err
	}
	tmpBin := binPath + ".tmp"
	cmd := exec.Command("go", "build", "-o", tmpBin, "./cmd/tooie")
	cmd.Dir = repoRoot
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := os.Chmod(tmpBin, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpBin, binPath)
}

func installSupportScriptsFromRepo() error {
	if err := os.MkdirAll(tooieConfigDir, 0o755); err != nil {
		return err
	}
	targets := []struct {
		rel string
		dst string
	}{
		{rel: filepath.Join("scripts", "apply-material.sh"), dst: installedApplyScriptPath()},
		{rel: filepath.Join("scripts", "restore-material.sh"), dst: installedRestoreScriptPath()},
		{rel: filepath.Join("scripts", "list-material-backups.sh"), dst: filepath.Join(tooieConfigDir, "list-material-backups.sh")},
		{rel: filepath.Join("scripts", "reset-bootstrap-defaults.sh"), dst: installedResetScriptPath()},
		{rel: filepath.Join("scripts", "setup-btop-helper.sh"), dst: installedBtopSetupScriptPath()},
	}
	for _, item := range targets {
		src, err := resolveRepoAssetPath(item.rel)
		if err != nil {
			return err
		}
		if err := copyFile(src, item.dst, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func applySetupSelection(settings tooieSettings, env setupEnv) error {
	home := homeDir
	if err := installSupportScriptsFromRepo(); err != nil {
		return err
	}

	tmuxDirSrc, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "tmux"))
	if err != nil {
		return err
	}
	if err := copyTree(tmuxDirSrc, filepath.Join(home, ".config", "tmux")); err != nil {
		return err
	}

	if settings.Tmux.Mode == "full" {
		tmuxConfSrc, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".tmux.conf"))
		if err != nil {
			return err
		}
		if err := copyFile(tmuxConfSrc, filepath.Join(home, ".tmux.conf"), 0o644); err != nil {
			return err
		}
	} else {
		if err := ensureFileWithDirs(filepath.Join(home, ".tmux.conf")); err != nil {
			return err
		}
	}

	if settings.Modules.TermuxAppearance && env.IsTermux {
		for _, name := range []string{"termux.properties", "colors.properties", "font.ttf", "font-italic.ttf"} {
			src, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".termux", name))
			if err != nil {
				return err
			}
			if err := copyFile(src, filepath.Join(home, ".termux", name), 0o644); err != nil {
				return err
			}
		}
	}

	if settings.Modules.ShellTheme {
		starshipSrc, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "starship.toml"))
		if err != nil {
			return err
		}
		if err := copyFile(starshipSrc, filepath.Join(home, ".config", "starship.toml"), 0o644); err != nil {
			return err
		}
		fishSrc, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "fish", "config.fish"))
		if err != nil {
			return err
		}
		if err := copyFile(fishSrc, filepath.Join(home, ".config", "fish", "config.fish"), 0o644); err != nil {
			return err
		}
	}

	if settings.Modules.PeaclockTheme {
		peaclockSrc, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "peaclock", "config"))
		if err != nil {
			return err
		}
		if err := copyFile(peaclockSrc, filepath.Join(home, ".config", "peaclock", "config"), 0o644); err != nil {
			return err
		}
	}

	if settings.Modules.BtopHelper {
		src, err := resolveRepoAssetPath(filepath.Join("scripts", "setup-btop-helper.sh"))
		if err != nil {
			return err
		}
		if err := copyFile(src, currentBtopSetupScriptPath(), 0o755); err != nil {
			return err
		}
	}

	themeArgs := []string{
		"--theme-source", "preset",
		"--preset-family", "catppuccin",
		"--preset-variant", "mocha",
		"--status-position", settings.Tmux.StatusPosition,
		"--status-layout", settings.Tmux.StatusLayout,
		"--status-separator", settings.Tmux.StatusSeparator,
		"--widget-battery", onOffFlag(settings.Widgets.WidgetBattery),
		"--widget-cpu", onOffFlag(settings.Widgets.WidgetCPU),
		"--widget-ram", onOffFlag(settings.Widgets.WidgetRAM),
		"--widget-weather", onOffFlag(settings.Widgets.WidgetWeather),
	}
	if runThemeApplyCommand(themeArgs) != 0 {
		return fmt.Errorf("theme apply failed")
	}
	if env.HasTmux {
		_ = exec.Command("tmux", "source-file", filepath.Join(home, ".tmux.conf")).Run()
		_ = exec.Command("tmux", "refresh-client", "-S").Run()
	}

	required := []string{
		filepath.Join(home, ".config", "tmux", "run-system-widget"),
		filepath.Join(home, ".config", "tmux", "system-widgets"),
		filepath.Join(home, ".config", "tmux", "widget-left"),
		installedApplyScriptPath(),
		installedRestoreScriptPath(),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("missing required installed file: %s", p)
		}
	}

	return nil
}
