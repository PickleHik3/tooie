package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

const defaultAppsCacheTTL = 10 * time.Minute

var execCommand = exec.Command

type launchableApp struct {
	PackageName   string `json:"packageName"`
	Label         string `json:"label,omitempty"`
	ActivityName  string `json:"activityName,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	IconCachePath string `json:"iconCachePath,omitempty"`
	SystemApp     bool   `json:"systemApp,omitempty"`
	Source        string `json:"source,omitempty"`
}

type appsCacheFile struct {
	GeneratedAt string          `json:"generatedAt"`
	Apps        []launchableApp `json:"apps"`
}

type appsCommandOutput struct {
	GeneratedAt string          `json:"generatedAt"`
	Cached      bool            `json:"cached"`
	Apps        []launchableApp `json:"apps"`
}

type backendAppMeta struct {
	Label     string
	SystemApp bool
}

func runCLI(args []string) int {
	if len(args) == 0 {
		return runUICommand(nil)
	}
	if len(args) == 1 {
		if handled, code := runSetWallpaperPathCommand(args[0]); handled {
			return code
		}
	}
	if showClock, showCal, ok := parseMiniModeFlags(args); ok {
		return runMiniTUI(showClock, showCal)
	}
	switch strings.TrimSpace(args[0]) {
	case "ui":
		return runUICommand(args[1:])
	case "setup":
		return runSetupCommand(args[1:])
	case "restart", "--restart", "-restart":
		if !canUseLauncherRestart() {
			fmt.Fprintln(os.Stderr, "tooie restart: only available when setup profile is termux-shizuku")
			return 2
		}
		return runRestartCommand(args[1:])
	case "doctor":
		return runDoctorCommand(args[1:])
	case "helper":
		return runHelperCommand(args[1:])
	case "--clock", "clock":
		return runClockCommand(args[1:])
	case "--cal", "cal":
		return runCalCommand(args[1:])
	case "theme":
		return runThemeCommand(args[1:])
	case "help", "--help", "-h":
		printCLIUsage(os.Stdout)
		return 0
	default:
		switch strings.TrimSpace(args[0]) {
		case "apps", "exec", "launch", "icon", "icons":
			fmt.Fprintf(os.Stderr, "tooie: %q was removed in v2 portable mode\n\n", args[0])
			printCLIUsage(os.Stderr)
			return 2
		}
		fmt.Fprintf(os.Stderr, "tooie: unknown command %q\n\n", args[0])
		printCLIUsage(os.Stderr)
		return 2
	}
}

func runSetWallpaperPathCommand(raw string) (bool, int) {
	arg := strings.TrimSpace(raw)
	if arg == "" || strings.HasPrefix(arg, "-") {
		return false, 0
	}
	if !looksLikeImagePath(arg) {
		return false, 0
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		home = homeDir
	}
	wall := canonicalWallpaperCandidate(arg)
	if wall == "" {
		fmt.Fprintf(os.Stderr, "tooie: wallpaper image not found: %s\n", arg)
		return true, 1
	}
	rememberWallpaperPath(home, wall)
	fmt.Printf("Set wallpaper base image: %s\n", wall)
	return true, 0
}

func looksLikeImagePath(arg string) bool {
	v := strings.ToLower(strings.TrimSpace(arg))
	return strings.HasSuffix(v, ".png") ||
		strings.HasSuffix(v, ".jpg") ||
		strings.HasSuffix(v, ".jpeg") ||
		strings.HasSuffix(v, ".webp") ||
		strings.HasSuffix(v, ".bmp") ||
		strings.HasSuffix(v, ".gif")
}

func printCLIUsage(w io.Writer) {
	fmt.Fprintln(w, "Tooie")
	fmt.Fprintln(w, "  Portable dashboard + tmux/theme setup for Termux and Linux.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage")
	fmt.Fprintln(w, "  tooie")
	fmt.Fprintln(w, "      Start the Tooie dashboard UI.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie /path/to/wallpaper.jpg")
	fmt.Fprintln(w, "      Set and persist the base wallpaper image used for theme generation.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie ui")
	fmt.Fprintln(w, "      Start the Tooie dashboard UI.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie setup")
	fmt.Fprintln(w, "      Run interactive guided setup (recommended).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie doctor")
	fmt.Fprintln(w, "      Show environment capability checks and dependency guidance.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie helper btop setup [--runner auto|rish|root|su|tsu|sudo]")
	fmt.Fprintln(w, "      Configure optional btop helper integration.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie helper uninstall [--snapshot latest|<id>]")
	fmt.Fprintln(w, "      Restore files from the last install snapshot.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie theme apply [flags]")
	fmt.Fprintln(w, "      Apply theme using Go engine.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  tooie theme compute [flags]")
	fmt.Fprintln(w, "      Print computed theme payload as JSON.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples")
	fmt.Fprintln(w, "  tooie setup")
	fmt.Fprintln(w, "  tooie doctor")
	fmt.Fprintln(w, "  tooie ~/Pictures/Wallpapers/my-wallpaper.jpg")
	if canUseLauncherRestart() {
		fmt.Fprintln(w, "  tooie restart")
	}
	fmt.Fprintln(w, "  tooie theme apply --theme-source preset --preset-family catppuccin --preset-variant mocha")
	fmt.Fprintln(w, "  tooie helper btop setup --runner auto")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Paths")
	fmt.Fprintln(w, "  Settings:     ~/.config/tooie/settings.json")
	fmt.Fprintln(w, "  Backup root:  ~/.config/tooie/backups/")
	fmt.Fprintln(w, "  Helper stats: ~/.cache/tooie/helper-stats.json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Notes")
	fmt.Fprintln(w, "  Launcher-specific commands were removed in v2 portable mode.")
}

func runUICommand(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "tooie ui: unexpected arguments")
		return 2
	}
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithFPS(60))
	_, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie error: %v\n", err)
		return 1
	}
	return 0
}

func canUseLauncherRestart() bool {
	settings, ok := loadTooieSettings()
	if !ok {
		return false
	}
	return normalizePlatformProfile(settings.Platform.Profile) == "termux-shizuku"
}

func isRestartCLIArgs(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch strings.TrimSpace(args[0]) {
	case "--restart", "-restart":
		return true
	default:
		return false
	}
}

func runAppsCommand(args []string) int {
	fs := flag.NewFlagSet("apps", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	refresh := fs.Bool("refresh", false, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie apps: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie apps: unexpected arguments")
		return 2
	}

	apps, cached, err := getLaunchableApps(defaultAppsCacheTTL, *refresh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie apps: %v\n", err)
		return 1
	}

	out := appsCommandOutput{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Cached:      cached,
		Apps:        apps,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "tooie apps: %v\n", err)
		return 1
	}
	return 0
}

func runClockCommand(args []string) int {
	fs := flag.NewFlagSet("clock", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie --clock: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie --clock: unexpected arguments")
		return 2
	}
	return runMiniTUI(true, false)
}

func runCalCommand(args []string) int {
	fs := flag.NewFlagSet("cal", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie --cal: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie --cal: unexpected arguments")
		return 2
	}
	return runMiniTUI(false, true)
}

func parseMiniModeFlags(args []string) (showClock bool, showCal bool, ok bool) {
	if len(args) == 0 {
		return false, false, false
	}
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		switch arg {
		case "--clock":
			showClock = true
		case "--cal":
			showCal = true
		default:
			return false, false, false
		}
	}
	if !showClock && !showCal {
		return false, false, false
	}
	return showClock, showCal, true
}

func runLaunchCommand(args []string) int {
	fs := flag.NewFlagSet("launch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	refresh := fs.Bool("refresh", false, "")
	ttl := fs.Duration("ttl", defaultAppsCacheTTL, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie launch: %v\n", err)
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "tooie launch: missing package or component")
		return 2
	}

	target := strings.TrimSpace(fs.Arg(0))
	component, err := resolveLaunchTarget(target, *ttl, *refresh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie launch: %v\n", err)
		return 1
	}
	if err := launchComponent(component); err != nil {
		fmt.Fprintf(os.Stderr, "tooie launch: %v\n", err)
		return 1
	}
	return 0
}

func runRestartCommand(args []string) int {
	fs := flag.NewFlagSet("restart", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "tooie restart: %v\n", err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "tooie restart: unexpected arguments")
		return 2
	}
	if err := restartLauncherApp(); err != nil {
		fmt.Fprintf(os.Stderr, "tooie restart: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "tooie restart: launcherctl restart requested")
	return 0
}

func runExecCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "tooie exec: missing command")
		return 2
	}
	command := strings.TrimSpace(strings.Join(args, " "))
	if command == "" {
		fmt.Fprintln(os.Stderr, "tooie exec: missing command")
		return 2
	}
	base, token, ok := readTooieEndpointToken()
	if !ok {
		fmt.Fprintln(os.Stderr, "tooie exec: launcherctl endpoint/token not configured")
		return 1
	}
	out, err := tooieExecCommand(base, token, command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie exec: %v\n", err)
		return 1
	}
	if strings.TrimSpace(out) != "" {
		fmt.Print(out)
		if !strings.HasSuffix(out, "\n") {
			fmt.Println()
		}
	}
	return 0
}

func runIconCommand(args []string) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(os.Stderr, "tooie icon: missing package")
		return 2
	}
	path, err := ensureAppIconCached(strings.TrimSpace(args[0]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie icon: %v\n", err)
		return 1
	}
	fmt.Println(path)
	return 0
}

func runIconsCommand(args []string) int {
	if len(args) == 0 || strings.TrimSpace(args[0]) != "refresh" {
		fmt.Fprintln(os.Stderr, "tooie icons: expected subcommand 'refresh'")
		return 2
	}
	fs := flag.NewFlagSet("icons refresh", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pinnedOnly := fs.Bool("pinned", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tooie icons refresh: %v\n", err)
		return 2
	}

	apps, _, err := getLaunchableApps(defaultAppsCacheTTL, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie icons refresh: %v\n", err)
		return 1
	}
	targets := apps
	if *pinnedOnly {
		targets = filterAppsByPinnedPackages(apps, loadPinnedApps())
	}
	count, refreshErr := refreshAppIcons(targets, true)
	if refreshErr != nil {
		fmt.Fprintf(os.Stderr, "tooie icons refresh: %v\n", refreshErr)
		return 1
	}
	fmt.Printf("refreshed %d icon(s)\n", count)
	return 0
}

func getLaunchableApps(ttl time.Duration, refresh bool) ([]launchableApp, bool, error) {
	if !refresh {
		if apps, ok := readAppsCache(ttl); ok {
			return apps, true, nil
		}
	}
	apps, err := buildLaunchableApps()
	if err != nil {
		if apps, ok := readAppsCache(365 * 24 * time.Hour); ok && len(apps) > 0 {
			return apps, true, nil
		}
		return nil, false, err
	}
	if err := writeAppsCache(apps); err != nil {
		return apps, false, err
	}
	return apps, false, nil
}

func buildLaunchableApps() ([]launchableApp, error) {
	localApps, localErr := queryLauncherAppsLocally()
	backendMeta, backendErr := fetchBackendAppMetadata()

	if len(localApps) == 0 && len(backendMeta) > 0 {
		localApps = resolveBackendAppsLocally(backendMeta)
	}

	if len(localApps) == 0 {
		switch {
		case localErr != nil && backendErr != nil:
			return nil, fmt.Errorf("launcher discovery failed: %v; backend metadata failed: %v", localErr, backendErr)
		case localErr != nil:
			return nil, localErr
		case backendErr != nil:
			return nil, backendErr
		default:
			return nil, errors.New("no launcher apps discovered")
		}
	}

	out := make([]launchableApp, 0, len(localApps))
	for _, app := range localApps {
		meta, ok := backendMeta[app.PackageName]
		if ok {
			if strings.TrimSpace(meta.Label) != "" {
				app.Label = strings.TrimSpace(meta.Label)
			}
			app.SystemApp = meta.SystemApp
			if app.Source == "" {
				app.Source = "local+endpoint"
			}
		}
		if strings.TrimSpace(app.Label) == "" {
			app.Label = app.PackageName
		}
		if iconPath, ok := cachedIconPath(app.PackageName); ok {
			app.IconCachePath = iconPath
		}
		out = append(out, app)
	}

	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(out[i].Label))
		lj := strings.ToLower(strings.TrimSpace(out[j].Label))
		if li == "" {
			li = out[i].PackageName
		}
		if lj == "" {
			lj = out[j].PackageName
		}
		if li == lj {
			return out[i].PackageName < out[j].PackageName
		}
		return li < lj
	})
	return out, nil
}

func resolveLaunchTarget(target string, ttl time.Duration, refresh bool) (string, error) {
	if _, _, ok := splitComponent(target); ok {
		return target, nil
	}
	apps, _, err := getLaunchableApps(ttl, refresh)
	if err == nil {
		for _, app := range apps {
			if app.PackageName == target && strings.TrimSpace(app.ComponentName) != "" {
				return app.ComponentName, nil
			}
		}
	}
	component, found, resolveErr := resolveLauncherComponentForPackage(target)
	if resolveErr != nil {
		return "", resolveErr
	}
	if !found {
		return "", fmt.Errorf("launcher activity not found for %s", target)
	}
	return component, nil
}

func launchComponent(component string) error {
	component = strings.TrimSpace(component)
	if component == "" {
		return errors.New("missing component")
	}
	base, token, ok := readTooieEndpointToken()
	if ok {
		if _, err := tooieExecCommand(base, token, "am start -n "+component+" --user 0"); err == nil {
			return nil
		}
		if _, err := tooieExecCommand(base, token, "am start -n "+component); err == nil {
			return nil
		}
	}
	if out, err := exec.Command("am", "start", "-n", component, "--user", "0").CombinedOutput(); err == nil {
		return nil
	} else if len(bytes.TrimSpace(out)) > 0 {
		_ = out
	}
	out, err := exec.Command("am", "start", "-n", component).CombinedOutput()
	if err != nil {
		return fmt.Errorf("local am launch failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func restartLauncherApp() error {
	out, err := execCommand("launcherctl", "restart").CombinedOutput()
	if err != nil {
		return fmt.Errorf("launcherctl restart failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func fetchBackendAppMetadata() (map[string]backendAppMeta, error) {
	base, token, ok := readTooieEndpointToken()
	if !ok {
		return nil, errors.New("launcherctl endpoint/token not configured")
	}
	res, ok := tooieJSONRequest(base, token, "/v1/apps", 2*time.Second)
	if !ok {
		return nil, errors.New("failed to read /v1/apps")
	}
	rawApps, ok := res["apps"].([]any)
	if !ok {
		return nil, errors.New("invalid /v1/apps response shape")
	}
	out := make(map[string]backendAppMeta, len(rawApps))
	for _, raw := range rawApps {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pkg, ok := anyToString(m["packageName"])
		if !ok || strings.TrimSpace(pkg) == "" {
			continue
		}
		label, _ := anyToString(m["label"])
		systemApp, _ := anyToBool(m["systemApp"])
		out[strings.TrimSpace(pkg)] = backendAppMeta{
			Label:     strings.TrimSpace(label),
			SystemApp: systemApp,
		}
	}
	return out, nil
}

func resolveBackendAppsLocally(meta map[string]backendAppMeta) []launchableApp {
	if len(meta) == 0 {
		return nil
	}
	pkgs := make([]string, 0, len(meta))
	for pkg := range meta {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	out := make([]launchableApp, 0, len(pkgs))
	for _, pkg := range pkgs {
		component, found, err := resolveLauncherComponentForPackage(pkg)
		if err != nil || !found {
			continue
		}
		_, activity, ok := splitComponent(component)
		if !ok {
			continue
		}
		item := launchableApp{
			PackageName:   pkg,
			Label:         meta[pkg].Label,
			ActivityName:  activity,
			ComponentName: component,
			SystemApp:     meta[pkg].SystemApp,
			Source:        "endpoint+resolve",
		}
		if strings.TrimSpace(item.Label) == "" {
			item.Label = pkg
		}
		out = append(out, item)
	}
	return out
}

func queryLauncherAppsLocally() ([]launchableApp, error) {
	intentURI := "intent:#Intent;action=android.intent.action.MAIN;category=android.intent.category.LAUNCHER;end"
	queryUser := strings.TrimSpace(os.Getenv("TERMUX__USER_ID"))
	if queryUser == "" {
		queryUser = "0"
	}

	var candidates []string
	cmds := [][]string{
		{"pm", "query-activities", "--brief", "--components", "--user", queryUser, intentURI},
		{"pm", "query-activities", "--brief", "--components", intentURI},
		{"cmd", "package", "query-activities", "--brief", "--components", "--user", queryUser, "-a", "android.intent.action.MAIN", "-c", "android.intent.category.LAUNCHER"},
	}
	for _, argv := range cmds {
		out, err := exec.Command(argv[0], argv[1:]...).Output()
		if err != nil {
			continue
		}
		lines := parseLauncherComponentLines(string(out))
		if len(lines) > 0 {
			candidates = lines
			break
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("no launcher activities found")
	}

	seen := map[string]bool{}
	out := make([]launchableApp, 0, len(candidates))
	for _, component := range candidates {
		pkg, activity, ok := splitComponent(component)
		if !ok || seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, launchableApp{
			PackageName:   pkg,
			Label:         pkg,
			ActivityName:  activity,
			ComponentName: component,
			Source:        "local",
		})
	}
	return out, nil
}

func parseLauncherComponentLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		component := strings.TrimSpace(line)
		if component == "" {
			continue
		}
		if strings.Contains(component, " ") {
			fields := strings.Fields(component)
			component = fields[len(fields)-1]
		}
		if _, _, ok := splitComponent(component); !ok {
			continue
		}
		out = append(out, component)
	}
	return out
}

func resolveLauncherComponentForPackage(pkg string) (string, bool, error) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return "", false, nil
	}
	args := []string{
		"package", "resolve-activity", "--brief", "--user", "0",
		"-a", "android.intent.action.MAIN",
		"-c", "android.intent.category.LAUNCHER",
		pkg,
	}
	out, err := exec.Command("cmd", args...).Output()
	if err != nil {
		return "", false, err
	}
	component := parseResolvedComponent(string(out))
	if component == "" {
		return "", false, nil
	}
	return component, true, nil
}

func parseResolvedComponent(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" || !strings.Contains(line, "/") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		component := fields[len(fields)-1]
		if _, _, ok := splitComponent(component); ok {
			return component
		}
	}
	return ""
}

func splitComponent(component string) (string, string, bool) {
	component = strings.TrimSpace(component)
	if component == "" {
		return "", "", false
	}
	parts := strings.SplitN(component, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	pkg := strings.TrimSpace(parts[0])
	activity := strings.TrimSpace(parts[1])
	if pkg == "" || activity == "" {
		return "", "", false
	}
	return pkg, activity, true
}

func appsCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("unable to resolve home directory")
	}
	return filepath.Join(home, ".cache", "tooie", "apps.json"), nil
}

func iconCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("unable to resolve home directory")
	}
	return filepath.Join(home, ".cache", "tooie", "icons"), nil
}

func ensureAppIconCached(pkg string) (string, error) {
	return ensureAppIconCachedForApp(launchableApp{PackageName: pkg}, false)
}

func ensureAppIconCachedForApp(app launchableApp, force bool) (string, error) {
	pkg := strings.TrimSpace(app.PackageName)
	if pkg == "" {
		return "", errors.New("missing package")
	}
	if force {
		removeCachedIcons(pkg)
	}
	base, token, ok := readTooieEndpointToken()
	dir, err := iconCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	var endpointErr error
	if ok {
		path, err := cacheIconFromTooieEndpoint(base, token, pkg, dir)
		if err == nil {
			return path, nil
		}
		endpointErr = err
	}
	if !force {
		if path, ok := cachedIconPath(pkg); ok {
			return path, nil
		}
	}
	path, err := cacheInternetIconForApp(app, dir)
	if err == nil {
		return path, nil
	}
	if endpointErr != nil {
		return "", endpointErr
	}
	if !ok {
		return "", errors.New("launcherctl endpoint/token not configured")
	}
	return "", err
}

func cachedIconPath(pkg string) (string, bool) {
	dir, err := iconCacheDir()
	if err != nil {
		return "", false
	}
	base := sanitizePackageName(pkg)
	candidates := []string{
		filepath.Join(dir, base+".png"),
		filepath.Join(dir, base+".webp"),
		filepath.Join(dir, base+".jpg"),
		filepath.Join(dir, base+".jpeg"),
		filepath.Join(dir, base+".img"),
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return path, true
		}
	}
	return "", false
}

func cacheIconFromTooieEndpoint(base, token, pkg, dir string) (string, error) {
	var lastErr error
	for _, path := range iconEndpointCandidates(pkg) {
		cachedPath, err := fetchAndCacheIconURL(strings.TrimRight(base, "/")+path, token, pkg, dir)
		if err == nil {
			return cachedPath, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no icon endpoint candidates configured")
	}
	return "", lastErr
}

func cacheInternetIconForApp(app launchableApp, dir string) (string, error) {
	url, ok := internetIconURLForApp(app)
	if !ok {
		return "", fmt.Errorf("no internet icon mapping for %s", app.PackageName)
	}
	return fetchAndCacheRemoteImage(url, app.PackageName, dir)
}

func fetchAndCacheRemoteImage(url, pkg, dir string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "tooie/0.1")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d for %s", resp.StatusCode, url)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("empty response from %s", url)
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
		return "", fmt.Errorf("decode remote icon: %w", err)
	}
	path, ok := writeImagePayload(pkg, dir, raw, resp.Header.Get("Content-Type"))
	if !ok {
		return "", fmt.Errorf("failed to cache %s", pkg)
	}
	return path, nil
}

func removeCachedIcons(pkg string) {
	dir, err := iconCacheDir()
	if err != nil {
		return
	}
	base := sanitizePackageName(pkg)
	for _, ext := range []string{".png", ".webp", ".jpg", ".jpeg", ".img"} {
		_ = os.Remove(filepath.Join(dir, base+ext))
	}
}

func refreshAppIcons(apps []launchableApp, force bool) (int, error) {
	success := 0
	var lastErr error
	for _, app := range apps {
		if _, err := ensureAppIconCachedForApp(app, force); err == nil {
			success++
		} else {
			lastErr = err
		}
	}
	if success == 0 && lastErr != nil {
		return 0, lastErr
	}
	return success, nil
}

func filterAppsByPinnedPackages(apps []launchableApp, pinned []string) []launchableApp {
	if len(pinned) == 0 {
		return nil
	}
	byPkg := make(map[string]launchableApp, len(apps))
	for _, app := range apps {
		byPkg[app.PackageName] = app
	}
	out := make([]launchableApp, 0, len(pinned))
	for _, pkg := range pinned {
		if app, ok := byPkg[pkg]; ok {
			out = append(out, app)
		}
	}
	return out
}

func internetIconURLForApp(app launchableApp) (string, bool) {
	if iconName, ok := tooieShelfConfigIconName(app); ok {
		return dashboardIconURL(iconName), true
	}
	if iconName, ok := builtinDashboardIconName(app); ok {
		return dashboardIconURL(iconName), true
	}
	return "", false
}

func dashboardIconURL(name string) string {
	return "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/" + strings.TrimSpace(name) + ".png"
}

func tooieShelfConfigIconName(app launchableApp) (string, bool) {
	path := filepath.Join(homeDir, ".config", "tooie-shelf", "config.yaml")
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return "", false
	}
	type shelfApp struct {
		Name    string `yaml:"name"`
		Icon    string `yaml:"icon"`
		Package string `yaml:"package"`
	}
	type shelfConfig struct {
		Apps []shelfApp `yaml:"apps"`
	}
	var cfg shelfConfig
	if err := yamlUnmarshal(raw, &cfg); err != nil {
		return "", false
	}
	pkg := strings.TrimSpace(app.PackageName)
	label := strings.TrimSpace(app.Label)
	for _, item := range cfg.Apps {
		if iconName, ok := dashboardIconNameFromRef(item.Icon); ok {
			if strings.TrimSpace(item.Package) == pkg {
				return iconName, true
			}
			if strings.TrimSpace(item.Name) != "" && strings.EqualFold(strings.TrimSpace(item.Name), label) {
				return iconName, true
			}
		}
	}
	return "", false
}

func dashboardIconNameFromRef(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "dashboard:") {
		name := strings.TrimSpace(strings.TrimPrefix(ref, "dashboard:"))
		return name, name != ""
	}
	return "", false
}

func builtinDashboardIconName(app launchableApp) (string, bool) {
	search := strings.ToLower(strings.TrimSpace(app.PackageName + " " + app.Label))
	candidates := []struct {
		needles []string
		name    string
	}{
		{[]string{"com.termux", "termux"}, "terminal"},
		{[]string{"chrome"}, "google-chrome"},
		{[]string{"chatgpt", "openai"}, "chatgpt"},
		{[]string{"whatsapp"}, "whatsapp"},
		{[]string{"reddit"}, "reddit"},
		{[]string{"telegram"}, "telegram"},
		{[]string{"instagram"}, "instagram"},
		{[]string{"github"}, "github"},
		{[]string{"maps", "waze"}, "google-maps"},
		{[]string{"obtainium"}, "obtainium"},
		{[]string{"immich"}, "immich"},
		{[]string{"tailscale"}, "tailscale"},
		{[]string{"settings"}, "settings"},
		{[]string{"bitwarden"}, "bitwarden"},
		{[]string{"spotify"}, "spotify"},
		{[]string{"youtube music", "yt music"}, "youtube-music"},
		{[]string{"youtube"}, "youtube"},
		{[]string{"files", "solid explorer"}, "files"},
		{[]string{"backdrops", "wallpaper", "lumina walls"}, "wallos"},
	}
	for _, cand := range candidates {
		for _, needle := range cand.needles {
			if strings.Contains(search, needle) {
				return cand.name, true
			}
		}
	}
	return "", false
}

func iconEndpointCandidates(pkg string) []string {
	return []string{
		"/v1/apps/icon/" + pkg,
		"/v1/apps/" + pkg + "/icon",
	}
}

func fetchAndCacheIconURL(url, token, pkg, dir string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 1800 * time.Millisecond}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return "", fmt.Errorf("%s: %s", url, msg)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("%s: empty response body", url)
	}
	if path, ok := writeImagePayload(pkg, dir, body, resp.Header.Get("Content-Type")); ok {
		return path, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("%s: unsupported icon response", url)
	}
	if b64, ok := anyToString(parsed["iconBase64"]); ok && strings.TrimSpace(b64) != "" {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err == nil {
			if path, ok := writeImagePayload(pkg, dir, raw, "image/png"); ok {
				return path, nil
			}
		}
	}
	if dataURL, ok := anyToString(parsed["icon"]); ok && strings.TrimSpace(dataURL) != "" {
		raw, ct, ok := decodeDataURLImage(dataURL)
		if ok {
			if path, ok := writeImagePayload(pkg, dir, raw, ct); ok {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("%s: icon payload not found", url)
}

func writeImagePayload(pkg, dir string, raw []byte, contentType string) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	ext := imageExtension(contentType, raw)
	path := filepath.Join(dir, sanitizePackageName(pkg)+ext)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", false
	}
	return path, true
}

func imageExtension(contentType string, raw []byte) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	}
	if len(raw) >= 8 && bytes.Equal(raw[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return ".png"
	}
	if len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP" {
		return ".webp"
	}
	if len(raw) >= 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff {
		return ".jpg"
	}
	return ".img"
}

func decodeDataURLImage(dataURL string) ([]byte, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(dataURL), ",", 2)
	if len(parts) != 2 {
		return nil, "", false
	}
	meta := parts[0]
	if !strings.Contains(strings.ToLower(meta), ";base64") {
		return nil, "", false
	}
	contentType := "image/png"
	if strings.HasPrefix(strings.ToLower(meta), "data:") {
		contentType = strings.TrimPrefix(meta, "data:")
		contentType = strings.TrimSuffix(contentType, ";base64")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil || len(raw) == 0 {
		return nil, "", false
	}
	return raw, contentType, true
}

func sanitizePackageName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", ".")
	return s
}

func readAppsCache(ttl time.Duration) ([]launchableApp, bool) {
	path, err := appsCachePath()
	if err != nil {
		return nil, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if ttl > 0 && time.Since(info.ModTime()) > ttl {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	var cache appsCacheFile
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, false
	}
	if len(cache.Apps) == 0 {
		return nil, false
	}
	return cache.Apps, true
}

func writeAppsCache(apps []launchableApp) error {
	path, err := appsCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload := appsCacheFile{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Apps:        apps,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func tooieExecCommand(base, token, command string) (string, error) {
	body := map[string]string{"command": command}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(base, "/") + "/v1/exec"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if !truthyJSON(out["ok"]) {
		msg, _ := anyToString(out["message"])
		errCode, _ := anyToString(out["error"])
		if strings.TrimSpace(msg) == "" {
			msg = "exec failed"
		}
		if strings.TrimSpace(errCode) != "" {
			msg = errCode + ": " + msg
		}
		return "", errors.New(msg)
	}
	result, _ := anyToString(out["output"])
	return result, nil
}

func truthyJSON(v any) bool {
	b, ok := anyToBool(v)
	return ok && b
}

func anyToString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case fmt.Stringer:
		return x.String(), true
	case json.Number:
		return x.String(), true
	default:
		return "", false
	}
}

func anyToBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "yes", "on":
			return true, true
		case "0", "false", "no", "off":
			return false, true
		default:
			return false, false
		}
	case float64:
		return x != 0, true
	case int:
		return x != 0, true
	default:
		return false, false
	}
}

func yamlUnmarshal(data []byte, out any) error {
	return yaml.Unmarshal(data, out)
}
