package main

import (
	"encoding/json"
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

type labeledChoice struct {
	Label string
	Value string
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
	if strings.TrimSpace(settings.Platform.Profile) == "" {
		settings.Platform.Profile = defaultPlatformProfile(env)
	}
	applyProfileDefaults(&settings, settings.Platform.Profile, env)
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
	normalizeTooieSettings(&settings)

	snapshotID, err := captureInstallSnapshot(installManagedPaths(homeDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie setup: failed to create install snapshot: %v\n", err)
		return 1
	}
	fmt.Printf("Created install snapshot: %s\n", snapshotID)

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

	clearSetupScreen()
	printSetupNextSteps(settings)
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
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Simple guided setup (choose and continue)")
	fmt.Println(title)
	fmt.Println(sub)
	fmt.Println()

	if strings.TrimSpace(cur.Platform.Profile) == "" {
		cur.Platform.Profile = defaultPlatformProfile(env)
	}
	applyProfileDefaults(&cur, normalizePlatformProfile(cur.Platform.Profile), env)

	step := 0
	for {
		switch step {
		case 0:
			v, back, err := gumChooseLabeled("Step 1/10: Choose your environment", platformChoices(env), cur.Platform.Profile, false)
			if err != nil {
				return cur, err
			}
			if back {
				continue
			}
			applyProfileDefaults(&cur, v, env)
			step++
		case 1:
			v, back, err := gumChooseSimple("Step 2/10: Tmux mode", []string{"full", "status-only"}, cur.Tmux.Mode, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Tmux.Mode = v
			step++
		case 2:
			v, back, err := gumChooseSimple("Step 3/10: Status position", []string{"top", "bottom"}, cur.Tmux.StatusPosition, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Tmux.StatusPosition = v
			step++
		case 3:
			v, back, err := gumChooseSimple("Step 4/10: Status layout", []string{"two-line", "single-line"}, cur.Tmux.StatusLayout, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Tmux.StatusLayout = v
			if v == "single-line" {
				cur.Tmux.StatusSeparator = "off"
				step = 5
			} else {
				step++
			}
		case 4:
			v, back, err := gumChooseSimple("Step 5/10: Separator line", []string{"on", "off"}, cur.Tmux.StatusSeparator, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Tmux.StatusSeparator = v
			step++
		case 5:
			if env.IsTermux {
				v, back, err := gumBoolWithBack("Step 6/10: Install Termux appearance files (.termux/*)?", cur.Modules.TermuxAppearance, true)
				if err != nil {
					return cur, err
				}
				if back {
					if cur.Tmux.StatusLayout == "two-line" {
						step = 4
					} else {
						step = 3
					}
					continue
				}
				cur.Modules.TermuxAppearance = v
			} else {
				cur.Modules.TermuxAppearance = false
			}
			step++
		case 6:
			v, back, err := gumBoolWithBack("Step 7/10: Install fish + starship theme files?", cur.Modules.ShellTheme, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Modules.ShellTheme = v
			step++
		case 7:
			v, back, err := gumBoolWithBack("Step 8/10: Install peaclock theme file?", cur.Modules.PeaclockTheme, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Modules.PeaclockTheme = v
			step++
		case 8:
			v, back, err := gumBoolWithBack("Step 9/10: Configure btop helper module?", cur.Modules.BtopHelper, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Modules.BtopHelper = v
			if !cur.Modules.BtopHelper {
				cur.Privileged.Runner = "auto"
				step = 10
			} else {
				step++
			}
		case 9:
			v, back, err := gumChooseSimple("Step 10/10: Privileged runner", []string{"auto", "rish", "su", "tsu"}, cur.Privileged.Runner, true)
			if err != nil {
				return cur, err
			}
			if back {
				step--
				continue
			}
			cur.Privileged.Runner = v
			step++
		default:
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render("Summary"))
			fmt.Printf("  profile: %s\n", cur.Platform.Profile)
			fmt.Printf("  tmux mode: %s\n", cur.Tmux.Mode)
			fmt.Printf("  status: %s, %s, separator=%s\n", cur.Tmux.StatusPosition, cur.Tmux.StatusLayout, cur.Tmux.StatusSeparator)
			fmt.Printf("  modules: termux=%s shell=%s peaclock=%s btop=%s\n",
				onOffFlag(cur.Modules.TermuxAppearance),
				onOffFlag(cur.Modules.ShellTheme),
				onOffFlag(cur.Modules.PeaclockTheme),
				onOffFlag(cur.Modules.BtopHelper),
			)
			fmt.Printf("  runner: %s\n", cur.Privileged.Runner)
			choice, err := gumChoose("Apply now?", []string{"apply", "back", "cancel"}, "apply")
			if err != nil {
				return cur, err
			}
			switch choice {
			case "apply":
				normalizeTooieSettings(&cur)
				return cur, nil
			case "back":
				if cur.Modules.BtopHelper {
					step = 9
				} else {
					step = 8
				}
			default:
				return cur, fmt.Errorf("setup cancelled")
			}
		}
	}
}

func defaultPlatformProfile(env setupEnv) string {
	if env.IsTermux {
		return "termux"
	}
	return "linux"
}

func platformChoices(env setupEnv) []labeledChoice {
	if !env.IsTermux {
		return []labeledChoice{{Label: "Linux", Value: "linux"}}
	}
	return []labeledChoice{
		{Label: "Termux", Value: "termux"},
		{Label: "Termux + root", Value: "termux-root"},
		{Label: "Termux + Shizuku", Value: "termux-shizuku"},
		{Label: "Termux + rish", Value: "termux-rish"},
		{Label: "Linux", Value: "linux"},
	}
}

func applyProfileDefaults(cur *tooieSettings, profile string, env setupEnv) {
	profile = normalizePlatformProfile(profile)
	cur.Platform.Profile = profile
	switch profile {
	case "termux-root":
		cur.Modules.TermuxAppearance = env.IsTermux
		cur.Modules.BtopHelper = true
		cur.Privileged.Runner = "su"
	case "termux-shizuku":
		cur.Modules.TermuxAppearance = env.IsTermux
		cur.Modules.BtopHelper = true
		cur.Privileged.Runner = "auto"
	case "termux-rish":
		cur.Modules.TermuxAppearance = env.IsTermux
		cur.Modules.BtopHelper = true
		cur.Privileged.Runner = "rish"
	case "linux":
		cur.Modules.TermuxAppearance = false
		cur.Modules.BtopHelper = false
		cur.Privileged.Runner = "auto"
	default:
		cur.Modules.TermuxAppearance = env.IsTermux
		cur.Modules.BtopHelper = false
		cur.Privileged.Runner = "auto"
	}
}

func gumChooseSimple(header string, options []string, current string, allowBack bool) (string, bool, error) {
	choices := make([]labeledChoice, 0, len(options))
	for _, opt := range options {
		choices = append(choices, labeledChoice{Label: opt, Value: opt})
	}
	return gumChooseLabeled(header, choices, current, allowBack)
}

func gumChooseLabeled(header string, options []labeledChoice, current string, allowBack bool) (string, bool, error) {
	reordered := reorderLabeledWithDefault(options, current)
	labels := make([]string, 0, len(reordered)+1)
	valueByLabel := map[string]string{}
	for _, item := range reordered {
		labels = append(labels, item.Label)
		valueByLabel[item.Label] = item.Value
	}
	if allowBack {
		labels = append(labels, "Back")
	}
	pick, err := gumChoose(header, labels, labels[0])
	if err != nil {
		return "", false, err
	}
	if allowBack && pick == "Back" {
		return "", true, nil
	}
	v, ok := valueByLabel[pick]
	if !ok {
		return "", false, fmt.Errorf("invalid choice: %s", pick)
	}
	return v, false, nil
}

func gumBoolWithBack(prompt string, current bool, allowBack bool) (bool, bool, error) {
	defaultValue := "no"
	if current {
		defaultValue = "yes"
	}
	v, back, err := gumChooseSimple(prompt, []string{"yes", "no"}, defaultValue, allowBack)
	if err != nil {
		return current, false, err
	}
	if back {
		return current, true, nil
	}
	return v == "yes", false, nil
}

func reorderLabeledWithDefault(options []labeledChoice, current string) []labeledChoice {
	if len(options) == 0 {
		return nil
	}
	cur := strings.ToLower(strings.TrimSpace(current))
	if cur == "" {
		return options
	}
	out := make([]labeledChoice, 0, len(options))
	for _, opt := range options {
		if strings.ToLower(strings.TrimSpace(opt.Value)) == cur {
			out = append(out, opt)
			break
		}
	}
	for _, opt := range options {
		if len(out) > 0 && opt.Value == out[0].Value {
			continue
		}
		out = append(out, opt)
	}
	return out
}

func gumChoose(header string, options []string, current string) (string, error) {
	reordered := reorderWithDefault(options, current)
	args := []string{
		"choose",
		"--header", header,
		"--cursor", "▶ ",
		"--cursor-prefix", " ",
		"--selected-prefix", " ",
		"--unselected-prefix", " ",
		"--header.foreground", "99",
		"--cursor.foreground", "255",
		"--cursor.background", "99",
		"--selected.foreground", "255",
		"--selected.background", "141",
		"--item.foreground", "183",
	}
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

func clearSetupScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func printSetupNextSteps(settings tooieSettings) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render("Setup complete")
	pathLine := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Settings: " + tooieSettingsPath())
	cmdStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("99")).Padding(0, 1)
	fmt.Println(title)
	fmt.Println(pathLine)
	fmt.Println()
	fmt.Println("Next:")
	fmt.Printf("  %s  open the Tooie TUI\n", cmdStyle.Render("tooie"))
	if normalizePlatformProfile(settings.Platform.Profile) == "termux-shizuku" {
		fmt.Printf("  %s  restart launcher (Shizuku profile)\n", cmdStyle.Render("tooie restart"))
	}
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
	profileEnvSrc, err := resolveRepoAssetPath(filepath.Join("assets", "defaults", ".config", "tmux", "profiles", normalizePlatformProfile(settings.Platform.Profile)+".env"))
	if err != nil {
		return err
	}
	if err := copyFile(profileEnvSrc, filepath.Join(home, ".config", "tmux", "profile.env"), 0o644); err != nil {
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
		runner := normalizeRunner(settings.Privileged.Runner)
		if err := saveHelperConfig(helperConfig{Runner: runner}); err != nil {
			return fmt.Errorf("failed to write helper config: %w", err)
		}
		if err := seedHelperStats(); err != nil {
			return fmt.Errorf("failed to seed helper stats: %w", err)
		}
		src, err := resolveRepoAssetPath(filepath.Join("scripts", "setup-btop-helper.sh"))
		if err != nil {
			return err
		}
		if err := copyFile(src, currentBtopSetupScriptPath(), 0o755); err != nil {
			return err
		}
		cmd := exec.Command(currentBtopSetupScriptPath(), "--runner", runner)
		if out, err := cmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			fmt.Fprintf(os.Stderr, "tooie setup: warning: btop helper script failed: %s\n", msg)
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
		filepath.Join(home, ".config", "tmux", "profile.env"),
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

func seedHelperStats() error {
	path := filepath.Join(homeDir, ".cache", "tooie", "helper-stats.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload := map[string]any{
		"cpuPercent":    0,
		"memUsedBytes":  0,
		"memTotalBytes": 0,
		"battery": map[string]any{
			"levelPercent": 0,
			"charging":     false,
		},
		"source":    "btop-helper",
		"updatedAt": "",
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

func installManagedPaths(home string) []string {
	return []string{
		filepath.Join(home, ".local", "bin", "tooie"),
		filepath.Join(home, ".config", "tooie"),
		filepath.Join(home, ".config", "tmux"),
		filepath.Join(home, ".tmux.conf"),
		filepath.Join(home, ".config", "starship.toml"),
		filepath.Join(home, ".config", "fish", "config.fish"),
		filepath.Join(home, ".config", "peaclock", "config"),
		filepath.Join(home, ".termux", "termux.properties"),
		filepath.Join(home, ".termux", "colors.properties"),
		filepath.Join(home, ".termux", "font.ttf"),
		filepath.Join(home, ".termux", "font-italic.ttf"),
	}
}
