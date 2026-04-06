package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	internaltheme "github.com/PickleHik3/tooie/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	homeDir        = resolveHomeDir()
	tooieConfigDir = filepath.Join(homeDir, ".config", "tooie")
	backupRoot     = filepath.Join(tooieConfigDir, "backups")
	defaultWall    = filepath.Join(homeDir, ".termux", "background", "background_portrait.jpeg")
)

const (
	defaultMode        = "dark"
	defaultPalette     = "default"
	defaultStatusTheme = "default"
	defaultProfile     = "auto"
	defaultPaletteType = "tonal-spot"
	defaultSource      = "wallpaper"
	defaultStarship    = "gruvbox"
	themeCacheSchema   = "2026-03-20-v2"
	pageHome           = 0
	pageTheme          = 1
	totalPages         = 2
)

var modePresets = []string{"dark", "light"}
var statusThemePresets = []string{"default", "rounded", "rectangle"}
var starshipPromptPresets = []string{"gruvbox", "pure", "jetpack"}
var paletteTypePresets = []string{"tonal-spot", "expressive", "fidelity", "content", "vibrant", "neutral", "rainbow", "fruit-salad"}
var profilePresets = []string{
	"auto",
	"source-0",
	"source-1",
	"source-2",
	"source-3",
	"source-4",
	"prefer-saturation",
	"prefer-lightness",
	"prefer-darkness",
	"prefer-less-saturation",
	"prefer-value",
	"prefer-closest-fallback",
}
var themeSources = []string{"wallpaper", "preset"}
var presetFamilyOrder = []string{"catppuccin", "rose-pine", "tokyo-night", "synthwave-84", "dracula", "gruvbox", "nord"}
var presetVariantsByFamily = map[string][]string{
	"catppuccin":   {"latte", "frappe", "macchiato", "mocha"},
	"rose-pine":    {"main", "moon", "dawn"},
	"tokyo-night":  {"storm", "moon", "night", "day"},
	"synthwave-84": {"default"},
	"dracula":      {"default"},
	"gruvbox":      {"dark", "light"},
	"nord":         {"default"},
}

type settingItem struct {
	Label  string
	Target string
}

type settingChoice struct {
	Value string
	Label string
}

type tickMsg time.Time
type metricsTickMsg time.Time
type applyTickMsg time.Time

type metricsMsg struct {
	cpuPercent  float64
	ramUsedGB   float64
	ramTotalGB  float64
	storUsedGB  float64
	storTotalGB float64
	storPercent float64
	uptimeSecs  uint64
	uptimeText  string
	err         error
}

type applyDoneMsg struct {
	label       string
	err         error
	out         string
	backupID    string
	cacheKey    string
	reused      bool
	previewOnly bool
}

type clockFontDef struct {
	Name string
	Dir  string
}

type fontClockLayout struct {
	gapYMin     int
	innerPadY   int
	topNudgeY   int
	bottomNudge int
}

type persistedClockSettings struct {
	Font    string `json:"font,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	CalFont string `json:"cal_font,omitempty"`
}

type persistedShellSettings struct {
	WidgetBattery bool `json:"widget_battery"`
	WidgetCPU     bool `json:"widget_cpu"`
	WidgetRAM     bool `json:"widget_ram"`
	WidgetWeather bool `json:"widget_weather"`
}

type backup struct {
	ID   string
	Meta map[string]string
}

type matugenJSON struct {
	Colors map[string]struct {
		Default struct {
			Color string `json:"color"`
		} `json:"default"`
	} `json:"colors"`
}

type applyProgressState struct {
	Label    string  `json:"label"`
	Progress float64 `json:"progress"`
}

type model struct {
	width             int
	height            int
	page              int
	now               time.Time
	cpuPercent        float64
	cpuFiltered       float64
	ramUsedGB         float64
	ramTotalGB        float64
	ramFiltered       float64
	storUsedGB        float64
	storTotalGB       float64
	storPercent       float64
	storFiltered      float64
	uptimeSeconds     uint64
	uptimeText        string
	cpuViz            float64
	ramViz            float64
	storViz           float64
	cpuVel            float64
	ramVel            float64
	storVel           float64
	lastTick          time.Time
	barSpring         harmonica.Spring
	clockPhase        float64
	introUntil        time.Time
	introMode         int
	introSeed         float64
	clockFontDefs     []clockFontDef
	clockFontIdx      int
	clockGlyphs       map[rune][]string
	clockPatterns     []string
	patternIndex      int
	clockLoc          *time.Location
	noticeText        string
	noticeKind        string
	noticeStart       time.Time
	noticeUntil       time.Time
	backups           []backup
	backupIndex       int
	settingIndex      int
	customIndex       int
	mode              string
	palette           string
	paletteType       string
	statusTheme       string
	starshipPrompt    string
	profile           string
	themeSource       string
	presetFamily      string
	presetVariant     string
	textColor         string
	cursorColor       string
	ansiRed           string
	ansiGreen         string
	ansiYellow        string
	ansiBlue          string
	ansiMagenta       string
	ansiCyan          string
	lastStatus        string
	pickerTarget      string
	pickerIndex       int
	customizing       bool
	showHints         bool
	showBackups       bool
	showApplyConfirm  bool
	applyConfirmIdx   int
	extractSwatches   map[string]string
	extractSwatchKey  string
	extractLoading    bool
	extractError      string
	selectedHexes     map[string]string
	metricsErr        error
	applying          bool
	applyProgress     float64
	applyVel          float64
	applyTarget       float64
	applyLabel        string
	applyCacheKey     string
	lastAppliedTheme  string
	previewCacheKey   string
	previewBackupID   string
	metricsPaused     bool
	systemInfo        systemInfo
	widgetBattery     bool
	widgetCPU         bool
	widgetRAM         bool
	widgetWeather     bool
	clockOnly         bool
	miniShowClock     bool
	miniShowCal       bool
	calFontDefs       []clockFontDef
	calFontIdx        int
	calGlyphs         map[int][]string
	switchAnimTarget  string
	switchAnimFrom    bool
	switchAnimTo      bool
	switchAnimProg    float64
	settingMenuTarget string
	settingMenuIndex  int
}

func initialModel() model {
	bs := loadBackups()
	now := time.Now()
	clockLoc := detectClockLocation()
	if clockLoc != nil {
		now = now.In(clockLoc)
	}
	fontDefs := discoverClockFonts()
	savedClock := loadClockSettings()
	clockFontIdx := 0
	if len(fontDefs) == 0 {
		clockFontIdx = -1
	} else {
		if strings.TrimSpace(savedClock.Font) != "" {
			for i, fd := range fontDefs {
				if strings.EqualFold(strings.TrimSpace(fd.Name), strings.TrimSpace(savedClock.Font)) {
					clockFontIdx = i
					break
				}
			}
		} else {
			for i, fd := range fontDefs {
				name := strings.ToLower(strings.TrimSpace(fd.Name))
				if strings.Contains(name, "shadow") {
					clockFontIdx = i
					break
				}
			}
		}
	}
	m := model{
		page:            pageHome,
		backups:         bs,
		now:             now,
		lastTick:        now,
		uptimeText:      "--",
		mode:            defaultMode,
		palette:         defaultPalette,
		paletteType:     defaultPaletteType,
		statusTheme:     defaultStatusTheme,
		starshipPrompt:  defaultStarship,
		profile:         defaultProfile,
		themeSource:     defaultSource,
		presetFamily:    "tokyo-night",
		presetVariant:   "night",
		lastStatus:      "Ready",
		textColor:       "",
		cursorColor:     "",
		ansiRed:         "",
		ansiGreen:       "",
		ansiYellow:      "",
		ansiBlue:        "",
		ansiMagenta:     "",
		ansiCyan:        "",
		barSpring:       harmonica.NewSpring(harmonica.FPS(20), 4.6, 0.90),
		clockFontDefs:   fontDefs,
		clockFontIdx:    clockFontIdx,
		clockPatterns:   []string{"wave", "stripes", "pulse", "solid", "outline", "sweep", "neon", "calm", "shimmer"},
		clockLoc:        clockLoc,
		widgetBattery:   true,
		widgetCPU:       true,
		widgetRAM:       true,
		widgetWeather:   true,
		extractSwatches: map[string]string{},
	}
	m.clockGlyphs = loadClockGlyphSet(m.clockFontDefs, m.clockFontIdx)
	m.loadStarshipSettings()
	if strings.TrimSpace(savedClock.Pattern) != "" {
		for i, p := range m.clockPatterns {
			if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(savedClock.Pattern)) {
				m.patternIndex = i
				break
			}
		}
	}
	m.loadThemeStateFromBackups()
	m.loadShellSettings()
	m.loadPreviewColors()
	if m.themeSource == "wallpaper" {
		m.extractLoading = true
	}
	m.lastAppliedTheme = m.applyCacheSignature()
	m.startHomeIntro()
	return m
}

func initialClockModel() model {
	m := initialMiniModel(true, false)
	return m
}

func initialCalModel() model {
	return initialMiniModel(false, true)
}

func initialClockCalModel() model {
	return initialMiniModel(true, true)
}

func initialMiniModel(showClock, showCal bool) model {
	now := time.Now()
	clockLoc := detectClockLocation()
	if clockLoc != nil {
		now = now.In(clockLoc)
	}
	fontDefs := discoverClockFonts()
	savedClock := loadClockSettings()
	clockFontIdx := 0
	if len(fontDefs) == 0 {
		clockFontIdx = -1
	} else if strings.TrimSpace(savedClock.Font) != "" {
		for i, fd := range fontDefs {
			if strings.EqualFold(strings.TrimSpace(fd.Name), strings.TrimSpace(savedClock.Font)) {
				clockFontIdx = i
				break
			}
		}
	}
	calDefs := discoverCalendarFonts()
	calFontIdx := 0
	if len(calDefs) == 0 {
		calFontIdx = -1
	} else if strings.TrimSpace(savedClock.CalFont) != "" {
		for i, fd := range calDefs {
			if strings.EqualFold(strings.TrimSpace(fd.Name), strings.TrimSpace(savedClock.CalFont)) {
				calFontIdx = i
				break
			}
		}
	}
	if !showClock && !showCal {
		showClock = true
	}
	m := model{
		page:            pageHome,
		now:             now,
		lastTick:        now,
		uptimeText:      "--",
		mode:            defaultMode,
		palette:         defaultPalette,
		paletteType:     defaultPaletteType,
		statusTheme:     defaultStatusTheme,
		starshipPrompt:  defaultStarship,
		profile:         defaultProfile,
		themeSource:     defaultSource,
		presetFamily:    "tokyo-night",
		presetVariant:   "night",
		lastStatus:      "Ready",
		barSpring:       harmonica.NewSpring(harmonica.FPS(20), 4.6, 0.90),
		clockFontDefs:   fontDefs,
		clockFontIdx:    clockFontIdx,
		clockPatterns:   []string{"wave", "stripes", "pulse", "solid", "outline", "sweep", "neon", "calm", "shimmer"},
		clockLoc:        clockLoc,
		clockOnly:       true,
		miniShowClock:   showClock,
		miniShowCal:     showCal,
		calFontDefs:     calDefs,
		calFontIdx:      calFontIdx,
		widgetBattery:   true,
		widgetCPU:       true,
		widgetRAM:       true,
		widgetWeather:   true,
		extractSwatches: map[string]string{},
	}
	if strings.TrimSpace(savedClock.Pattern) != "" {
		for i, p := range m.clockPatterns {
			if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(savedClock.Pattern)) {
				m.patternIndex = i
				break
			}
		}
	}
	m.clockGlyphs = loadClockGlyphSet(m.clockFontDefs, m.clockFontIdx)
	m.calGlyphs = loadCalendarGlyphSet(m.calFontDefs, m.calFontIdx)
	m.loadStarshipSettings()
	m.loadThemeStateFromBackups()
	m.loadShellSettings()
	m.loadPreviewColors()
	if m.themeSource == "wallpaper" {
		m.extractLoading = true
	}
	m.lastAppliedTheme = m.applyCacheSignature()
	return m
}

func (m *model) loadThemeStateFromBackups() {
	if len(m.backups) == 0 {
		m.normalizeThemeSelection()
		return
	}
	meta := m.backups[0].Meta
	if v := strings.TrimSpace(meta["theme_source"]); v != "" {
		m.themeSource = v
	}
	if v := strings.TrimSpace(meta["mode"]); v != "" {
		m.mode = v
	}
	if v := strings.TrimSpace(meta["status_palette"]); v != "" {
		m.palette = v
	}
	if v := strings.TrimSpace(meta["status_theme"]); v != "" {
		m.statusTheme = v
	}
	if v := strings.TrimSpace(meta["type"]); v != "" {
		m.paletteType = canonicalPaletteType(v)
	} else if v := strings.TrimSpace(meta["auto_selected_scheme"]); v != "" {
		m.paletteType = canonicalPaletteType(v)
	}
	if v := strings.TrimSpace(meta["style_family"]); v != "" {
		m.profile = v
	} else if v := strings.TrimSpace(meta["profile"]); v != "" {
		m.profile = v
	} else if v := strings.TrimSpace(meta["style_preset"]); v != "" {
		m.profile = canonicalProfile(v)
	}
	if v := strings.TrimSpace(meta["preset_family"]); v != "" {
		m.presetFamily = v
	}
	if v := strings.TrimSpace(meta["preset_variant"]); v != "" {
		m.presetVariant = v
	}
	if v := strings.TrimSpace(meta["starship_prompt"]); v != "" {
		m.starshipPrompt = v
	}
	if v := strings.TrimSpace(meta["text_color_override"]); v != "" {
		m.textColor = v
	}
	if v := strings.TrimSpace(meta["cursor_color_override"]); v != "" {
		m.cursorColor = v
	}
	if v := strings.TrimSpace(meta["ansi_red_override"]); v != "" {
		m.ansiRed = v
	}
	if v := strings.TrimSpace(meta["ansi_green_override"]); v != "" {
		m.ansiGreen = v
	}
	if v := strings.TrimSpace(meta["ansi_yellow_override"]); v != "" {
		m.ansiYellow = v
	}
	if v := strings.TrimSpace(meta["ansi_blue_override"]); v != "" {
		m.ansiBlue = v
	}
	if v := strings.TrimSpace(meta["ansi_magenta_override"]); v != "" {
		m.ansiMagenta = v
	}
	if v := strings.TrimSpace(meta["ansi_cyan_override"]); v != "" {
		m.ansiCyan = v
	}
	m.normalizeThemeSelection()
}

func (m *model) loadStarshipSettings() {
	settings, ok := loadTooieSettings()
	if !ok {
		m.starshipPrompt = normalizeStarshipPrompt(m.starshipPrompt)
		return
	}
	m.starshipPrompt = normalizeStarshipPrompt(settings.Starship.Prompt)
}

func (m *model) normalizeThemeSelection() {
	m.mode = canonicalMode(m.mode)
	m.profile = canonicalProfile(m.profile)
	m.paletteType = canonicalPaletteType(m.paletteType)
	m.statusTheme = normalizeStatusTheme(m.statusTheme)
	if !contains(themeSources, m.themeSource) {
		m.themeSource = defaultSource
	}
	if !contains(modePresets, m.mode) {
		m.mode = defaultMode
	}
	if !contains(profilePresets, m.profile) {
		m.profile = defaultProfile
	}
	if !contains(paletteTypePresets, m.paletteType) {
		m.paletteType = defaultPaletteType
	}
	if !contains(statusThemePresets, m.statusTheme) {
		m.statusTheme = defaultStatusTheme
	}
	m.starshipPrompt = normalizeStarshipPrompt(m.starshipPrompt)
	if !contains(presetFamilyOrder, m.presetFamily) {
		m.presetFamily = presetFamilyOrder[0]
	}
	variants := presetVariantsByFamily[m.presetFamily]
	if len(variants) == 0 {
		m.presetVariant = ""
		return
	}
	if !contains(variants, m.presetVariant) {
		m.presetVariant = variants[0]
	}
	m.clampMergedSettingIndex()
}

func defaultShellSettings() persistedShellSettings {
	return persistedShellSettings{
		WidgetBattery: true,
		WidgetCPU:     true,
		WidgetRAM:     true,
		WidgetWeather: true,
	}
}

func (m *model) loadShellSettings() {
	settings, ok := loadPersistedShellSettings()
	if !ok {
		settings = loadShellSettingsFromBackups(m.backups)
	}
	m.applyShellSettings(settings)
}

func (m *model) applyShellSettings(settings persistedShellSettings) {
	m.widgetBattery = settings.WidgetBattery
	m.widgetCPU = settings.WidgetCPU
	m.widgetRAM = settings.WidgetRAM
	m.widgetWeather = settings.WidgetWeather
}

func (m model) currentShellSettings() persistedShellSettings {
	return persistedShellSettings{
		WidgetBattery: m.widgetBattery,
		WidgetCPU:     m.widgetCPU,
		WidgetRAM:     m.widgetRAM,
		WidgetWeather: m.widgetWeather,
	}
}

func loadShellSettingsFromBackups(backups []backup) persistedShellSettings {
	out := defaultShellSettings()
	if len(backups) == 0 {
		return out
	}
	meta := backups[0].Meta
	out.WidgetBattery = parseOnOffDefault(meta["widget_battery"], true)
	out.WidgetCPU = parseOnOffDefault(meta["widget_cpu"], true)
	out.WidgetRAM = parseOnOffDefault(meta["widget_ram"], true)
	out.WidgetWeather = parseOnOffDefault(meta["widget_weather"], true)
	return out
}

func loadPersistedShellSettings() (persistedShellSettings, bool) {
	if settings, ok := loadTooieSettings(); ok {
		return settings.Widgets, true
	}
	return loadLegacyShellSettings()
}

func loadLegacyShellSettings() (persistedShellSettings, bool) {
	out := defaultShellSettings()
	path := shellSettingsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, false
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, false
	}
	return out, true
}

func savePersistedShellSettings(settings persistedShellSettings) error {
	conf, ok := loadTooieSettings()
	if !ok {
		conf = defaultTooieSettings()
	}
	conf.Widgets = settings
	if err := saveTooieSettings(conf); err != nil {
		return err
	}
	return saveLegacyShellSettings(settings)
}

func saveLegacyShellSettings(settings persistedShellSettings) error {
	path := shellSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func parseOnOffDefault(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func canonicalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return defaultMode
	}
}

func canonicalPaletteType(name string) string {
	v := strings.ToLower(strings.TrimSpace(name))
	v = strings.TrimPrefix(v, "scheme-")
	switch v {
	case "", "default", "auto":
		return defaultPaletteType
	case "tonal-spot", "expressive", "fidelity", "content", "vibrant", "neutral", "rainbow", "fruit-salad":
		return v
	case "monochrome":
		return "neutral"
	default:
		return v
	}
}

func canonicalProfile(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto", "adaptive":
		return "auto"
	case "source-0", "source-1", "source-2", "source-3", "source-4":
		return strings.ToLower(strings.TrimSpace(name))
	case "prefer-saturation", "prefer-lightness", "prefer-darkness", "prefer-less-saturation", "prefer-value", "prefer-closest-fallback":
		return strings.ToLower(strings.TrimSpace(name))
	case "dominant":
		return "source-0"
	case "alt-1":
		return "source-1"
	case "alt-2":
		return "source-2"
	case "alt-3":
		return "source-3"
	case "alt-4":
		return "source-4"
	case "default", "balanced", "vivid", "mellow", "friendly", "positive", "punchy", "playful", "energetic", "creative":
		return "auto"
	case "soft-pastel", "studio-dark", "neon-night", "warm-retro", "vivid-noir", "arctic-calm":
		return "auto"
	case "catppuccin", "onedark", "tokyonight", "gruvbox", "dracula", "nord":
		return "auto"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func (m model) Init() tea.Cmd {
	if m.clockOnly {
		return tickClockOnly()
	}
	cmds := []tea.Cmd{
		tickClock(),
		pollMetrics(),
		loadSystemInfoCmd(),
		syncTmuxWidgetSettingsCmd(m.currentShellSettings()),
	}
	if m.themeSource == "wallpaper" {
		cmds = append(cmds, loadExtractSwatchesCmd(m.mode, m.paletteType))
	}
	if !m.metricsPaused {
		cmds = append(cmds, tickMetrics())
	}
	return tea.Batch(cmds...)
}

func tickClockOnly() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func tickApply() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return applyTickMsg(t)
	})
}

func tickClock() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func tickMetrics() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return metricsTickMsg(t)
	})
}

func pollMetrics() tea.Cmd {
	return func() tea.Msg {
		up, err := host.Uptime()
		if err != nil {
			return metricsMsg{err: err}
		}

		d := up / 86400
		h := (up % 86400) / 3600

		cpuPct, memUsedBytes, memTotalBytes, backendOK := readTooieResources(900 * time.Millisecond)
		if !isTermuxRuntime() {
			backendOK = false
		}
		if !backendOK {
			cpuVals, cpuErr := cpu.Percent(250*time.Millisecond, false)
			if cpuErr != nil {
				return metricsMsg{err: cpuErr}
			}
			vm, vmErr := mem.VirtualMemory()
			if vmErr != nil {
				return metricsMsg{err: vmErr}
			}
			if len(cpuVals) > 0 {
				cpuPct = cpuVals[0]
			}
			memUsedBytes = effectiveMemoryUsedBytes(vm.Total, vm.Available, vm.Used)
			memTotalBytes = vm.Total
		}
		if cpuPct > 0 && cpuPct <= 1 {
			cpuPct *= 100
		}
		cpuPct = clampPct(cpuPct)

		st, stErr := disk.Usage("/data")
		if stErr != nil {
			st, stErr = disk.Usage("/")
		}
		stUsedGB := 0.0
		stTotalGB := 0.0
		stPct := 0.0
		if stErr == nil && st != nil {
			stUsedGB = float64(st.Used) / (1024 * 1024 * 1024)
			stTotalGB = float64(st.Total) / (1024 * 1024 * 1024)
			stPct = st.UsedPercent
		}

		return metricsMsg{
			cpuPercent:  cpuPct,
			ramUsedGB:   float64(memUsedBytes) / (1024 * 1024 * 1024),
			ramTotalGB:  float64(memTotalBytes) / (1024 * 1024 * 1024),
			storUsedGB:  stUsedGB,
			storTotalGB: stTotalGB,
			storPercent: stPct,
			uptimeSecs:  up,
			uptimeText:  fmt.Sprintf("%dd %dh", d, h),
		}
	}
}

func isTermuxRuntime() bool {
	if strings.Contains(homeDir, "/data/data/com.termux") {
		return true
	}
	return strings.TrimSpace(os.Getenv("PREFIX")) == "/data/data/com.termux/files/usr"
}

func effectiveMemoryUsedBytes(total, available, used uint64) uint64 {
	if total > 0 && available > 0 && total >= available {
		return total - available
	}
	return used
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		sizeChanged := m.width != msg.Width || m.height != msg.Height
		m.width, m.height = msg.Width, msg.Height
		if sizeChanged && m.page == pageHome {
			clearPinnedSixelCache()
			return m, m.homeRedrawCmd(false)
		}
		return m, nil
	case tickMsg:
		now := time.Time(msg)
		if m.clockLoc != nil {
			now = now.In(m.clockLoc)
		}
		if m.clockOnly {
			dt := now.Sub(m.lastTick).Seconds()
			if dt <= 0 || dt > 2 {
				dt = 0.25
			}
			m.lastTick = now
			m.now = now
			m.clockPhase += dt * 0.20
			return m, tickClockOnly()
		}
		dt := now.Sub(m.lastTick).Seconds()
		if dt <= 0 || dt > 1 {
			dt = 1.0 / 30.0
		}
		m.lastTick = now
		m.now = now
		ramTarget := 0.0
		if m.ramFiltered > 0 {
			ramTarget = m.ramFiltered
		} else if m.ramTotalGB > 0 {
			ramTarget = (m.ramUsedGB / m.ramTotalGB) * 100.0
		}
		cpuTarget := clampPct(m.cpuFiltered)
		if cpuTarget == 0 {
			cpuTarget = clampPct(m.cpuPercent)
		}
		ramVizTarget := clampPct(ramTarget)
		storTarget := clampPct(m.storFiltered)
		if storTarget == 0 {
			storTarget = clampPct(m.storPercent)
		}
		if m.introActive(now) {
			introCPU, introRAM, introStor := m.introTargets(now)
			weight := introWeight(now, m.introUntil)
			cpuTarget = blendPercent(cpuTarget, introCPU, weight)
			ramVizTarget = blendPercent(ramVizTarget, introRAM, weight)
			storTarget = blendPercent(storTarget, introStor, weight)
		}
		m.cpuViz, m.cpuVel = m.barSpring.Update(m.cpuViz, m.cpuVel, cpuTarget)
		m.ramViz, m.ramVel = m.barSpring.Update(m.ramViz, m.ramVel, ramVizTarget)
		m.storViz, m.storVel = m.barSpring.Update(m.storViz, m.storVel, storTarget)
		m.clockPhase += dt * 0.45
		m.advanceSwitchAnimation(dt)
		return m, tickClock()
	case metricsTickMsg:
		if m.metricsPaused {
			return m, nil
		}
		return m, tea.Batch(pollMetrics(), tickMetrics())
	case metricsMsg:
		if msg.err != nil {
			m.metricsErr = msg.err
			return m, nil
		}
		m.metricsErr = nil
		m.cpuPercent = msg.cpuPercent
		m.ramUsedGB = msg.ramUsedGB
		m.ramTotalGB = msg.ramTotalGB
		m.storUsedGB = msg.storUsedGB
		m.storTotalGB = msg.storTotalGB
		m.storPercent = msg.storPercent
		m.uptimeSeconds = msg.uptimeSecs
		m.uptimeText = msg.uptimeText
		ramPct := 0.0
		if msg.ramTotalGB > 0 {
			ramPct = (msg.ramUsedGB / msg.ramTotalGB) * 100.0
		}
		const alpha = 0.18
		if m.cpuFiltered == 0 {
			m.cpuFiltered = clampPct(msg.cpuPercent)
		} else {
			m.cpuFiltered = clampPct((alpha * msg.cpuPercent) + ((1 - alpha) * m.cpuFiltered))
		}
		if m.ramFiltered == 0 {
			m.ramFiltered = clampPct(ramPct)
		} else {
			m.ramFiltered = clampPct((alpha * ramPct) + ((1 - alpha) * m.ramFiltered))
		}
		if m.storFiltered == 0 {
			m.storFiltered = clampPct(msg.storPercent)
		} else {
			m.storFiltered = clampPct((alpha * msg.storPercent) + ((1 - alpha) * m.storFiltered))
		}
		return m, nil
	case systemInfoMsg:
		m.systemInfo = msg.info
		return m, nil
	case applyTickMsg:
		if !m.applying {
			return m, nil
		}
		if st, ok := readApplyProgressState(); ok {
			if strings.TrimSpace(st.Label) != "" {
				m.applyLabel = strings.TrimSpace(st.Label)
			}
			if st.Progress >= 0 {
				m.applyTarget = st.Progress
			}
		} else if m.applyTarget < 0.08 {
			m.applyTarget = 0.08
		}
		m.applyProgress, m.applyVel = m.barSpring.Update(m.applyProgress, m.applyVel, m.applyTarget)
		return m, tickApply()
	case applyDoneMsg:
		m.applying = false
		m.applyProgress = 1.0
		m.applyVel = 0
		m.applyTarget = 1.0
		if msg.err != nil {
			s := strings.TrimSpace(msg.out)
			if s == "" {
				m.lastStatus = msg.label + " failed: " + msg.err.Error()
			} else {
				lines := strings.Split(s, "\n")
				last := strings.TrimSpace(lines[len(lines)-1])
				if last == "" {
					last = msg.err.Error()
				}
				m.lastStatus = msg.label + " failed: " + last
			}
		} else {
			switch {
			case msg.previewOnly:
				if m.themeSource == "preset" {
					m.lastStatus = "Preset preview updated"
				} else {
					m.lastStatus = "Colors updated"
				}
			case msg.reused:
				m.lastStatus = msg.label + " completed from cached preview"
			default:
				m.lastStatus = msg.label + " completed"
			}
			if !msg.previewOnly {
				m.lastAppliedTheme = msg.cacheKey
			}
			if strings.TrimSpace(msg.backupID) != "" {
				m.previewBackupID = msg.backupID
			}
			if msg.previewOnly {
				m.previewCacheKey = msg.cacheKey
			} else if msg.cacheKey == m.previewCacheKey && msg.reused {
				m.previewCacheKey = ""
				m.previewBackupID = ""
			}
		}
		_ = os.Remove(applyProgressPath())
		m.backups = loadBackups()
		if m.backupIndex >= len(m.backups) {
			m.backupIndex = max(0, len(m.backups)-1)
		}
		if id := strings.TrimSpace(msg.backupID); id != "" {
			if idx := findBackupIndexByID(m.backups, id); idx >= 0 {
				m.backupIndex = idx
			}
		}
		m.loadPreviewColors()
		if m.themeSource == "wallpaper" {
			return m, loadExtractSwatchesCmd(m.mode, m.paletteType)
		}
		return m, nil
	case extractSwatchesMsg:
		m.extractLoading = false
		if msg.err != nil {
			m.extractError = msg.err.Error()
			return m, nil
		}
		m.extractError = ""
		m.extractSwatchKey = msg.Key
		m.extractSwatches = msg.Swatches
		return m, nil
	case statusMsg:
		m.lastStatus = string(msg)
		var post tea.Cmd
		if strings.HasPrefix(m.lastStatus, "Bootstrap defaults restored.") {
			m.applyShellSettings(defaultShellSettings())
			if err := savePersistedShellSettings(m.currentShellSettings()); err != nil {
				m.lastStatus = "Reset completed but failed to save settings: " + err.Error()
			} else {
				post = syncTmuxWidgetSettingsCmd(m.currentShellSettings())
			}
		}
		m.backups = loadBackups()
		if m.backupIndex >= len(m.backups) {
			m.backupIndex = max(0, len(m.backups)-1)
		}
		m.loadPreviewColors()
		m.lastAppliedTheme = m.applyCacheSignature()
		if m.themeSource == "wallpaper" {
			if post != nil {
				return m, tea.Batch(post, loadExtractSwatchesCmd(m.mode, m.paletteType))
			}
			return m, loadExtractSwatchesCmd(m.mode, m.paletteType)
		}
		return m, post
	case tea.KeyMsg:
		if m.clockOnly {
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				return m, tea.Quit
			case "f":
				if m.miniShowClock {
					m.cycleClockFont()
				}
				return m, nil
			case "a":
				if m.miniShowClock {
					m.cycleClockPattern()
				}
				return m, nil
			case "d":
				if m.miniShowCal {
					m.cycleCalendarFont()
				}
				return m, nil
			}
			return m, nil
		}
		if m.canSwitchPage() {
			switch msg.String() {
			case "tab", "right", "l":
				m.page = nextPageIndex(m.page)
				if m.page == pageHome {
					m.startHomeIntro()
					if m.metricsPaused {
						return m, nil
					}
					return m, tea.Batch(tea.ClearScreen, pollMetrics())
				}
				return m, tea.ClearScreen
			case "left", "h":
				m.page = prevPageIndex(m.page)
				if m.page == pageHome {
					m.startHomeIntro()
					if m.metricsPaused {
						return m, nil
					}
					return m, tea.Batch(tea.ClearScreen, pollMetrics())
				}
				return m, tea.ClearScreen
			}
		}

		if m.page == pageHome {
			return m.updateHomePage(msg)
		}

		if m.pickerTarget != "" {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.pickerTarget = ""
				m.pickerIndex = 0
				return m, nil
			case "up", "k":
				if m.pickerIndex > 0 {
					m.pickerIndex--
				}
				return m, nil
			case "down", "j":
				opts := m.colorPickerOptions(m.pickerTarget)
				if m.pickerIndex < len(opts)-1 {
					m.pickerIndex++
				}
				return m, nil
			case "enter":
				opts := m.colorPickerOptions(m.pickerTarget)
				if len(opts) == 0 {
					m.pickerTarget = ""
					m.pickerIndex = 0
					return m, nil
				}
				if m.pickerIndex >= len(opts) {
					m.pickerIndex = len(opts) - 1
				}
				selected := opts[m.pickerIndex]
				m.setColorTarget(m.pickerTarget, selected.Hex)
				m.pickerTarget = ""
				m.pickerIndex = 0
				return m, nil
			}
			return m, nil
		}

		if m.settingMenuTarget != "" {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.settingMenuTarget = ""
				m.settingMenuIndex = 0
				return m, nil
			case "up", "k":
				if m.settingMenuIndex > 0 {
					m.settingMenuIndex--
				}
				return m, nil
			case "down", "j":
				opts := m.settingMenuChoices(m.settingMenuTarget)
				if m.settingMenuIndex < len(opts)-1 {
					m.settingMenuIndex++
				}
				return m, nil
			case " ", "space":
				if m.settingMenuTarget == "segments" {
					opts := m.settingMenuChoices(m.settingMenuTarget)
					if len(opts) == 0 {
						return m, nil
					}
					if m.settingMenuIndex >= len(opts) {
						m.settingMenuIndex = len(opts) - 1
					}
					if m.settingMenuIndex < 0 {
						m.settingMenuIndex = 0
					}
					m.toggleSegment(opts[m.settingMenuIndex].Value)
				}
				return m, nil
			case "enter":
				target := m.settingMenuTarget
				if target == "segments" {
					m.settingMenuTarget = ""
					m.settingMenuIndex = 0
					return m, nil
				}
				opts := m.settingMenuChoices(m.settingMenuTarget)
				if len(opts) == 0 {
					m.settingMenuTarget = ""
					m.settingMenuIndex = 0
					return m, nil
				}
				if m.settingMenuIndex >= len(opts) {
					m.settingMenuIndex = len(opts) - 1
				}
				selected := opts[m.settingMenuIndex]
				m.applySettingChoice(m.settingMenuTarget, selected.Value)
				m.settingMenuTarget = ""
				m.settingMenuIndex = 0
				if m.themeSource == "wallpaper" && (target == "mode" || target == "palette_type" || target == "theme_source") {
					m.extractLoading = true
					return m, loadExtractSwatchesCmd(m.mode, m.paletteType)
				}
				return m, nil
			}
			return m, nil
		}

		if m.showHints {
			switch msg.String() {
			case "?", "esc":
				m.showHints = false
				return m, nil
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		if m.showBackups {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "b":
				m.showBackups = false
				return m, nil
			case "up", "k":
				if m.backupIndex > 0 {
					m.backupIndex--
					m.loadPreviewColors()
				}
				return m, nil
			case "down", "j":
				if m.backupIndex < len(m.backups)-1 {
					m.backupIndex++
					m.loadPreviewColors()
				}
				return m, nil
			case "enter":
				if len(m.backups) == 0 {
					m.lastStatus = "No backups to restore"
					return m, nil
				}
				id := m.backups[m.backupIndex].ID
				if err := ensureTooieSupportScripts(); err != nil {
					m.lastStatus = "Restore unavailable: " + err.Error()
					return m, nil
				}
				cmd := exec.Command(currentRestoreScriptPath(), id)
				m.lastStatus = "Restoring " + id + "..."
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					if err != nil {
						return statusMsg("Restore failed: " + err.Error())
					}
					return statusMsg("Restore completed")
				})
			}
			return m, nil
		}

		if m.showApplyConfirm {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "n":
				m.showApplyConfirm = false
				m.applyConfirmIdx = 0
				m.lastStatus = "Apply canceled"
				return m, nil
			case "left", "h", "up", "k":
				m.applyConfirmIdx = 0
				return m, nil
			case "right", "l", "down", "j", "tab":
				m.applyConfirmIdx = 1
				return m, nil
			case "A", "y":
				m.showApplyConfirm = false
				m.applyConfirmIdx = 0
				return m.requestThemeApply()
			case "a", "enter":
				confirm := m.applyConfirmIdx == 0
				m.showApplyConfirm = false
				m.applyConfirmIdx = 0
				if confirm {
					return m.requestThemeApply()
				}
				m.lastStatus = "Apply canceled"
				return m, nil
			}
			return m, nil
		}

		if m.customizing {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.customizing = false
				m.customIndex = 0
				return m, nil
			case "up", "k":
				if m.customIndex > 0 {
					m.customIndex--
				}
				return m, nil
			case "down", "j":
				if m.customIndex < len(m.customizeItems())-1 {
					m.customIndex++
				}
				return m, nil
			case "enter":
				return m.activateCustomizeItem()
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHints = true
			return m, nil
		case "b":
			m.showBackups = true
			return m, nil
		case "up", "k":
			if m.settingIndex > 0 {
				m.settingIndex--
			}
			return m, nil
		case "down", "j":
			if m.settingIndex < len(m.mergedPageItems())-1 {
				m.settingIndex++
			}
			return m, nil
		case "u", "U":
			m.refreshCurrentPreviewColors()
			return m.startApply(m.themeActionLabel(true), true, true)
		case "s":
			m.themeSource = nextThemeSource(m.themeSource)
			m.clampMergedSettingIndex()
			if m.themeSource == "wallpaper" {
				m.extractLoading = true
				return m, loadExtractSwatchesCmd(m.mode, m.paletteType)
			}
			return m, nil
		case "S":
			if m.page == pageTheme {
				m.openSettingMenu("starship_prompt")
			}
			return m, nil
		case "m":
			if m.themeSource == "preset" {
				return m, nil
			}
			m.mode = nextMode(m.mode)
			m.extractLoading = true
			return m, loadExtractSwatchesCmd(m.mode, m.paletteType)
		case "f":
			if m.themeSource == "preset" {
				return m, nil
			}
			m.profile = nextProfile(m.profile)
			return m, nil
		case "p", "P":
			if m.themeSource == "preset" {
				return m, nil
			}
			m.paletteType = nextPaletteType(m.paletteType)
			m.extractLoading = true
			return m, loadExtractSwatchesCmd(m.mode, m.paletteType)
		case "t", "T":
			m.statusTheme = nextStatusTheme(m.statusTheme)
			return m, nil
		case "a":
			if m.page == pageTheme {
				m.showApplyConfirm = true
				m.applyConfirmIdx = 0
			}
			return m, nil
		case "A":
			if m.page == pageTheme {
				return m.requestThemeApply()
			}
			return m, nil
		case "enter":
			return m.activateSetting()
		}
	}
	return m, nil
}

func (m model) settings() []settingItem {
	items := []settingItem{
		{Label: hotkeyLabel("source", "s", "6") + ": " + displayThemeSource(m.themeSource), Target: "theme_source"},
	}
	if m.themeSource == "preset" {
		items = append(items,
			settingItem{Label: "Preset: " + displayPresetFamily(m.presetFamily), Target: "preset_family"},
			settingItem{Label: "Preset Variant: " + displayPresetVariant(m.presetVariant), Target: "preset_variant"},
		)
	} else {
		items = append(items,
			settingItem{Label: hotkeyLabel("Mode", "m", "5") + ": " + displayMode(m.mode), Target: "mode"},
			settingItem{Label: hotkeyLabel("Palette", "p", "13") + ": " + displayPaletteType(m.paletteType), Target: "palette_type"},
			settingItem{Label: hotkeyLabel("Primary", "f", "2") + ": " + m.extractStateLabel(), Target: "profile"},
		)
	}
	items = append(items,
		settingItem{Label: "Customize Colors", Target: "customize"},
	)
	return items
}

func (m model) settingsPageItems() []settingItem {
	starshipState := "N/A"
	if m.starshipConfigEnabled() {
		starshipState = displayStarshipPrompt(m.starshipPrompt)
	}
	return []settingItem{
		{Label: hotkeyLabel("tmux status", "t", "3") + ": " + displayStatusTheme(m.statusTheme), Target: "status_theme"},
		{Label: "tmux status segments: " + m.segmentSummary(), Target: "segments"},
		{Label: hotkeyLabel("Starship", "S", "10") + ": " + starshipState, Target: "starship_prompt"},
	}
}

func (m model) mergedPageItems() []settingItem {
	items := append([]settingItem{}, m.settings()...)
	return append(items, m.settingsPageItems()...)
}

func (m *model) clampMergedSettingIndex() {
	total := len(m.mergedPageItems())
	if total == 0 {
		m.settingIndex = 0
		return
	}
	if m.settingIndex < 0 {
		m.settingIndex = 0
		return
	}
	if m.settingIndex >= total {
		m.settingIndex = total - 1
	}
}

func (m model) activateSetting() (tea.Model, tea.Cmd) {
	if m.applying {
		return m, nil
	}
	items := m.mergedPageItems()
	if m.settingIndex < 0 || m.settingIndex >= len(items) {
		return m, nil
	}
	switch items[m.settingIndex].Target {
	case "theme_source":
		m.openSettingMenu("theme_source")
		return m, nil
	case "status_theme":
		m.openSettingMenu("status_theme")
		return m, nil
	case "segments":
		m.openSettingMenu("segments")
		return m, nil
	case "starship_prompt":
		m.openSettingMenu("starship_prompt")
		return m, nil
	case "mode":
		m.openSettingMenu("mode")
		return m, nil
	case "profile":
		m.openSettingMenu("profile")
		return m, nil
	case "palette_type":
		m.openSettingMenu("palette_type")
		return m, nil
	case "preset_family":
		m.openSettingMenu("preset_family")
		return m, nil
	case "preset_variant":
		m.openSettingMenu("preset_variant")
		return m, nil
	case "customize":
		m.customizing = true
		m.customIndex = 0
		return m, nil
	}
	return m, nil
}

func onOffLabel(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func (m model) segmentEnabled(target string) bool {
	switch target {
	case "widget_battery":
		return m.widgetBattery
	case "widget_cpu":
		return m.widgetCPU
	case "widget_ram":
		return m.widgetRAM
	case "widget_weather":
		return m.widgetWeather
	default:
		return false
	}
}

func (m model) segmentSummary() string {
	labels := []string{}
	if m.widgetBattery {
		labels = append(labels, "Battery")
	}
	if m.widgetCPU {
		labels = append(labels, "CPU")
	}
	if m.widgetRAM {
		labels = append(labels, "RAM")
	}
	if m.widgetWeather {
		labels = append(labels, "Weather")
	}
	if len(labels) == 0 {
		return "None"
	}
	return strings.Join(labels, ", ")
}

func (m *model) toggleSegment(target string) {
	prev := m.segmentEnabled(target)
	switch target {
	case "widget_battery":
		m.widgetBattery = !m.widgetBattery
	case "widget_cpu":
		m.widgetCPU = !m.widgetCPU
	case "widget_ram":
		m.widgetRAM = !m.widgetRAM
	case "widget_weather":
		m.widgetWeather = !m.widgetWeather
	default:
		return
	}
	m.startSwitchAnimation(target, prev, m.segmentEnabled(target))
	if err := savePersistedShellSettings(m.currentShellSettings()); err != nil {
		m.lastStatus = "Failed to save settings: " + err.Error()
		return
	}
	m.lastStatus = "Segments updated"
	go func(cfg persistedShellSettings) {
		_ = syncTmuxWidgetSettings(cfg)
	}(m.currentShellSettings())
}

func (m *model) startSwitchAnimation(target string, from, to bool) {
	if target == "" || from == to {
		m.switchAnimTarget = ""
		m.switchAnimProg = 1
		return
	}
	m.switchAnimTarget = target
	m.switchAnimFrom = from
	m.switchAnimTo = to
	m.switchAnimProg = 0
}

func (m *model) advanceSwitchAnimation(dt float64) {
	if strings.TrimSpace(m.switchAnimTarget) == "" {
		return
	}
	if dt <= 0 {
		dt = 1.0 / 30.0
	}
	m.switchAnimProg += dt * 4.2
	if m.switchAnimProg >= 1 {
		m.switchAnimProg = 1
		m.switchAnimTarget = ""
	}
}

func onOffFlag(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func tmuxStatusRightTemplate() string {
	return "#($HOME/.config/tooie/configs/tmux/run-system-widget all)#($HOME/.config/tooie/configs/tmux/widget-weather)"
}

func syncTmuxWidgetSettings(settings persistedShellSettings) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil
	}
	options := []struct {
		key string
		val bool
	}{
		{key: "@status-tmux-widget-battery", val: settings.WidgetBattery},
		{key: "@status-tmux-widget-cpu", val: settings.WidgetCPU},
		{key: "@status-tmux-widget-ram", val: settings.WidgetRAM},
		{key: "@status-tmux-widget-weather", val: settings.WidgetWeather},
	}
	for _, item := range options {
		_ = exec.Command("tmux", "set-option", "-g", item.key, onOffFlag(item.val)).Run()
	}
	_ = exec.Command("tmux", "set-option", "-g", "base-index", "1").Run()
	_ = exec.Command("tmux", "set-window-option", "-g", "pane-base-index", "1").Run()
	_ = exec.Command("tmux", "set-option", "-g", "renumber-windows", "on").Run()
	_ = exec.Command("tmux", "set-option", "-g", "status-right", tmuxStatusRightTemplate()).Run()
	_ = exec.Command("tmux", "refresh-client", "-S").Run()
	return nil
}

func syncTmuxWidgetSettingsCmd(settings persistedShellSettings) tea.Cmd {
	return func() tea.Msg {
		_ = syncTmuxWidgetSettings(settings)
		return nil
	}
}

func runResetBootstrapCmd() tea.Cmd {
	return func() tea.Msg {
		if err := ensureTooieSupportScripts(); err != nil {
			return statusMsg("Reset unavailable: " + err.Error())
		}
		cmd := exec.Command(currentResetScriptPath())
		out, err := cmd.CombinedOutput()
		outText := strings.TrimSpace(string(out))
		if err != nil {
			if outText == "" {
				return statusMsg("Reset failed: " + err.Error())
			}
			lines := strings.Split(outText, "\n")
			last := strings.TrimSpace(lines[len(lines)-1])
			if last == "" {
				last = err.Error()
			}
			return statusMsg("Reset failed: " + last)
		}
		if outText == "" {
			return statusMsg("Bootstrap configs reset completed")
		}
		lines := strings.Split(outText, "\n")
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" {
			last = "Bootstrap configs reset completed"
		}
		return statusMsg(last)
	}
}

func runSetupBtopCmd() tea.Cmd {
	return func() tea.Msg {
		if err := ensureTooieSupportScripts(); err != nil {
			return statusMsg("Btop setup unavailable: " + err.Error())
		}
		cmd := exec.Command(currentBtopSetupScriptPath())
		out, err := cmd.CombinedOutput()
		outText := strings.TrimSpace(string(out))
		if err != nil {
			if outText == "" {
				return statusMsg("Btop setup failed: " + err.Error())
			}
			lines := strings.Split(outText, "\n")
			last := strings.TrimSpace(lines[len(lines)-1])
			if last == "" {
				last = err.Error()
			}
			return statusMsg("Btop setup failed: " + last)
		}
		if outText == "" {
			return statusMsg("Btop setup complete")
		}
		lines := strings.Split(outText, "\n")
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" {
			last = "Btop setup complete"
		}
		return statusMsg(last)
	}
}

type statusMsg string
type colorOption struct {
	Label string
	Hex   string
}

type customizeItem struct {
	Label  string
	Target string
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	if m.clockOnly {
		if m.miniShowClock && m.miniShowCal {
			return m.renderClockCalendarView()
		}
		if m.miniShowCal {
			return m.renderCalOnlyView()
		}
		return m.renderClockOnlyView()
	}

	const outerPad = 1
	innerW := max(20, m.width-(outerPad*2))
	innerH := max(8, m.height-(outerPad*2))
	renderPadBottom := outerPad
	if m.page == pageHome {
		renderPadBottom = 0
		innerH = max(8, m.height-outerPad-renderPadBottom)
	}

	title := headerChip("Tooie", "12")
	if m.page == pageTheme {
		title = headerChip("Tooie / Settings", "12")
	}
	statusColor := ensureReadableTextColor(
		m.themeRoleColor("background", "#11131c"),
		m.themeRoleColor("text_accent_secondary", m.themeRoleColor("secondary", "#94e2d5")),
		"#8ec3b0",
	)
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
	if strings.Contains(strings.ToLower(m.lastStatus), "failed") {
		errColor := ensureReadableTextColor(
			m.themeRoleColor("background", "#11131c"),
			m.themeRoleColor("error", "#f38ba8"),
			"#e57373",
		)
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(errColor))
	}
	statusText := "status: " + m.lastStatus
	if m.applying {
		statusText = m.renderApplyStatus(innerW)
	}
	status := statusText
	if !m.applying {
		status = statusStyle.Render(statusText)
	}
	hints := ""
	if !m.applying {
		hintColor := ensureReadableTextColor(
			m.themeRoleColor("background", "#11131c"),
			m.themeRoleColor("text_muted", blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), m.themeRoleColor("background", "#11131c"), 0.28)),
			"#b3bcc8",
		)
		hints = lipgloss.NewStyle().Foreground(lipgloss.Color(hintColor)).Render("[? hints]")
	}
	topBar := joinLR(status, hints, innerW)

	panelH := max(4, innerH-3)
	if m.page == pageHome {
		panelH = max(4, innerH-2)
	}
	main := m.renderMain(innerW, panelH)

	body := fmt.Sprintf("%s\n%s\n%s", title, topBar, main)
	overlays := []sixelOverlay(nil)
	if m.page == pageHome {
		header := title
		if notice := m.homeNoticeLine(innerW); notice != "" {
			header = joinLR(title, notice, innerW)
		}
		body = fmt.Sprintf("%s\n%s\n%s", header, main, m.homeHintsLine(innerW))
	}
	rendered := lipgloss.NewStyle().Padding(outerPad, outerPad, renderPadBottom, outerPad).Render(body)
	if len(overlays) > 0 {
		rendered += renderSixelOverlays(overlays)
	}
	return rendered
}

func (m model) renderClockOnlyView() string {
	const outerPad = 1
	usableW := max(24, m.width-(outerPad*2))
	usableH := max(8, m.height-(outerPad*2))

	glyphW, glyphH := clockGlyphMetrics(m.clockGlyphs)
	if m.clockOnly {
		glyphW, glyphH = clockGlyphMetricsNormalized(m.clockGlyphs)
	}
	innerW := max(20, glyphW*2)
	innerH := max(8, (glyphH*2)+1)
	panelW := max(24, min(usableW, innerW+4))
	panelH := max(8, min(usableH, innerH+2))
	if panelH%2 == 0 {
		if panelH < usableH {
			panelH++
		} else if panelH > 8 {
			panelH--
		}
	}

	clockLines := m.renderDashboardVerticalClockTest(max(1, panelW-4), max(1, panelH-2))
	clockBorder := blendHexColor(m.themeRoleColor("primary", "#89b4fa"), m.themeRoleColor("outline", "#565f89"), 0.35)
	body := framedPanel(panelW, panelH, clockBorder, strings.Join(clockLines, "\n"), "", "left", m.clockMeridiemLabel(), "right")
	body = placeCenterBlockStyled(body, usableW)
	baseHints := m.miniHintsText()
	status := baseHints
	if strings.TrimSpace(m.noticeText) != "" && !m.noticeUntil.IsZero() && !m.now.After(m.noticeUntil) {
		status = m.noticeText + "  |  " + baseHints
	}
	hints := lipgloss.NewStyle().
		Foreground(lipgloss.Color(blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#000000", 0.32))).
		Render(status)
	content := body + "\n" + placeCenterStyled(hints, usableW)
	return lipgloss.NewStyle().Padding(outerPad, outerPad, outerPad, outerPad).Render(content)
}

func (m model) renderCalOnlyView() string {
	const outerPad = 1
	usableW := max(24, m.width-(outerPad*2))
	usableH := max(8, m.height-(outerPad*2))
	panelW := max(24, min(usableW, 44))
	panelH := max(8, min(usableH, 20))
	if panelH%2 == 0 {
		if panelH < usableH {
			panelH++
		} else if panelH > 8 {
			panelH--
		}
	}
	body := m.renderCalendarStack(panelW, panelH)
	body = placeCenterBlockStyled(body, usableW)
	baseHints := m.miniHintsText()
	status := baseHints
	if strings.TrimSpace(m.noticeText) != "" && !m.noticeUntil.IsZero() && !m.now.After(m.noticeUntil) {
		status = m.noticeText + "  |  " + baseHints
	}
	hints := lipgloss.NewStyle().
		Foreground(lipgloss.Color(blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#000000", 0.32))).
		Render(status)
	content := body + "\n" + placeCenterStyled(hints, usableW)
	return lipgloss.NewStyle().Padding(outerPad, outerPad, outerPad, outerPad).Render(content)
}

func (m model) renderClockCalendarView() string {
	const outerPad = 1
	usableW := max(48, m.width-(outerPad*2))
	usableH := max(10, m.height-(outerPad*2))

	clockW := max(24, min(44, usableW/2))
	calW := max(24, min(44, usableW-clockW-2))
	if clockW+calW+2 > usableW {
		calW = max(24, usableW-clockW-2)
	}
	rowH := max(8, min(usableH, 20))
	if rowH%2 == 0 {
		if rowH < usableH {
			rowH++
		} else if rowH > 8 {
			rowH--
		}
	}

	clockLines := m.renderDashboardVerticalClockTest(max(1, clockW-4), max(1, rowH-2))
	clockBorder := blendHexColor(m.themeRoleColor("primary", "#89b4fa"), m.themeRoleColor("outline", "#565f89"), 0.35)
	clockPanel := framedPanel(clockW, rowH, clockBorder, strings.Join(clockLines, "\n"), "", "left", m.clockMeridiemLabel(), "right")
	calPanel := m.renderCalendarStack(calW, rowH)
	clockPanel, calPanel = equalizeBlockHeights(clockPanel, calPanel)
	row := lipgloss.JoinHorizontal(lipgloss.Top, clockPanel, "  ", calPanel)
	row = placeCenterBlockStyled(row, usableW)

	baseHints := m.miniHintsText()
	status := baseHints
	if strings.TrimSpace(m.noticeText) != "" && !m.noticeUntil.IsZero() && !m.now.After(m.noticeUntil) {
		status = m.noticeText + "  |  " + baseHints
	}
	hints := lipgloss.NewStyle().
		Foreground(lipgloss.Color(blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#000000", 0.32))).
		Render(status)
	content := row + "\n" + placeCenterStyled(hints, usableW)
	return lipgloss.NewStyle().Padding(outerPad, outerPad, outerPad, outerPad).Render(content)
}

func (m model) renderCalendarStack(w, h int) string {
	if h < 8 {
		return m.renderCalendarPanel(w, h)
	}
	topH := int(math.Round(float64(h) * 0.60))
	if topH < 6 {
		topH = 6
	}
	if topH > h-4 {
		topH = h - 4
	}
	bottomH := h - topH
	if bottomH < 3 {
		bottomH = 3
		topH = max(4, h-bottomH)
	}
	top := m.renderCalendarPanel(w, topH)
	bottom := m.renderMonthCalendarPanel(w, bottomH)
	return top + "\n" + bottom
}

func (m model) miniHintsText() string {
	parts := []string{}
	if m.miniShowClock {
		parts = append(parts, "f: clock font", "a: anim")
	}
	if m.miniShowCal {
		parts = append(parts, "d: date font")
	}
	parts = append(parts, "q: quit")
	return strings.Join(parts, "  ")
}

func (m model) renderCalendarPanel(w, h int) string {
	now := m.now
	if now.IsZero() {
		now = time.Now()
	}
	dayLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.themeRoleColor("secondary", "#94e2d5"))).
		Render(now.Format("Monday"))
	dateLines := m.renderCalendarDateLines(max(1, w-4), max(1, h-2), now.Day())
	borderColor := blendHexColor(m.themeRoleColor("outline", "#565f89"), m.themeRoleColor("primary", "#89b4fa"), 0.40)
	return framedPanel(w, h, borderColor, strings.Join(dateLines, "\n"), dayLabel, "left", "", "right")
}

func (m model) renderMonthCalendarPanel(w, h int) string {
	now := m.now
	if now.IsZero() {
		now = time.Now()
	}
	monthYear := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.themeRoleColor("tertiary", "#cba6f7"))).
		Render(now.Format("January 2006"))
	lines := m.renderMonthCalendarLines(max(1, w-4), max(1, h-2), now)
	borderColor := blendHexColor(m.themeRoleColor("outline", "#565f89"), m.themeRoleColor("secondary", "#94e2d5"), 0.45)
	return framedPanel(w, h, borderColor, strings.Join(lines, "\n"), "", "center", monthYear, "right")
}

func (m model) renderMonthCalendarLines(width, height int, now time.Time) []string {
	if width < 1 || height < 1 {
		return []string{""}
	}
	loc := now.Location()
	year, month, today := now.Date()
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	startCol := int(first.Weekday()) // Sunday=0

	fgNormal := m.themeRoleColor("on_surface", "#cdd6f4")
	fgMuted := m.themeRoleColor("outline", "#6c7086")
	fgWeekend := blendHexColor(m.themeRoleColor("error", "#f38ba8"), fgNormal, 0.20)
	hlBg := m.themeRoleColor("primary", "#89b4fa")
	hlFg := ensureReadableTextColor(hlBg, m.themeRoleColor("on_primary", "#0b0f16"), m.themeRoleColor("on_surface", "#e6e2d5"))

	colW := make([]int, 7)
	base := width / 7
	if base < 2 {
		base = 2
	}
	rem := width - (base * 7)
	for i := 0; i < 7; i++ {
		colW[i] = base
		if rem > 0 {
			colW[i]++
			rem--
		}
	}

	weekday := []string{"S", "M", "T", "W", "T", "F", "S"}
	headerCells := make([]string, 0, 7)
	for i, lbl := range weekday {
		hColor := fgNormal
		if i == 5 || i == 6 {
			hColor = fgWeekend
		}
		headerCells = append(headerCells, lipgloss.NewStyle().
			Width(colW[i]).
			Align(lipgloss.Center).
			Bold(true).
			Foreground(lipgloss.Color(hColor)).
			Render(lbl))
	}

	weeks := make([]string, 0, 7)
	weeks = append(weeks, strings.Join(headerCells, ""))
	day := 1
	for week := 0; week < 6; week++ {
		cells := make([]string, 0, 7)
		for col := 0; col < 7; col++ {
			idx := week*7 + col
			if idx < startCol || day > daysInMonth {
				cells = append(cells, lipgloss.NewStyle().
					Width(colW[col]).
					Align(lipgloss.Center).
					Foreground(lipgloss.Color(fgMuted)).
					Render(""))
				continue
			}
			txt := fmt.Sprintf("%d", day)
			dColor := fgNormal
			if col == 5 || col == 6 {
				dColor = fgWeekend
			}
			if day == today {
				cells = append(cells, lipgloss.NewStyle().
					Width(colW[col]).
					Align(lipgloss.Center).
					Bold(true).
					Foreground(lipgloss.Color(hlFg)).
					Background(lipgloss.Color(hlBg)).
					Render(txt))
			} else {
				cells = append(cells, lipgloss.NewStyle().
					Width(colW[col]).
					Align(lipgloss.Center).
					Foreground(lipgloss.Color(dColor)).
					Render(txt))
			}
			day++
		}
		weeks = append(weeks, strings.Join(cells, ""))
		if day > daysInMonth {
			for len(weeks) < 7 {
				emptyCells := make([]string, 0, 7)
				for col := 0; col < 7; col++ {
					emptyCells = append(emptyCells, lipgloss.NewStyle().
						Width(colW[col]).
						Align(lipgloss.Center).
						Foreground(lipgloss.Color(fgMuted)).
						Render(""))
				}
				weeks = append(weeks, strings.Join(emptyCells, ""))
			}
			break
		}
	}
	if len(weeks) > height {
		return weeks[:height]
	}
	for len(weeks) < height {
		weeks = append(weeks, strings.Repeat(" ", width))
	}
	return weeks
}

func (m model) renderCalendarDateLines(width, height, day int) []string {
	if width < 1 || height < 1 {
		return []string{""}
	}
	glyph := m.calGlyphs[day]
	if len(glyph) == 0 {
		lines := []string{centerText(fmt.Sprintf("%02d", day), width)}
		return applyVerticalCenter(lines, height)
	}
	glyph = normalizeGlyphLines(glyph)
	gw := maxLineRunes(glyph)
	gh := len(glyph)
	startX := max(0, (width-gw)/2)
	startY := max(0, (height-gh)/2)
	if nudge := m.currentCalendarTopNudge(); nudge != 0 {
		startY += nudge
		maxStartY := max(0, height-gh)
		if startY > maxStartY {
			startY = maxStartY
		}
	}
	canvas := make([][]rune, height)
	for y := 0; y < height; y++ {
		canvas[y] = []rune(strings.Repeat(" ", width))
	}
	placeGlyphAligned(canvas, glyph, startX, startY, gw, "left")
	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		lines = append(lines, string(canvas[y]))
	}
	palette := boostPalette(m.clockPalette(), 0.18*introWeight(m.now, m.introUntil))
	shadow := m.themeRoleColor("on_surface", "#565f89")
	return applyClockPatternLinesStable(lines, palette, m.clockPhase, m.currentClockPattern(), m.themeRoleColor("error", "#f38ba8"), shadow)
}

func (m model) homeHintsLine(width int) string {
	muted := blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#000000", 0.32)
	keyTab := blendHexColor(muted, m.themeRoleColor("primary", "#89b4fa"), 0.35)
	keyFont := m.themeRoleColor("primary", "#89b4fa")
	keyAnim := m.themeRoleColor("secondary", "#94e2d5")
	keyPause := blendHexColor(m.themeRoleColor("secondary", "#94e2d5"), muted, 0.10)
	keyRedraw := blendHexColor(m.themeRoleColor("primary", "#89b4fa"), muted, 0.18)
	keyQuit := m.themeRoleColor("error", "#f38ba8")

	styleMuted := lipgloss.NewStyle().Foreground(lipgloss.Color(muted))
	tab := lipgloss.NewStyle().Foreground(lipgloss.Color(keyTab)).Render("tab/h/l")
	font := lipgloss.NewStyle().Foreground(lipgloss.Color(keyFont)).Render("f") + styleMuted.Render("ont")
	anim := lipgloss.NewStyle().Foreground(lipgloss.Color(keyAnim)).Render("a") + styleMuted.Render("nim")
	pause := ""
	if m.metricsPaused {
		pause = styleMuted.Render("un") + lipgloss.NewStyle().Foreground(lipgloss.Color(keyPause)).Render("p") + styleMuted.Render("ause")
	} else {
		pause = lipgloss.NewStyle().Foreground(lipgloss.Color(keyPause)).Render("p") + styleMuted.Render("ause")
	}
	redraw := lipgloss.NewStyle().Foreground(lipgloss.Color(keyRedraw)).Render("r") + styleMuted.Render("edraw")
	quit := lipgloss.NewStyle().Foreground(lipgloss.Color(keyQuit)).Render("q") + styleMuted.Render("uit")
	sp := styleMuted.Render("  ")
	line := tab + sp + font + sp + anim + sp + pause + sp + redraw + sp + quit
	return placeCenterStyled(line, width)
}

func (m model) homeNoticeLine(width int) string {
	if strings.TrimSpace(m.noticeText) == "" || m.noticeUntil.IsZero() || m.now.After(m.noticeUntil) {
		return ""
	}
	muted := m.themeRoleColor("on_surface", "#7f849c")
	accent := m.themeRoleColor("primary", "#89b4fa")
	if m.noticeKind == "anim" {
		accent = m.themeRoleColor("secondary", "#94e2d5")
	}
	card := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(accent)).
		Background(lipgloss.Color(blendHexColor(muted, "#000000", 0.65))).
		Padding(0, 2).
		Render(m.noticeText)
	cardW := lipgloss.Width(card)

	total := m.noticeUntil.Sub(m.noticeStart).Seconds()
	if total <= 0 {
		total = 1.0
	}
	elapsed := m.now.Sub(m.noticeStart).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > total {
		elapsed = total
	}
	p := elapsed / total
	slide := 0.0
	switch {
	case p < 0.10:
		// fast entrance
		slide = 1.0 - (p / 0.10)
	case p > 0.55:
		// slower exit
		slide = (p - 0.55) / 0.45
	default:
		slide = 0
	}
	offset := int(math.Round(float64(cardW+3) * slide))
	if offset < 0 {
		offset = 0
	}
	return strings.Repeat(" ", offset) + card
}

func placeCenterStyled(text string, width int) string {
	if width <= 0 {
		return text
	}
	w := lipgloss.Width(text)
	if w >= width {
		return text
	}
	left := (width - w) / 2
	right := width - w - left
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}

func placeCenterBlockStyled(text string, width int) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = placeCenterStyled(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

func forceBlockHeight(block string, h int, lineWidth int) string {
	if h < 1 {
		return block
	}
	lines := strings.Split(block, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	pad := strings.Repeat(" ", max(0, lineWidth))
	for len(lines) < h {
		lines = append(lines, pad)
	}
	return strings.Join(lines, "\n")
}

func equalizeBlockHeights(a, b string) (string, string) {
	hA := lipgloss.Height(a)
	hB := lipgloss.Height(b)
	target := max(hA, hB)
	a = forceBlockHeight(a, target, blockLineWidth(a))
	b = forceBlockHeight(b, target, blockLineWidth(b))
	for lipgloss.Height(a) < target {
		a += "\n" + strings.Repeat(" ", max(0, blockLineWidth(a)))
	}
	for lipgloss.Height(b) < target {
		b += "\n" + strings.Repeat(" ", max(0, blockLineWidth(b)))
	}
	return a, b
}

func blockLineWidth(block string) int {
	lines := strings.Split(block, "\n")
	w := 0
	for _, ln := range lines {
		if lw := lipgloss.Width(ln); lw > w {
			w = lw
		}
	}
	return w
}

func (m model) renderMain(usableW, contentH int) string {
	if m.page == pageHome {
		return m.renderHomePage(usableW, contentH)
	}
	return m.renderThemePage(usableW, contentH)
}

func (m model) renderThemePage(usableW, contentH int) string {
	usableW = max(28, usableW)
	contentH = max(10, contentH)

	mainH := contentH
	overlayH := 0
	if m.hasActiveOverlay() {
		overlayH = max(6, m.interactionLineCount()+2)
		if mainH-overlayH < 8 {
			overlayH = max(4, mainH-8)
		}
		mainH = max(8, contentH-overlayH)
	}

	mainBody := m.settingsCombinedBlock(mainH-2, usableW-4)
	mainRow := panelStyle(usableW, mainH, "12").Render(mainBody)
	footer := m.themeHintsLine(usableW)
	if overlayH <= 0 {
		return mainRow + "\n" + footer
	}
	overlayBody := m.interactionBlock(overlayH - 2)
	overlayRow := panelStyle(usableW, overlayH, "8").Render(overlayBody)
	return mainRow + "\n" + overlayRow + "\n" + footer
}

func (m model) shouldShowWallpaperSegment() bool {
	return m.width >= 76 && m.height >= 32
}

func (m model) themeHintsLine(width int) string {
	hintColor := ensureReadableTextColor(
		m.themeRoleColor("background", "#11131c"),
		m.themeRoleColor("text_muted", blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), m.themeRoleColor("background", "#11131c"), 0.28)),
		"#b3bcc8",
	)
	text := strings.Join([]string{
		hotkeyLabel("Update Colors", "u", "4"),
		hotkeyLabel("Apply", "A", "1"),
		hotkeyLabel("Backups", "b", "8"),
		hotkeyLabel("Hints", "?", "13"),
	}, "   ")
	line := lipgloss.NewStyle().Foreground(lipgloss.Color(hintColor)).Render(text)
	return placeCenterStyled(line, width)
}

func (m model) interactionLineCount() int {
	if m.pickerTarget != "" {
		return 10
	}
	if m.customizing {
		return 10
	}
	if m.showApplyConfirm {
		return 8
	}
	if m.showHints {
		return 6
	}
	if m.showBackups {
		return 10
	}
	return 4
}

func (m model) settingsBlock(limit, width int) string {
	lines := []string{headerChip("Colors", "12")}
	items := m.settings()
	selected := m.settingIndex
	if selected < 0 || selected >= len(items) {
		selected = 0
	}
	visible := max(1, (max(1, limit-3)+1)/2)
	start, end := listWindow(len(items), selected, visible)
	rows := make([]string, 0, (end-start)*2)
	for i := start; i < end; i++ {
		label, state, kind, toggle := m.settingsRowView(items[i].Target)
		menuRows := []string(nil)
		showMenuAbove := false
		if m.settingMenuTarget == items[i].Target {
			menuRows = m.renderInlineSettingMenu(items[i].Target, width)
			showMenuAbove = m.shouldRenderInlineMenuAbove(i, start, end, len(menuRows))
		}
		if showMenuAbove {
			rows = append(rows, menuRows...)
		}
		rows = append(rows, m.renderSettingsMenuRow(label, state, kind, items[i].Target, toggle, i == m.settingIndex, width))
		if m.settingMenuTarget == items[i].Target && !showMenuAbove {
			rows = append(rows, menuRows...)
		}
		if i < end-1 {
			rows = append(rows, "")
		}
	}
	lines = append(lines, m.renderSettingsGroupPanel(rows, width)...)
	return strings.Join(lines, "\n")
}

func (m model) settingsPageBlock(limit, width int) string {
	lines := []string{headerChip("Misc", "8")}
	items := m.settingsPageItems()
	selected := m.settingIndex - len(m.settings())
	if selected < 0 {
		selected = 0
	}
	visible := max(1, (max(1, limit-3)+1)/2)
	start, end := listWindow(len(items), selected, visible)
	rows := make([]string, 0, (end-start)*2)
	for i := start; i < end; i++ {
		label, state, kind, toggle := m.settingsRowView(items[i].Target)
		menuRows := []string(nil)
		showMenuAbove := false
		if m.settingMenuTarget == items[i].Target {
			menuRows = m.renderInlineSettingMenu(items[i].Target, width)
			showMenuAbove = m.shouldRenderInlineMenuAbove(i, start, end, len(menuRows))
		}
		if showMenuAbove {
			rows = append(rows, menuRows...)
		}
		rows = append(rows, m.renderSettingsMenuRow(label, state, kind, items[i].Target, toggle, len(m.settings())+i == m.settingIndex, width))
		if m.settingMenuTarget == items[i].Target && !showMenuAbove {
			rows = append(rows, menuRows...)
		}
		if i < end-1 {
			rows = append(rows, "")
		}
	}
	lines = append(lines, m.renderSettingsGroupPanel(rows, width)...)
	return strings.Join(lines, "\n")
}

func (m model) shouldRenderInlineMenuAbove(index, start, end, menuLineCount int) bool {
	if menuLineCount <= 0 {
		return false
	}
	itemsBelow := end - index - 1
	if itemsBelow <= 0 {
		return true
	}
	availableBelow := (itemsBelow * 2) - 1
	return availableBelow < menuLineCount
}

func settingsSectionLines(itemCount int) int {
	if itemCount <= 0 {
		return 4
	}
	// header line + bordered panel (rows + top/bottom border)
	// rows are rendered as: row, blank, row, blank... => (2*n - 1)
	return 1 + ((2 * itemCount) - 1) + 2
}

func (m model) settingsCombinedBlock(limit, width int) string {
	if limit < 6 {
		return strings.Join([]string{
			headerChip("Settings", "12"),
			"  (expand terminal)",
		}, "\n")
	}
	minMisc := 4
	fullColor := settingsSectionLines(len(m.settings()))
	colorLimit := fullColor
	if colorLimit > limit-minMisc {
		colorLimit = max(3, limit-minMisc)
	}
	statusLimit := max(minMisc, limit-colorLimit)
	lines := []string{}
	lines = append(lines, strings.Split(m.settingsBlock(colorLimit, width), "\n")...)
	lines = append(lines, strings.Split(m.settingsPageBlock(statusLimit, width), "\n")...)
	for len(lines) < limit {
		lines = append(lines, "")
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func (m model) renderInlineSettingMenu(target string, width int) []string {
	choices := m.settingMenuChoices(target)
	if len(choices) == 0 {
		return nil
	}
	menuW := max(14, width-8)
	lines := []string{}
	if target == "segments" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("    choose: ↑/↓   space: toggle   enter/esc: close"))
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("    choose: ↑/↓ then Enter"))
	}
	for i, choice := range choices {
		prefix := "    "
		style := lipgloss.NewStyle().
			Width(menuW).
			Foreground(lipgloss.Color(m.themeRoleColor("on_surface", "#d4d8e5")))
		if i == m.settingMenuIndex {
			bg := m.themeRoleColor("primary", "#b18cff")
			fg := ensureReadableTextColor(bg, m.themeRoleColor("background", "#0e1016"), "#111111")
			prefix = "    ▶ "
			style = style.
				Bold(true).
				Background(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fg))
		} else if target != "segments" && choice.Value == m.currentSettingChoice(target) {
			style = style.Bold(true).Foreground(lipgloss.Color(m.themeRoleColor("secondary", "#94e2d5")))
		}
		label := choice.Label
		if target == "segments" {
			mark := "x "
			if m.segmentEnabled(choice.Value) {
				mark = "✓ "
			}
			label = mark + label
		}
		lines = append(lines, style.Render(prefix+label))
	}
	return lines
}

func (m model) paletteBlock(limit, width int) string {
	lines := []string{headerChip("Palette", "13"), ""}
	lines = append(lines, m.palettePreviewLines(width)...)
	for len(lines) < limit {
		lines = append(lines, "")
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func (m model) wallpaperBlock(limit, width int) string {
	lines := []string{headerChip("Wallpaper", "12"), ""}
	innerWidth := max(8, width-4)
	imageRows := max(3, limit-len(lines))
	framePad := 1
	imageWidth := max(6, innerWidth-(framePad*2))
	rendered := renderCachedImageFile(preferredWallpaperPath(), imageWidth, imageRows)
	if strings.TrimSpace(rendered) == "" {
		lines = append(lines, strings.Repeat(" ", framePad+2)+"wallpaper preview")
		lines = append(lines, strings.Repeat(" ", framePad+2)+"unavailable")
	} else {
		for _, row := range strings.Split(rendered, "\n") {
			lines = append(lines, strings.Repeat(" ", framePad)+row+strings.Repeat(" ", framePad))
		}
	}
	for len(lines) < limit {
		lines = append(lines, "")
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func (m model) compactPaletteWallpaperBlock(limit, width int) string {
	lines := []string{headerChip("Palette", "13"), ""}
	lines = append(lines, m.compactPaletteGridLines(width)...)
	if len(lines) < limit {
		lines = append(lines, "")
	}
	if len(lines) < limit {
		lines = append(lines, headerChip("Wallpaper", "12"), "")
	}
	remaining := limit - len(lines)
	if remaining > 0 {
		framePad := 1
		rendered := renderCachedImageFile(preferredWallpaperPath(), max(8, width-4-(framePad*2)), max(3, remaining))
		if strings.TrimSpace(rendered) == "" {
			lines = append(lines, strings.Repeat(" ", framePad+2)+"wallpaper preview")
			if len(lines) < limit {
				lines = append(lines, strings.Repeat(" ", framePad+2)+"unavailable")
			}
		} else {
			for _, row := range strings.Split(rendered, "\n") {
				lines = append(lines, strings.Repeat(" ", framePad)+row+strings.Repeat(" ", framePad))
			}
		}
	}
	for len(lines) < limit {
		lines = append(lines, "")
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func (m model) compactPaletteGridLines(width int) []string {
	lines := m.palettePreviewLines(width)
	if len(lines) > 2 {
		return lines[:2]
	}
	return lines
}

func (m model) renderSettingsGroupPanel(rows []string, width int) []string {
	if len(rows) == 0 {
		rows = []string{" "}
	}
	borderColor := blendHexColor(m.themeRoleColor("outline", "#565f89"), m.themeRoleColor("background", "#11131c"), 0.55)
	panel := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(max(16, width-2)).
		Render(strings.Join(rows, "\n"))
	return strings.Split(panel, "\n")
}

func (m *model) openSettingMenu(target string) {
	choices := m.settingMenuChoices(target)
	if len(choices) == 0 {
		return
	}
	m.settingMenuTarget = target
	m.settingMenuIndex = 0
	current := m.currentSettingChoice(target)
	for i, choice := range choices {
		if choice.Value == current {
			m.settingMenuIndex = i
			break
		}
	}
}

func (m model) currentSettingChoice(target string) string {
	switch target {
	case "theme_source":
		return m.themeSource
	case "status_theme":
		return m.statusTheme
	case "mode":
		return canonicalMode(m.mode)
	case "profile":
		return canonicalProfile(m.profile)
	case "palette_type":
		return canonicalPaletteType(m.paletteType)
	case "preset_family":
		return m.presetFamily
	case "preset_variant":
		return m.presetVariant
	case "segments":
		return ""
	case "starship_prompt":
		return strings.TrimSpace(strings.ToLower(m.starshipPrompt))
	default:
		return ""
	}
}

func (m model) settingMenuChoices(target string) []settingChoice {
	switch target {
	case "theme_source":
		return []settingChoice{
			{Value: "wallpaper", Label: "Wallpaper"},
			{Value: "preset", Label: "Preset"},
		}
	case "status_theme":
		return []settingChoice{
			{Value: "default", Label: "Default"},
			{Value: "rounded", Label: "Rounded"},
			{Value: "rectangle", Label: "Rectangle"},
		}
	case "mode":
		return []settingChoice{
			{Value: "dark", Label: "Dark"},
			{Value: "light", Label: "Light"},
		}
	case "profile":
		out := make([]settingChoice, 0, len(profilePresets))
		for _, p := range profilePresets {
			out = append(out, settingChoice{Value: p, Label: m.extractChoiceLabel(p)})
		}
		return out
	case "palette_type":
		out := make([]settingChoice, 0, len(paletteTypePresets))
		for _, p := range paletteTypePresets {
			out = append(out, settingChoice{Value: p, Label: displayPaletteType(p)})
		}
		return out
	case "preset_family":
		out := make([]settingChoice, 0, len(presetFamilyOrder))
		for _, family := range presetFamilyOrder {
			out = append(out, settingChoice{Value: family, Label: displayPresetFamily(family)})
		}
		return out
	case "preset_variant":
		variants := presetVariantsByFamily[m.presetFamily]
		out := make([]settingChoice, 0, len(variants))
		for _, variant := range variants {
			out = append(out, settingChoice{Value: variant, Label: displayPresetVariant(variant)})
		}
		return out
	case "segments":
		return []settingChoice{
			{Value: "widget_battery", Label: "Battery"},
			{Value: "widget_cpu", Label: "CPU"},
			{Value: "widget_ram", Label: "RAM"},
			{Value: "widget_weather", Label: "Weather"},
		}
	case "starship_prompt":
		if !m.starshipConfigEnabled() {
			return nil
		}
		out := make([]settingChoice, 0, len(starshipPromptPresets))
		for _, sp := range starshipPromptPresets {
			out = append(out, settingChoice{Value: sp, Label: displayStarshipPrompt(sp)})
		}
		return out
	default:
		return nil
	}
}

func (m *model) applySettingChoice(target, value string) {
	switch target {
	case "theme_source":
		if contains(themeSources, value) {
			m.themeSource = value
		} else {
			m.themeSource = defaultSource
		}
		m.clampMergedSettingIndex()
	case "status_theme":
		m.statusTheme = value
		m.normalizeThemeSelection()
	case "mode":
		m.mode = canonicalMode(value)
	case "profile":
		m.profile = value
		m.normalizeThemeSelection()
	case "palette_type":
		m.paletteType = value
		m.normalizeThemeSelection()
	case "preset_family":
		m.presetFamily = value
		m.normalizeThemeSelection()
	case "preset_variant":
		m.presetVariant = value
		m.normalizeThemeSelection()
	case "segments":
		m.toggleSegment(value)
	case "starship_prompt":
		v := strings.TrimSpace(strings.ToLower(value))
		if contains(starshipPromptPresets, v) {
			m.starshipPrompt = v
		} else {
			m.starshipPrompt = defaultStarship
		}
		settings, ok := loadTooieSettings()
		if !ok {
			settings = defaultTooieSettings()
		}
		settings.Starship.Prompt = m.starshipPrompt
		_ = saveTooieSettings(settings)
	}
}

func settingMenuLabel(target string) string {
	switch target {
	case "theme_source":
		return "source"
	case "status_theme":
		return "Status Theme"
	case "mode":
		return "Mode"
	case "profile":
		return "Primary"
	case "palette_type":
		return "Palette"
	case "preset_family":
		return "Preset"
	case "preset_variant":
		return "Preset Variant"
	case "segments":
		return "tmux status segments"
	case "starship_prompt":
		return "Starship"
	default:
		return "Options"
	}
}

func (m model) settingsRowView(target string) (label, state, kind string, toggle bool) {
	switch target {
	case "theme_source":
		return hotkeyLabel("source", "s", "6"), displayThemeSource(m.themeSource) + " ▾", "info", false
	case "mode":
		return hotkeyLabel("Mode", "m", "5"), displayMode(m.mode) + " ▾", "info", false
	case "profile":
		return hotkeyLabel("Primary", "f", "2"), m.extractStateLabel(), "swatch", false
	case "palette_type":
		return hotkeyLabel("Palette", "p", "13"), displayPaletteType(m.paletteType) + " ▾", "info", false
	case "preset_family":
		return "Preset", displayPresetFamily(m.presetFamily) + " ▾", "info", false
	case "preset_variant":
		return "Preset Variant", displayPresetVariant(m.presetVariant) + " ▾", "info", false
	case "customize":
		return "Customize Colors", "Open", "action", false
	case "status_theme":
		return hotkeyLabel("tmux status", "t", "3"), displayStatusTheme(m.statusTheme) + " ▾", "info", false
	case "segments":
		return "tmux status segments", m.segmentSummary() + " ▾", "info", false
	case "starship_prompt":
		if !m.starshipConfigEnabled() {
			return hotkeyLabel("Starship", "S", "10"), "N/A", "info", false
		}
		return hotkeyLabel("Starship", "S", "10"), displayStarshipPrompt(m.starshipPrompt) + " ▾", "info", false
	default:
		return target, "", "info", false
	}
}

func (m model) starshipConfigEnabled() bool {
	settings, ok := loadTooieSettings()
	if !ok {
		return true
	}
	return normalizeStarshipInstallMode(settings.Modules.StarshipMode) != "off"
}

func (m model) renderSettingsMenuRow(label, state, kind, target string, toggle bool, selected bool, width int) string {
	width = max(24, width-4)
	labelW := (width * 3) / 5
	if labelW < 16 {
		labelW = 16
	}
	if labelW > width-12 {
		labelW = width - 12
	}
	stateW := max(10, width-labelW-2)

	labelFG := ensureReadableTextColor(
		m.themeRoleColor("background", "#11131c"),
		m.themeRoleColor("text_primary", m.themeRoleColor("on_surface", "#e6e2d5")),
		"#f1f1f3",
	)
	prefix := "  "
	if selected {
		prefix = "▶ "
	}
	labelBody := lipgloss.NewStyle().
		Width(max(1, labelW)).
		Foreground(lipgloss.Color(labelFG)).
		Bold(selected).
		Render(prefix + label)

	stateCell := ""
	if kind == "toggle" {
		stateCell = m.renderToggleStateCell(target, toggle, stateW, selected)
	} else if kind == "swatch" {
		stateCell = m.renderSwatchStateCell(state, stateW, selected)
	} else {
		stateCell = m.renderInfoStateCell(state, kind, stateW, selected)
	}
	return labelBody + "  " + stateCell
}

func (m model) renderSwatchStateCell(state string, width int, selected bool) string {
	width = max(10, width)
	if strings.TrimSpace(state) == "" {
		state = renderSwatchChip("")
	}
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		Bold(selected).
		Render(state)
}

func (m model) renderInfoStateCell(state, kind string, width int, selected bool) string {
	width = max(8, width)
	state = cutPad(strings.TrimSpace(state), max(1, width-2))
	bg := blendHexColor(m.themeRoleColor("surface_container", "#1f2335"), m.themeRoleColor("background", "#11131c"), 0.30)
	fgPreferred := m.themeRoleColor("text_primary", m.themeRoleColor("on_surface", "#e6e2d5"))
	switch kind {
	case "action":
		bg = m.themeRoleColor("text_action_bg", blendHexColor(m.themeRoleColor("tertiary", "#cba6f7"), m.themeRoleColor("background", "#11131c"), 0.70))
		fgPreferred = m.themeRoleColor("text_action_fg", fgPreferred)
	case "info":
		bg = m.themeRoleColor("text_info_bg", blendHexColor(m.themeRoleColor("surface_variant", "#2f3650"), m.themeRoleColor("background", "#11131c"), 0.68))
		fgPreferred = m.themeRoleColor("text_info_fg", fgPreferred)
	}
	if selected {
		bg = blendHexColor(bg, m.themeRoleColor("primary", "#89b4fa"), 0.12)
	}
	fg := ensureReadableTextColor(bg, fgPreferred, "#f6f7fb")
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg)).
		Bold(selected || kind == "action").
		Render(state)
}

func (m model) renderToggleStateCell(target string, on bool, width int, selected bool) string {
	width = max(13, width)
	_, animating := m.toggleMixForTarget(target, on)
	yesLabel := " yes "
	noLabel := " no "
	activeStops := []string{
		m.themeRoleColor("primary", "#ff4fa3"),
		m.themeRoleColor("tertiary", "#b96cff"),
	}
	if !on {
		activeStops = []string{
			m.themeRoleColor("secondary", "#8f967f"),
			blendHexColor(m.themeRoleColor("secondary", "#8f967f"), m.themeRoleColor("background", "#11131c"), 0.35),
		}
	}
	yesActiveBG := gradientFromStops(0.25, activeStops)
	noActiveBG := gradientFromStops(0.75, activeStops)
	if selected || animating {
		yesActiveBG = blendHexColor(yesActiveBG, "#ffffff", 0.10)
		noActiveBG = blendHexColor(noActiveBG, "#ffffff", 0.10)
	}
	inactiveBG := blendHexColor(m.themeRoleColor("surface_container", "#2f3138"), m.themeRoleColor("background", "#11131c"), 0.22)
	activeFGYes := ensureReadableTextColor(yesActiveBG, m.themeRoleColor("on_primary", "#111111"), "#f7f7fb")
	activeFGNo := ensureReadableTextColor(noActiveBG, m.themeRoleColor("on_primary", "#111111"), "#f7f7fb")
	inactiveFG := ensureReadableTextColor(inactiveBG, m.themeRoleColor("on_surface", "#a7adbb"), "#c9cfda")
	yesStyle := lipgloss.NewStyle().Bold(true)
	noStyle := lipgloss.NewStyle().Bold(true)
	if on {
		yesStyle = yesStyle.Background(lipgloss.Color(yesActiveBG)).Foreground(lipgloss.Color(activeFGYes))
		noStyle = noStyle.Background(lipgloss.Color(inactiveBG)).Foreground(lipgloss.Color(inactiveFG))
	} else {
		yesStyle = yesStyle.Background(lipgloss.Color(inactiveBG)).Foreground(lipgloss.Color(inactiveFG))
		noStyle = noStyle.Background(lipgloss.Color(noActiveBG)).Foreground(lipgloss.Color(activeFGNo))
	}
	text := yesStyle.Render(yesLabel) + " " + noStyle.Render(noLabel)
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(text)
}

func (m model) renderGradientToggleSegment(text string, emphasize bool) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	stops := []string{
		m.themeRoleColor("primary", "#89b4fa"),
		m.themeRoleColor("secondary", "#94e2d5"),
		m.themeRoleColor("tertiary", "#cba6f7"),
	}
	out := strings.Builder{}
	for i, r := range runes {
		t := 0.0
		if len(runes) > 1 {
			t = float64(i) / float64(len(runes)-1)
		}
		bg := gradientFromStops(t, stops)
		if emphasize {
			bg = blendHexColor(bg, "#ffffff", 0.08)
		}
		fg := ensureReadableTextColor(bg, m.themeRoleColor("on_primary", "#0b0f16"), "#f7f7fb")
		out.WriteString(lipgloss.NewStyle().
			Background(lipgloss.Color(bg)).
			Foreground(lipgloss.Color(fg)).
			Bold(true).
			Render(string(r)))
	}
	return out.String()
}

func (m model) toggleMixForTarget(target string, on bool) (float64, bool) {
	if strings.TrimSpace(target) != "" && target == m.switchAnimTarget && m.switchAnimProg >= 0 && m.switchAnimProg < 1 {
		p := m.switchAnimProg
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		p = 0.5 - (0.5 * math.Cos(math.Pi*p))
		if m.switchAnimFrom == m.switchAnimTo {
			if m.switchAnimTo {
				return 1, true
			}
			return 0, true
		}
		if m.switchAnimFrom {
			return 1 - p, true
		}
		return p, true
	}
	if on {
		return 1, false
	}
	return 0, false
}

func (m model) renderHomePage(usableW, contentH int) string {
	usableW = max(28, usableW)
	contentH = max(8, contentH)
	topH := contentH
	metricW, metricH := clockGlyphMetrics(m.clockGlyphs)
	clockMinH := desiredClockPanelHeight(metricH)
	switch m.currentClockFontName() {
	case "fivebyfive", "squaresounds":
		clockMinH = max(6, clockMinH-3)
	}
	topH = max(topH, clockMinH)

	leftW := desiredClockPanelWidth(usableW, topH, metricW, metricH)
	if leftW < 18 {
		leftW = 18
	}
	rightW := usableW - leftW
	if rightW < 38 {
		rightW = 38
		leftW = usableW - rightW
	}

	rowH := max(6, topH)
	clockLines := m.renderDashboardVerticalClockTest(max(1, leftW-4), max(1, rowH-2))
	clockBorder := blendHexColor(m.themeRoleColor("primary", "#89b4fa"), m.themeRoleColor("outline", "#565f89"), 0.35)
	sysBorder := blendHexColor(m.themeRoleColor("secondary", "#94e2d5"), m.themeRoleColor("outline", "#565f89"), 0.30)
	clockPanel := framedPanel(leftW, rowH, clockBorder, strings.Join(clockLines, "\n"), "", "left", m.clockMeridiemLabel(), "right")
	sysPanel := framedPanel(rightW, rowH, sysBorder, m.homeSystemBlock(rightW-4, rowH-2), m.systemPanelTitle(), "left", "", "left")
	return lipgloss.JoinHorizontal(lipgloss.Top, clockPanel, sysPanel)
}

func (m model) homeSystemBlock(innerW, limit int) string {
	if innerW < 28 {
		innerW = 28
	}

	rows := m.systemInfoRows()
	labelW := 8
	fixedLines := 9 + 5
	infoSlots := len(rows)
	if limit > 0 {
		infoSlots = max(0, min(len(rows), limit-fixedLines))
	}
	shown := make([]systemInfoRow, 0, infoSlots)
	for priority := 0; priority <= 4 && len(shown) < infoSlots; priority++ {
		for _, row := range rows {
			if row.Priority == priority {
				shown = append(shown, row)
				if len(shown) >= infoSlots {
					break
				}
			}
		}
	}

	lines := []string{""}
	for _, row := range shown {
		lines = append(lines, m.renderSystemInfoRow(innerW, row, labelW))
	}
	lines = append(lines, "")
	lines = append(lines, m.renderSystemMetric(
		innerW,
		"",
		"CPU",
		fmt.Sprintf("%d%%", int(clampPct(m.cpuFiltered)+0.5)),
		m.cpuViz,
		m.cpuBarGradientColor,
	)...)
	lines = append(lines, m.renderSystemMetric(
		innerW,
		"",
		"RAM",
		m.systemMetricSummary(m.ramUsedGB, m.ramTotalGB, m.ramFiltered),
		m.ramViz,
		m.ramBarGradientColor,
	)...)
	lines = append(lines, m.renderSystemMetric(
		innerW,
		"󰋊",
		"Storage",
		m.systemMetricSummary(m.storUsedGB, m.storTotalGB, m.storFiltered),
		m.storViz,
		m.storageBarGradientColor,
	)...)
	lines = append(lines, "")
	lines = append(lines, m.systemInfoFooter(innerW))

	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func (m model) canSwitchPage() bool {
	return m.pickerTarget == "" && m.settingMenuTarget == "" && !m.customizing && !m.showBackups && !m.showApplyConfirm
}

func (m model) pageLabel() string {
	if m.page == pageHome {
		return "Tooie"
	}
	return "Theme"
}

func (m model) interactionBorderColor() string {
	if m.pickerTarget != "" {
		return "13"
	}
	if m.settingMenuTarget != "" {
		return "12"
	}
	if m.showApplyConfirm {
		return "10"
	}
	if m.customizing {
		return "12"
	}
	if m.showHints || m.showBackups {
		return "11"
	}
	return "8"
}

func (m model) interactionBlock(limit int) string {
	if m.pickerTarget != "" {
		return m.colorPickerBlock(limit)
	}
	if m.settingMenuTarget != "" {
		choices := m.settingMenuChoices(m.settingMenuTarget)
		lines := []string{
			headerChip("Choose "+settingMenuLabel(m.settingMenuTarget), "12"),
			"  up/down: move",
			"  enter: select   esc: close",
		}
		start, end := listWindow(len(choices), m.settingMenuIndex, max(1, limit-3))
		for i := start; i < end; i++ {
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == m.settingMenuIndex {
				prefix = "▶ "
				style = style.Bold(true)
			}
			line := prefix + choices[i].Label
			if choices[i].Value == m.currentSettingChoice(m.settingMenuTarget) {
				line += "  (current)"
			}
			lines = append(lines, style.Render(line))
		}
		return strings.Join(lines, "\n")
	}
	if m.customizing {
		return m.customizeBlock(limit)
	}
	if m.showHints {
		return strings.Join([]string{
			headerChip("Hints", "13"),
			"  up/down or j/k: move",
			"  enter: select action",
			"  u: Update colors",
			"  A: Apply",
			"  b: Backups",
			"  q: quit",
			"  esc or ?: close",
		}, "\n")
	}
	if m.showApplyConfirm {
		yes := "  Yes"
		cancel := "  Cancel"
		if m.applyConfirmIdx == 0 {
			yes = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render("▶ Yes")
		} else {
			cancel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("▶ Cancel")
		}
		return strings.Join([]string{
			headerChip("Confirm Apply", "10"),
			"",
			"  Apply updated theme now?",
			"",
			yes,
			cancel,
			"",
			"  enter/a: confirm   esc: cancel   Shift+A: apply now",
		}, "\n")
	}
	if m.showBackups {
		lines := []string{headerChip("Backups", "8"), "  enter=restore, esc=close"}
		if len(m.backups) == 0 {
			lines = append(lines, "  (none)")
		} else {
			visible := max(1, limit-2)
			start, end := listWindow(len(m.backups), m.backupIndex, visible)
			for i := start; i < end; i++ {
				b := m.backups[i]
				prefix := "  "
				if i == m.backupIndex {
					prefix = "▶ "
				}
				line := prefix + b.ID
				if v, ok := b.Meta["theme_source"]; ok && v != "" {
					line += " <" + v + ">"
				}
				if v, ok := b.Meta["preset_family"]; ok && v != "" {
					line += " [" + v
					if vv, ok := b.Meta["preset_variant"]; ok && vv != "" {
						line += ":" + vv
					}
					line += "]"
				}
				if v, ok := b.Meta["status_palette"]; ok && v != "" {
					line += " {" + v + "}"
				}
				if b.Meta["theme_source"] != "preset" {
					if v, ok := b.Meta["style_family"]; ok && v != "" {
						line += " (" + displayProfile(canonicalProfile(v)) + ")"
					} else if v, ok := b.Meta["profile"]; ok && v != "" {
						line += " (" + displayProfile(canonicalProfile(v)) + ")"
					} else if v, ok := b.Meta["style_preset"]; ok && v != "" {
						line += " (" + displayProfile(canonicalProfile(v)) + ")"
					}
				}
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func (m model) palettePreviewLines(width int) []string {
	terminal := m.terminalPalette16()
	if len(terminal) == 0 {
		return []string{"  (no generated palette in selected backup)"}
	}
	cellW := 2
	cell := func(hex string) string {
		return lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render(strings.Repeat(" ", cellW))
	}
	palette := make([]string, 0, 64)
	palette = append(palette, terminal...)
	for _, hex := range m.previewPaletteColors() {
		if !internaltheme.IsHexColor(hex) {
			continue
		}
		palette = append(palette, strings.ToLower(strings.TrimSpace(hex)))
	}
	if len(palette) == 0 {
		return []string{"  (no generated palette in selected backup)"}
	}
	cols := max(12, (max(16, width)-2)/cellW)
	lines := make([]string, 0, (len(palette)+cols-1)/cols)
	for i := 0; i < len(palette); i += cols {
		end := min(len(palette), i+cols)
		var row strings.Builder
		row.WriteString("  ")
		for _, hex := range palette[i:end] {
			row.WriteString(cell(hex))
		}
		lines = append(lines, row.String())
	}
	return lines
}

func (m model) hasActiveOverlay() bool {
	return m.showHints || m.showBackups || m.pickerTarget != "" || m.customizing || m.showApplyConfirm
}

func (m model) previewPaletteColors() []string {
	order := []string{
		"background", "surface", "on_surface", "outline",
		"primary", "primary_container", "secondary", "secondary_container",
		"tertiary", "tertiary_container", "error", "error_container",
	}
	out := []string{}
	seen := map[string]bool{}
	appendHex := func(raw string) {
		hex := strings.ToLower(strings.TrimSpace(raw))
		if !internaltheme.IsHexColor(hex) || seen[hex] {
			return
		}
		seen[hex] = true
		out = append(out, hex)
	}
	for _, key := range order {
		appendHex(m.selectedHexes[key])
	}
	keys := make([]string, 0, len(m.selectedHexes))
	for k := range m.selectedHexes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		appendHex(m.selectedHexes[key])
	}
	for _, raw := range []string{m.ansiRed, m.ansiGreen, m.ansiYellow, m.ansiBlue, m.ansiMagenta, m.ansiCyan} {
		appendHex(raw)
	}
	return out
}

func (m model) terminalPalette16() []string {
	seed := m.previewPaletteColors()
	if len(seed) == 0 {
		return nil
	}
	pick := func(candidates ...string) string {
		for _, c := range candidates {
			hex := strings.ToLower(strings.TrimSpace(c))
			if internaltheme.IsHexColor(hex) {
				return hex
			}
		}
		return ""
	}
	term := make([]string, 16)
	term[0] = pick(m.selectedHexes["surface"], m.selectedHexes["background"], seed[0])
	term[1] = pick(m.ansiRed, m.selectedHexes["error"], blendHexColor(term[0], "#ff5f5f", 0.75))
	term[2] = pick(m.ansiGreen, m.selectedHexes["secondary"], blendHexColor(term[0], "#5fff87", 0.75))
	term[3] = pick(m.ansiYellow, m.selectedHexes["tertiary"], blendHexColor(term[0], "#ffd75f", 0.75))
	term[4] = pick(m.ansiBlue, m.selectedHexes["primary"], blendHexColor(term[0], "#5f87ff", 0.75))
	term[5] = pick(m.ansiMagenta, m.selectedHexes["tertiary_container"], blendHexColor(term[0], "#d787ff", 0.75))
	term[6] = pick(m.ansiCyan, m.selectedHexes["secondary_container"], blendHexColor(term[0], "#5fd7ff", 0.75))
	term[7] = pick(m.selectedHexes["on_surface"], "#d7d7d7")
	term[8] = pick(m.selectedHexes["outline"], blendHexColor(term[0], term[7], 0.35))
	for i := 9; i < 16; i++ {
		base := term[i-8]
		if base == "" {
			base = seed[(i-9)%len(seed)]
		}
		term[i] = pick(blendHexColor(base, "#ffffff", 0.22), base)
	}
	for i, hex := range term {
		if internaltheme.IsHexColor(hex) {
			continue
		}
		term[i] = seed[i%len(seed)]
	}
	return term
}

func (m model) paletteRoleAccentLines(width int) []string {
	type roleAccent struct {
		role  string
		label string
	}
	order := []roleAccent{
		{role: "primary", label: "Primary"},
		{role: "secondary", label: "Secondary"},
		{role: "tertiary", label: "Tertiary"},
		{role: "error", label: "Error"},
		{role: "surface", label: "Surface"},
		{role: "on_surface", label: "On Surface"},
	}
	parts := make([]string, 0, len(order))
	for _, item := range order {
		hex := strings.ToLower(strings.TrimSpace(m.selectedHexes[item.role]))
		if !internaltheme.IsHexColor(hex) {
			continue
		}
		fg := ensureReadableTextColor(hex, "#101216", "#f3f5f8")
		parts = append(parts, lipgloss.NewStyle().
			Foreground(lipgloss.Color(fg)).
			Background(lipgloss.Color(hex)).
			Padding(0, 1).
			Render(item.label))
	}
	if len(parts) == 0 {
		return []string{"  (role accents unavailable)"}
	}
	return wrapPaletteParts(parts, max(18, width-2))
}

func wrapPaletteParts(parts []string, maxWidth int) []string {
	if len(parts) == 0 {
		return nil
	}
	lines := []string{}
	cur := "  "
	for _, p := range parts {
		if lipgloss.Width(cur)+lipgloss.Width(p)+1 > maxWidth && strings.TrimSpace(cur) != "" {
			lines = append(lines, cur)
			cur = "  " + p
			continue
		}
		if strings.TrimSpace(cur) == "" {
			cur = "  " + p
		} else {
			cur += " " + p
		}
	}
	if strings.TrimSpace(cur) != "" {
		lines = append(lines, cur)
	}
	return lines
}

func hotkeyLabel(label, key, color string) string {
	lowerLabel := strings.ToLower(label)
	lowerKey := strings.ToLower(key)
	idx := strings.Index(lowerLabel, lowerKey)
	if idx < 0 {
		return label
	}
	runes := []rune(label)
	start := len([]rune(label[:idx]))
	if start < 0 || start >= len(runes) {
		return label
	}
	hot := string(runes[start])
	hotStyled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(hot)
	return string(runes[:start]) + hotStyled + string(runes[start+1:])
}

func headerChip(text, color string) string {
	bg := terminalColorHex(color)
	fg := ensureReadableTextColor(bg, "#111111", "#f5f5f8")
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Render(text)
}

func (m model) colorPickerOptions(target string) []colorOption {
	opts := []colorOption{{Label: "Auto", Hex: ""}}
	if strings.HasPrefix(target, "ansi_") {
		return append(opts, m.ansiColorOptions(target)...)
	}
	order := []string{"primary", "secondary", "tertiary", "error", "on_surface", "surface", "outline", "inverse_primary", "primary_container", "secondary_container", "tertiary_container", "error_container"}
	seen := map[string]bool{}
	for _, key := range order {
		hex := strings.ToLower(strings.TrimSpace(m.selectedHexes[key]))
		if hex == "" || seen[hex] {
			continue
		}
		seen[hex] = true
		opts = append(opts, colorOption{
			Label: roleLabel(key),
			Hex:   hex,
		})
	}
	return opts
}

func (m model) ansiColorOptions(target string) []colorOption {
	type pair struct {
		role  string
		label string
	}
	priority := map[string][]pair{
		"ansi_red": {
			{"error", "error"},
			{"error_container", "error bg"},
			{"primary", "accent"},
			{"tertiary", "accent alt"},
		},
		"ansi_green": {
			{"secondary", "secondary"},
			{"secondary_container", "secondary bg"},
			{"primary", "accent"},
			{"tertiary", "accent alt"},
		},
		"ansi_yellow": {
			{"tertiary", "tertiary"},
			{"tertiary_container", "tertiary bg"},
			{"secondary", "secondary"},
			{"primary", "primary"},
		},
		"ansi_blue": {
			{"primary", "primary"},
			{"inverse_primary", "inverse primary"},
			{"secondary", "secondary"},
			{"tertiary", "tertiary"},
		},
		"ansi_magenta": {
			{"tertiary", "tertiary"},
			{"error", "error"},
			{"primary", "primary"},
			{"secondary", "secondary"},
		},
		"ansi_cyan": {
			{"secondary", "secondary"},
			{"secondary_container", "secondary bg"},
			{"primary", "primary"},
			{"inverse_primary", "inverse primary"},
		},
	}

	bg := strings.ToLower(strings.TrimSpace(m.selectedHexes["background"]))
	if bg == "" {
		bg = "#11131c"
	}
	seen := map[string]bool{}
	out := make([]colorOption, 0, 16)
	add := func(label, hex string) {
		hex = strings.ToLower(strings.TrimSpace(hex))
		if hex == "" || seen[hex] {
			return
		}
		if contrastRatioHex(hex, bg) < 2.4 {
			return
		}
		seen[hex] = true
		out = append(out, colorOption{Label: label, Hex: hex})
	}

	for _, item := range priority[target] {
		if hex := strings.TrimSpace(m.selectedHexes[item.role]); hex != "" {
			add(item.label, hex)
		}
	}
	// Build channel-specific derived tones so picker stays meaningful even when extracted roles are sparse.
	for _, item := range priority[target] {
		base := strings.TrimSpace(m.selectedHexes[item.role])
		if base == "" {
			continue
		}
		add(item.label+" vivid", blendHexColor(base, "#ffffff", 0.12))
		add(item.label+" deep", blendHexColor(base, "#000000", 0.16))
	}

	family := strings.TrimPrefix(target, "ansi_")
	for _, opt := range m.familyColorOptions(family) {
		add(opt.Label, opt.Hex)
	}
	return out
}

func (m model) colorOptionIndexForHex(target, hex string) int {
	hex = strings.TrimSpace(strings.ToLower(hex))
	opts := m.colorPickerOptions(target)
	if hex == "" {
		return 0
	}
	for i, opt := range opts {
		if strings.ToLower(opt.Hex) == hex {
			return i
		}
	}
	return 0
}

func (m model) colorLabelByHex(hex string) string {
	hex = strings.TrimSpace(strings.ToLower(hex))
	if hex == "" {
		return "auto"
	}
	opts := m.colorPickerOptions("text")
	for _, opt := range opts {
		if strings.ToLower(opt.Hex) == hex {
			return opt.Label
		}
	}
	return hex
}

func (m model) colorPickerBlock(limit int) string {
	targetLabel := colorTargetLabel(m.pickerTarget)
	lines := []string{headerChip("Choose "+targetLabel, "13")}
	opts := m.colorPickerOptions(m.pickerTarget)
	visible := max(1, limit-3)
	start, end := listWindow(len(opts), m.pickerIndex, visible)
	for i := start; i < end; i++ {
		opt := opts[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.pickerIndex {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("11"))
		}
		label := "auto"
		if opt.Hex != "" {
			swatch := lipgloss.NewStyle().Background(lipgloss.Color(opt.Hex)).Render("  ")
			label = swatch + " " + opt.Label
		}
		lines = append(lines, style.Render(prefix+label))
	}
	lines = append(lines, "  up/down to choose", "  enter=apply, esc=cancel")
	return strings.Join(lines, "\n")
}

func colorTargetLabel(target string) string {
	switch target {
	case "text":
		return "Text Color"
	case "cursor":
		return "Cursor Color"
	case "ansi_red":
		return "ANSI Red"
	case "ansi_green":
		return "ANSI Green"
	case "ansi_yellow":
		return "ANSI Yellow"
	case "ansi_blue":
		return "ANSI Blue"
	case "ansi_magenta":
		return "ANSI Magenta"
	case "ansi_cyan":
		return "ANSI Cyan"
	default:
		return "Color"
	}
}

func roleLabel(role string) string {
	switch role {
	case "background":
		return "bg"
	case "on_background":
		return "fg"
	case "surface":
		return "panel"
	case "surface_dim":
		return "panel dim"
	case "surface_bright":
		return "panel bright"
	case "surface_container":
		return "panel base"
	case "surface_container_high":
		return "panel raised"
	case "surface_variant":
		return "panel alt"
	case "on_surface":
		return "text"
	case "on_surface_variant":
		return "muted text"
	case "outline":
		return "border"
	case "outline_variant":
		return "border soft"
	case "primary":
		return "accent primary"
	case "secondary":
		return "accent secondary"
	case "tertiary":
		return "accent tertiary"
	case "error":
		return "accent error"
	case "inverse_primary":
		return "accent inverse"
	case "primary_container":
		return "accent primary bg"
	case "secondary_container":
		return "accent secondary bg"
	case "tertiary_container":
		return "accent tertiary bg"
	case "error_container":
		return "accent error bg"
	default:
		return strings.ReplaceAll(role, "_", " ")
	}
}

func (m model) familyColorOptions(family string) []colorOption {
	type c struct {
		label string
		hex   string
		h     float64
		v     float64
	}
	candidates := make([]c, 0, len(m.selectedHexes))
	seen := map[string]bool{}
	for role, hex := range m.selectedHexes {
		hex = strings.ToLower(strings.TrimSpace(hex))
		if hex == "" || seen[hex] {
			continue
		}
		seen[hex] = true
		r, g, b, ok := parseHexRGB(hex)
		if !ok {
			continue
		}
		h, s, v := rgbToHSV(r, g, b)
		if s < 0.14 {
			continue
		}
		if !matchesFamilyHue(family, h) {
			continue
		}
		candidates = append(candidates, c{
			label: roleLabel(role),
			hex:   hex,
			h:     h,
			v:     v,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].h-candidates[j].h) > 0.001 {
			return candidates[i].h < candidates[j].h
		}
		return candidates[i].v > candidates[j].v
	})
	out := make([]colorOption, 0, len(candidates))
	for _, item := range candidates {
		out = append(out, colorOption{
			Label: item.label,
			Hex:   item.hex,
		})
	}
	return out
}

func matchesFamilyHue(family string, h float64) bool {
	switch family {
	case "red":
		return h >= 345 || h < 20
	case "yellow":
		return h >= 35 && h < 75
	case "green":
		return h >= 75 && h < 165
	case "cyan":
		return h >= 165 && h < 210
	case "blue":
		return h >= 210 && h < 280
	case "magenta":
		return h >= 280 && h < 345
	default:
		return false
	}
}

func parseHexRGB(hex string) (float64, float64, float64, bool) {
	if len(hex) != 7 || hex[0] != '#' {
		return 0, 0, 0, false
	}
	hexToByte := func(s string) (float64, bool) {
		var v uint8
		for i := 0; i < len(s); i++ {
			ch := s[i]
			v <<= 4
			switch {
			case ch >= '0' && ch <= '9':
				v |= ch - '0'
			case ch >= 'a' && ch <= 'f':
				v |= ch - 'a' + 10
			case ch >= 'A' && ch <= 'F':
				v |= ch - 'A' + 10
			default:
				return 0, false
			}
		}
		return float64(v) / 255.0, true
	}
	r, ok := hexToByte(hex[1:3])
	if !ok {
		return 0, 0, 0, false
	}
	g, ok := hexToByte(hex[3:5])
	if !ok {
		return 0, 0, 0, false
	}
	b, ok := hexToByte(hex[5:7])
	if !ok {
		return 0, 0, 0, false
	}
	return r, g, b, true
}

func rgbToHSV(r, g, b float64) (float64, float64, float64) {
	maxv := math.Max(r, math.Max(g, b))
	minv := math.Min(r, math.Min(g, b))
	delta := maxv - minv
	h := 0.0
	switch {
	case delta == 0:
		h = 0
	case maxv == r:
		h = 60 * math.Mod(((g-b)/delta), 6)
	case maxv == g:
		h = 60 * (((b - r) / delta) + 2)
	default:
		h = 60 * (((r - g) / delta) + 4)
	}
	if h < 0 {
		h += 360
	}
	s := 0.0
	if maxv != 0 {
		s = delta / maxv
	}
	v := maxv
	return h, s, v
}

func (m model) customizeItems() []customizeItem {
	return []customizeItem{
		{Label: "Text Color", Target: "text"},
		{Label: "Cursor Color", Target: "cursor"},
		{Label: "ANSI Red", Target: "ansi_red"},
		{Label: "ANSI Green", Target: "ansi_green"},
		{Label: "ANSI Yellow", Target: "ansi_yellow"},
		{Label: "ANSI Blue", Target: "ansi_blue"},
		{Label: "ANSI Magenta", Target: "ansi_magenta"},
		{Label: "ANSI Cyan", Target: "ansi_cyan"},
		{Label: "Status Palette", Target: "status_palette"},
		{Label: "Back", Target: "back"},
	}
}

func (m model) customizeBlock(limit int) string {
	items := m.customizeItems()
	lines := []string{headerChip("Customize Colors", "12"), "  choose item and press enter"}
	visible := max(1, limit-3)
	start, end := listWindow(len(items), m.customIndex, visible)
	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.customIndex {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("11"))
		}
		value := ""
		switch item.Target {
		case "status_palette":
			value = m.palette
		case "back":
			value = ""
		default:
			hex := m.getColorTarget(item.Target)
			if strings.TrimSpace(hex) == "" {
				value = "auto"
			} else {
				value = lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render("  ")
			}
		}
		label := item.Label
		if value != "" {
			label += ": " + value
		}
		lines = append(lines, style.Render(prefix+label))
	}
	lines = append(lines, "  esc=close")
	return strings.Join(lines, "\n")
}

func listWindow(total, selected, visible int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if visible < 1 {
		visible = 1
	}
	if total <= visible {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}
	start := selected - visible/2
	if start < 0 {
		start = 0
	}
	maxStart := total - visible
	if start > maxStart {
		start = maxStart
	}
	return start, start + visible
}

func (m model) activateCustomizeItem() (tea.Model, tea.Cmd) {
	if m.applying {
		return m, nil
	}
	items := m.customizeItems()
	if m.customIndex < 0 || m.customIndex >= len(items) {
		return m, nil
	}
	target := items[m.customIndex].Target
	switch target {
	case "status_palette":
		if m.palette == "default" {
			m.palette = "vibrant"
		} else {
			m.palette = "default"
		}
		return m, nil
	case "back":
		m.customizing = false
		m.customIndex = 0
		return m, nil
	default:
		m.pickerTarget = target
		m.pickerIndex = m.colorOptionIndexForHex(target, m.getColorTarget(target))
		return m, nil
	}
}

func (m model) getColorTarget(target string) string {
	switch target {
	case "text":
		return m.textColor
	case "cursor":
		return m.cursorColor
	case "ansi_red":
		return m.ansiRed
	case "ansi_green":
		return m.ansiGreen
	case "ansi_yellow":
		return m.ansiYellow
	case "ansi_blue":
		return m.ansiBlue
	case "ansi_magenta":
		return m.ansiMagenta
	case "ansi_cyan":
		return m.ansiCyan
	default:
		return ""
	}
}

func (m *model) setColorTarget(target, hex string) {
	switch target {
	case "text":
		m.textColor = hex
	case "cursor":
		m.cursorColor = hex
	case "ansi_red":
		m.ansiRed = hex
	case "ansi_green":
		m.ansiGreen = hex
	case "ansi_yellow":
		m.ansiYellow = hex
	case "ansi_blue":
		m.ansiBlue = hex
	case "ansi_magenta":
		m.ansiMagenta = hex
	case "ansi_cyan":
		m.ansiCyan = hex
	}
}

func (m model) applyArgs(includeOverrides bool) []string {
	statusTheme := normalizeStatusTheme(m.statusTheme)
	if statusTheme == "" {
		statusTheme = defaultStatusTheme
	}
	args := []string{"--theme-source", m.themeSource, "--status-palette", m.palette, "--status-theme", statusTheme}
	args = append(args, "--starship-prompt", normalizeStarshipPrompt(m.starshipPrompt))
	setupCfg, _ := loadTooieSettings()
	args = append(args,
		"--status-position", normalizeStatusPosition(setupCfg.Tmux.StatusPosition),
		"--status-layout", normalizeStatusLayout(setupCfg.Tmux.StatusLayout),
		"--status-separator", normalizeSeparatorMode(setupCfg.Tmux.StatusSeparator),
	)
	if m.themeSource == "preset" {
		args = append(args, "--preset-family", m.presetFamily, "--preset-variant", m.presetVariant)
	} else {
		paletteType := canonicalPaletteType(m.paletteType)
		if !contains(paletteTypePresets, paletteType) {
			paletteType = defaultPaletteType
		}
		args = append(args, "-m", m.mode, "--style-family", m.profile, "--type", paletteType)
	}
	if includeOverrides {
		if strings.TrimSpace(m.textColor) != "" {
			args = append(args, "--text-color", strings.TrimSpace(m.textColor))
		}
		if strings.TrimSpace(m.cursorColor) != "" {
			args = append(args, "--cursor-color", strings.TrimSpace(m.cursorColor))
		}
		if strings.TrimSpace(m.ansiRed) != "" {
			args = append(args, "--ansi-red", strings.TrimSpace(m.ansiRed))
		}
		if strings.TrimSpace(m.ansiGreen) != "" {
			args = append(args, "--ansi-green", strings.TrimSpace(m.ansiGreen))
		}
		if strings.TrimSpace(m.ansiYellow) != "" {
			args = append(args, "--ansi-yellow", strings.TrimSpace(m.ansiYellow))
		}
		if strings.TrimSpace(m.ansiBlue) != "" {
			args = append(args, "--ansi-blue", strings.TrimSpace(m.ansiBlue))
		}
		if strings.TrimSpace(m.ansiMagenta) != "" {
			args = append(args, "--ansi-magenta", strings.TrimSpace(m.ansiMagenta))
		}
		if strings.TrimSpace(m.ansiCyan) != "" {
			args = append(args, "--ansi-cyan", strings.TrimSpace(m.ansiCyan))
		}
	}
	args = append(args,
		"--widget-battery", onOffFlag(m.widgetBattery),
		"--widget-cpu", onOffFlag(m.widgetCPU),
		"--widget-ram", onOffFlag(m.widgetRAM),
		"--widget-weather", onOffFlag(m.widgetWeather),
	)
	return args
}

func parseBackupID(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "Backup created: ") || strings.HasPrefix(ln, "Preview created: ") {
			path := strings.TrimSpace(strings.SplitN(ln, ":", 2)[1])
			if path != "" {
				return filepath.Base(path)
			}
		}
	}
	return ""
}

func findBackupIndexByID(backups []backup, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return -1
	}
	for i, b := range backups {
		if strings.TrimSpace(b.ID) == id {
			return i
		}
	}
	return -1
}

func resolveTooieApplyRunner() (string, []string) {
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe, []string{"theme", "apply"}
	}
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) != "" {
		bin := filepath.Join(home, ".local", "bin", "tooie")
		if info, err := os.Stat(bin); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return bin, []string{"theme", "apply"}
		}
	}
	if lp, err := exec.LookPath("tooie"); err == nil && strings.TrimSpace(lp) != "" {
		return lp, []string{"theme", "apply"}
	}
	return "", nil
}

func runApplyCommand(args []string, label, cacheKey, reuseBackup string, previewOnly bool) tea.Cmd {
	return func() tea.Msg {
		runner, prefix := resolveTooieApplyRunner()
		if strings.TrimSpace(runner) == "" {
			return applyDoneMsg{
				label:       label,
				err:         fmt.Errorf("tooie binary not found for theme apply"),
				out:         "",
				cacheKey:    cacheKey,
				previewOnly: previewOnly,
			}
		}
		if previewOnly {
			args = append(args, "--preview-only")
		}
		if strings.TrimSpace(reuseBackup) != "" {
			args = append(args, "--reuse-backup", strings.TrimSpace(reuseBackup))
		}
		cmdArgs := append(append([]string{}, prefix...), args...)
		cmd := exec.Command(runner, cmdArgs...)
		cmd.Env = append(os.Environ(), "TOOIE_APPLY_PROGRESS_FILE="+applyProgressPath())
		out, err := cmd.CombinedOutput()
		outText := strings.TrimSpace(string(out))
		return applyDoneMsg{
			label:       label,
			err:         err,
			out:         outText,
			backupID:    parseBackupID(outText),
			cacheKey:    cacheKey,
			reused:      strings.TrimSpace(reuseBackup) != "",
			previewOnly: previewOnly,
		}
	}
}

func (m model) applyCacheSignature() string {
	parts := m.applyArgs(true)
	parts = append(parts, "cache_schema="+themeCacheSchema)
	if m.themeSource == "wallpaper" {
		parts = append(parts, "wallpaper_fingerprint="+wallpaperCacheFingerprint())
	}
	return strings.Join(parts, "\x1f")
}

func wallpaperCacheFingerprint() string {
	wall := preferredWallpaperPath()
	if st, err := os.Stat(wall); err == nil {
		return fmt.Sprintf("fixed:%s:%d:%d", wall, st.ModTime().UnixNano(), st.Size())
	}
	bgDir := filepath.Dir(defaultWall)
	entries, err := os.ReadDir(bgDir)
	if err != nil {
		return "none"
	}
	type fileEntry struct {
		name    string
		modTime time.Time
		size    int64
	}
	var newest *fileEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		item := fileEntry{name: e.Name(), modTime: info.ModTime(), size: info.Size()}
		if newest == nil || item.modTime.After(newest.modTime) || (item.modTime.Equal(newest.modTime) && item.name > newest.name) {
			cp := item
			newest = &cp
		}
	}
	if newest == nil {
		return "none"
	}
	return fmt.Sprintf("latest:%s:%d:%d", newest.name, newest.modTime.UnixNano(), newest.size)
}

func preferredWallpaperPath() string {
	if wall, ok := bestWallpaperPath(homeDir); ok {
		return wall
	}
	return defaultWall
}

func (m model) startApply(label string, includeOverrides bool, previewOnly bool) (tea.Model, tea.Cmd) {
	if m.applying {
		return m, nil
	}
	_ = os.Remove(applyProgressPath())
	args := m.applyArgs(includeOverrides)
	cacheKey := m.applyCacheSignature()
	reuseBackup := ""
	if !previewOnly && cacheKey == m.previewCacheKey && strings.TrimSpace(m.previewBackupID) != "" {
		reuseBackup = m.previewBackupID
	}
	m.applying = true
	m.applyLabel = label
	m.applyCacheKey = cacheKey
	m.applyProgress = 0
	m.applyVel = 0
	m.applyTarget = 0.02
	m.lastStatus = label + " in progress..."
	return m, tea.Batch(tickApply(), runApplyCommand(args, label, cacheKey, reuseBackup, previewOnly))
}

func (m model) requestThemeApply() (tea.Model, tea.Cmd) {
	cacheKey := m.applyCacheSignature()
	if cacheKey == m.lastAppliedTheme {
		if !(cacheKey == m.previewCacheKey && strings.TrimSpace(m.previewBackupID) != "") {
			m.lastStatus = "No theme changes to apply"
			return m, nil
		}
	}
	return m.startApply(m.themeActionLabel(false), true, false)
}

func (m *model) refreshCurrentPreviewColors() {
	cfg, err := parseThemeApplyFlags(m.applyArgs(true))
	if err != nil {
		return
	}
	payload, _, err := computeThemePayload(cfg, "")
	if err != nil {
		return
	}
	m.selectedHexes = map[string]string{}
	for role, hex := range payload.Roles {
		hex = strings.ToLower(strings.TrimSpace(hex))
		if hex == "" {
			continue
		}
		m.selectedHexes[role] = hex
	}
	if strings.TrimSpace(payload.Background) != "" {
		m.selectedHexes["background"] = strings.ToLower(strings.TrimSpace(payload.Background))
	}
	if strings.TrimSpace(payload.Foreground) != "" {
		m.selectedHexes["on_surface"] = strings.ToLower(strings.TrimSpace(payload.Foreground))
	}
}

func nextProfile(cur string) string {
	if len(profilePresets) == 0 {
		return cur
	}
	for i, p := range profilePresets {
		if p == cur {
			return profilePresets[(i+1)%len(profilePresets)]
		}
	}
	return profilePresets[0]
}

func nextPaletteType(cur string) string {
	if len(paletteTypePresets) == 0 {
		return cur
	}
	cur = canonicalPaletteType(cur)
	for i, p := range paletteTypePresets {
		if p == cur {
			return paletteTypePresets[(i+1)%len(paletteTypePresets)]
		}
	}
	return paletteTypePresets[0]
}

func nextMode(cur string) string {
	if len(modePresets) == 0 {
		return cur
	}
	cur = canonicalMode(cur)
	for i, mode := range modePresets {
		if mode == cur {
			return modePresets[(i+1)%len(modePresets)]
		}
	}
	return modePresets[0]
}

func (m model) themeActionLabel(previewOnly bool) string {
	if m.themeSource == "preset" {
		if previewOnly {
			return "Preview preset"
		}
		return "Apply preset"
	}
	if previewOnly {
		return "Update colors"
	}
	return "Apply theme"
}

func nextPageIndex(cur int) int {
	n := cur + 1
	if n >= totalPages {
		n = 0
	}
	return n
}

func prevPageIndex(cur int) int {
	n := cur - 1
	if n < 0 {
		n = totalPages - 1
	}
	return n
}

func nextThemeSource(cur string) string {
	if len(themeSources) == 0 {
		return cur
	}
	for i, src := range themeSources {
		if src == cur {
			return themeSources[(i+1)%len(themeSources)]
		}
	}
	return themeSources[0]
}

func nextStatusTheme(cur string) string {
	if len(statusThemePresets) == 0 {
		return cur
	}
	for i, theme := range statusThemePresets {
		if theme == cur {
			return statusThemePresets[(i+1)%len(statusThemePresets)]
		}
	}
	return statusThemePresets[0]
}

func nextPresetFamily(cur string) string {
	if len(presetFamilyOrder) == 0 {
		return cur
	}
	for i, family := range presetFamilyOrder {
		if family == cur {
			return presetFamilyOrder[(i+1)%len(presetFamilyOrder)]
		}
	}
	return presetFamilyOrder[0]
}

func nextPresetVariant(family, cur string) string {
	variants := presetVariantsByFamily[family]
	if len(variants) == 0 {
		return cur
	}
	for i, variant := range variants {
		if variant == cur {
			return variants[(i+1)%len(variants)]
		}
	}
	return variants[0]
}

func displayProfile(name string) string {
	switch canonicalProfile(name) {
	case "auto":
		return "Dominant (Idx 0)"
	case "source-0":
		return "Source Index 0"
	case "source-1":
		return "Source Index 1"
	case "source-2":
		return "Source Index 2"
	case "source-3":
		return "Source Index 3"
	case "source-4":
		return "Source Index 4"
	case "prefer-saturation":
		return "Prefer Saturation"
	case "prefer-lightness":
		return "Prefer Lightness"
	case "prefer-darkness":
		return "Prefer Darkness"
	case "prefer-less-saturation":
		return "Prefer Less Saturation"
	case "prefer-value":
		return "Prefer Value"
	case "prefer-closest-fallback":
		return "Prefer Closest Fallback"
	default:
		name = strings.TrimSpace(name)
		if name == "" || name == "default" {
			return "Dominant (Idx 0)"
		}
		return strings.ToUpper(name[:1]) + name[1:]
	}
}

func (m model) extractColor(profile string) string {
	key := canonicalProfile(profile)
	if m.extractSwatches != nil {
		if c := strings.TrimSpace(m.extractSwatches[key]); internaltheme.IsHexColor(c) {
			return normalizeHexColor(c)
		}
	}
	return ""
}

func renderSwatchChip(hex string) string {
	hex = normalizeHexColor(strings.TrimSpace(hex))
	if !internaltheme.IsHexColor(hex) {
		hex = "#5a606f"
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(hex)).
		Width(7).
		Render("       ")
}

func (m model) extractChoiceLabel(profile string) string {
	if c := m.extractColor(profile); c != "" {
		return renderSwatchChip(c)
	}
	if m.extractLoading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("loading")
	}
	return renderSwatchChip("")
}

func (m model) extractStateLabel() string {
	if c := m.extractColor(m.profile); c != "" {
		return renderSwatchChip(c) + " ▾"
	}
	if m.extractLoading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("loading") + " ▾"
	}
	return renderSwatchChip("") + " ▾"
}

func displayMode(mode string) string {
	switch canonicalMode(mode) {
	case "dark":
		return "Dark"
	case "light":
		return "Light"
	default:
		return strings.TrimSpace(mode)
	}
}

func displayThemeSource(source string) string {
	switch strings.TrimSpace(source) {
	case "preset":
		return "Preset"
	default:
		return "Wallpaper"
	}
}

func displayPaletteType(name string) string {
	switch canonicalPaletteType(name) {
	case "tonal-spot":
		return "Tonal Spot"
	case "expressive":
		return "Expressive"
	case "fidelity":
		return "Fidelity"
	case "content":
		return "Content"
	case "vibrant":
		return "Vibrant"
	case "neutral":
		return "Neutral"
	case "rainbow":
		return "Rainbow"
	case "fruit-salad":
		return "Fruit Salad"
	default:
		name = strings.TrimSpace(name)
		if name == "" {
			return "Tonal Spot"
		}
		parts := strings.Split(name, "-")
		for i := range parts {
			if parts[i] == "" {
				continue
			}
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
		return strings.Join(parts, " ")
	}
}

func displayStarshipPrompt(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "", "gruvbox":
		return "Fancy"
	case "pure":
		return "Pure"
	case "jetpack":
		return "Jetpack"
	default:
		parts := strings.Split(strings.TrimSpace(name), "-")
		for i := range parts {
			if parts[i] == "" {
				continue
			}
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
		return strings.Join(parts, " ")
	}
}

func normalizeStarshipPrompt(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	if contains(starshipPromptPresets, v) {
		return v
	}
	return defaultStarship
}

func displayStatusTheme(name string) string {
	switch normalizeStatusTheme(name) {
	case "rounded":
		return "Rounded"
	case "rectangle":
		return "Rectangle"
	default:
		return "Default"
	}
}

func displayPresetFamily(family string) string {
	switch strings.TrimSpace(family) {
	case "catppuccin":
		return "Catppuccin"
	case "rose-pine":
		return "Rose Pine"
	case "tokyo-night":
		return "Tokyo Night"
	case "synthwave-84":
		return "Synthwave 84"
	case "dracula":
		return "Dracula"
	case "gruvbox":
		return "Gruvbox"
	case "nord":
		return "Nord"
	default:
		return displayProfile(family)
	}
}

func displayPresetVariant(variant string) string {
	switch strings.TrimSpace(variant) {
	case "default":
		return "Default"
	default:
		return displayProfile(variant)
	}
}

func presetVariantMode(family, variant string) string {
	switch family + ":" + variant {
	case "catppuccin:latte", "rose-pine:dawn", "tokyo-night:day", "gruvbox:light":
		return "light"
	default:
		return "dark"
	}
}

type threeColumnSpec struct {
	leftW  int
	midW   int
	rightW int
	sep    string
	sepW   int
}

func threeColumnLayout(totalWidth int) (threeColumnSpec, bool) {
	if totalWidth < 72 {
		return threeColumnSpec{}, false
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" │ ")
	sepW := lipgloss.Width(sep)
	leftW := (totalWidth - 2*sepW) / 3
	midW := leftW
	rightW := totalWidth - 2*sepW - leftW - midW
	if leftW < 18 || midW < 18 || rightW < 18 {
		return threeColumnSpec{}, false
	}
	return threeColumnSpec{leftW: leftW, midW: midW, rightW: rightW, sep: sep, sepW: sepW}, true
}

func renderThreeColumns(left, middle, right []string, totalWidth int) string {
	spec, ok := threeColumnLayout(totalWidth)
	if !ok {
		joined := make([]string, 0, len(left)+len(middle)+len(right)+4)
		joined = append(joined, left...)
		joined = append(joined, "")
		joined = append(joined, middle...)
		joined = append(joined, "")
		joined = append(joined, right...)
		return strings.Join(joined, "\n")
	}

	rowCount := max(len(left), max(len(middle), len(right)))
	lines := make([]string, 0, rowCount)
	leftStyle := lipgloss.NewStyle().Width(spec.leftW)
	middleStyle := lipgloss.NewStyle().Width(spec.midW)
	rightStyle := lipgloss.NewStyle().Width(spec.rightW)
	for i := 0; i < rowCount; i++ {
		l, m, r := "", "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(middle) {
			m = middle[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, leftStyle.Render(l)+spec.sep+middleStyle.Render(m)+spec.sep+rightStyle.Render(r))
	}
	return strings.Join(lines, "\n")
}

func renderTwoColumns(left, right []string, totalWidth int) string {
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" │ ")
	sepW := lipgloss.Width(sep)
	if totalWidth < 2*18+sepW {
		joined := make([]string, 0, len(left)+len(right)+2)
		joined = append(joined, left...)
		joined = append(joined, "")
		joined = append(joined, right...)
		return strings.Join(joined, "\n")
	}
	avail := totalWidth - sepW
	leftW := (avail * 12) / 20
	if leftW < 24 {
		leftW = 24
	}
	if leftW > avail-20 {
		leftW = avail - 20
	}
	rightW := avail - leftW
	rowCount := max(len(left), len(right))
	lines := make([]string, 0, rowCount)
	leftStyle := lipgloss.NewStyle().Width(leftW)
	rightStyle := lipgloss.NewStyle().Width(rightW)
	for i := 0; i < rowCount; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, leftStyle.Render(l)+sep+rightStyle.Render(r))
	}
	return strings.Join(lines, "\n")
}

func twoColumnWidths(totalWidth int) (int, int) {
	sepW := lipgloss.Width(lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" │ "))
	if totalWidth < 2*18+sepW {
		return 0, 0
	}
	avail := totalWidth - sepW
	leftW := (avail * 12) / 20
	if leftW < 24 {
		leftW = 24
	}
	if leftW > avail-20 {
		leftW = avail - 20
	}
	rightW := avail - leftW
	return leftW, rightW
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func (m *model) cycleClockFont() {
	if len(m.clockFontDefs) == 0 {
		return
	}
	m.clockFontIdx = (m.clockFontIdx + 1) % len(m.clockFontDefs)
	m.clockGlyphs = loadClockGlyphSet(m.clockFontDefs, m.clockFontIdx)
	if m.clockFontIdx >= 0 && m.clockFontIdx < len(m.clockFontDefs) {
		m.showHomeNotice("font: "+m.clockFontDefs[m.clockFontIdx].Name, "font")
	}
	m.persistClockSettings()
}

func (m *model) cycleClockPattern() {
	if len(m.clockPatterns) == 0 {
		return
	}
	m.patternIndex = (m.patternIndex + 1) % len(m.clockPatterns)
	m.showHomeNotice("anim: "+m.currentClockPattern(), "anim")
	m.persistClockSettings()
}

func (m *model) cycleCalendarFont() {
	if len(m.calFontDefs) == 0 {
		return
	}
	m.calFontIdx = (m.calFontIdx + 1) % len(m.calFontDefs)
	m.calGlyphs = loadCalendarGlyphSet(m.calFontDefs, m.calFontIdx)
	if m.calFontIdx >= 0 && m.calFontIdx < len(m.calFontDefs) {
		m.showHomeNotice("date font: "+m.calFontDefs[m.calFontIdx].Name, "font")
	}
	m.persistClockSettings()
}

func (m model) currentClockPattern() string {
	if len(m.clockPatterns) == 0 {
		return "wave"
	}
	idx := m.patternIndex % len(m.clockPatterns)
	if idx < 0 {
		idx = 0
	}
	return m.clockPatterns[idx]
}

func (m model) currentClockFontName() string {
	if m.clockFontIdx < 0 || m.clockFontIdx >= len(m.clockFontDefs) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(m.clockFontDefs[m.clockFontIdx].Name))
}

func (m model) currentFontClockLayout() fontClockLayout {
	name := m.currentClockFontName()
	switch name {
	case "mousetrap":
		return fontClockLayout{gapYMin: 3, innerPadY: 1, topNudgeY: 0, bottomNudge: 1}
	case "pixelzone":
		return fontClockLayout{gapYMin: 2, innerPadY: 1, topNudgeY: 0, bottomNudge: 1}
	case "retropixelthick":
		return fontClockLayout{gapYMin: 3, innerPadY: 1, topNudgeY: 0, bottomNudge: 1}
	case "squaresounds", "edges", "fivebyfive":
		return fontClockLayout{gapYMin: 1, innerPadY: 0, topNudgeY: 0, bottomNudge: 0}
	default:
		return fontClockLayout{gapYMin: 1, innerPadY: 0, topNudgeY: 0, bottomNudge: 0}
	}
}

func (m model) currentCalendarFontName() string {
	if m.calFontIdx < 0 || m.calFontIdx >= len(m.calFontDefs) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(m.calFontDefs[m.calFontIdx].Name))
}

func (m model) currentCalendarTopNudge() int {
	switch m.currentCalendarFontName() {
	case "cal-ember":
		return 1
	default:
		return 0
	}
}

func (m *model) showHomeNotice(text, kind string) {
	now := m.now
	if now.IsZero() {
		now = time.Now()
		if m.clockLoc != nil {
			now = now.In(m.clockLoc)
		}
	}
	m.noticeText = strings.TrimSpace(text)
	m.noticeKind = kind
	m.noticeStart = now
	m.noticeUntil = now.Add(5 * time.Second)
}

func (m *model) renderDashboardVerticalClockTest(width, height int) []string {
	if width < 1 || height < 1 {
		return []string{""}
	}
	now := m.now
	if now.IsZero() {
		now = time.Now()
	}
	hh := now.Format("03")
	mm := now.Format("04")
	glyphs := m.clockGlyphs
	if len(glyphs) == 0 {
		lines := []string{
			centerText(hh, width),
			centerText(":", width),
			centerText(mm, width),
		}
		return applyVerticalCenter(lines, height)
	}
	if m.clockOnly {
		d0 := normalizeGlyphLines(renderDashboardDigitGlyph(rune(hh[0]), glyphs, width, height))
		d1 := normalizeGlyphLines(renderDashboardDigitGlyph(rune(hh[1]), glyphs, width, height))
		d2 := normalizeGlyphLines(renderDashboardDigitGlyph(rune(mm[0]), glyphs, width, height))
		d3 := normalizeGlyphLines(renderDashboardDigitGlyph(rune(mm[1]), glyphs, width, height))
		fixedColW, _ := clockGlyphMetricsNormalized(glyphs)

		lines := renderClockOnlyGlyphGrid(width, height, d0, d1, d2, d3, fixedColW)
		palette := boostPalette(m.clockPalette(), 0.18*introWeight(now, m.introUntil))
		shadow := m.themeRoleColor("on_surface", "#565f89")
		return applyClockPatternLinesStable(lines, palette, m.clockPhase, m.currentClockPattern(), m.themeRoleColor("primary", "#7aa2f7"), shadow)
	}

	outerPad := max(1, min(width, height)/20)
	drawW := width - (outerPad * 2)
	drawH := height - (outerPad * 2)
	if drawW < 4 {
		drawW = width
		outerPad = 0
	}
	if drawH < 4 {
		drawH = height
		outerPad = 0
	}

	_, fontH := clockGlyphMetrics(glyphs)
	fontLayout := m.currentFontClockLayout()
	gapX := max(2, drawW/16)
	gapY := max(1, drawH/18)
	gapY = max(gapY, fontH/3)
	gapY = max(gapY, fontLayout.gapYMin)
	if gapX > drawW-2 {
		gapX = max(1, drawW/4)
	}
	if gapY > drawH-2 {
		gapY = 1
	}
	leftW := max(1, (drawW-gapX)/2)
	rightW := max(1, drawW-gapX-leftW)
	topH := max(1, (drawH-gapY)/2)
	botH := max(1, drawH-gapY-topH)

	innerPadX := max(1, min(leftW, rightW)/10)
	innerPadY := max(0, gapY/3)
	innerPadY = max(innerPadY, fontLayout.innerPadY)

	d0 := renderDashboardDigitGlyph(rune(hh[0]), glyphs, leftW-innerPadX, topH-innerPadY)
	d1 := renderDashboardDigitGlyph(rune(hh[1]), glyphs, rightW-innerPadX, topH-innerPadY)
	d2 := renderDashboardDigitGlyph(rune(mm[0]), glyphs, leftW-innerPadX, botH-innerPadY)
	d3 := renderDashboardDigitGlyph(rune(mm[1]), glyphs, rightW-innerPadX, botH-innerPadY)
	canvas := make([][]rune, height)
	for y := 0; y < height; y++ {
		canvas[y] = []rune(strings.Repeat(" ", width))
	}

	blitSlotAnchored(canvas, d0, outerPad, outerPad+fontLayout.topNudgeY, leftW, topH, "bottom-right", innerPadX, innerPadY)
	blitSlotAnchored(canvas, d1, outerPad+leftW+gapX, outerPad+fontLayout.topNudgeY, rightW, topH, "bottom-left", innerPadX, innerPadY)
	blitSlotAnchored(canvas, d2, outerPad, outerPad+topH+gapY+fontLayout.bottomNudge, leftW, botH, "top-right", innerPadX, innerPadY)
	blitSlotAnchored(canvas, d3, outerPad+leftW+gapX, outerPad+topH+gapY+fontLayout.bottomNudge, rightW, botH, "top-left", innerPadX, innerPadY)

	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		lines = append(lines, string(canvas[y]))
	}
	palette := boostPalette(m.clockPalette(), 0.18*introWeight(now, m.introUntil))
	shadow := m.themeRoleColor("on_surface", "#565f89")
	return applyClockPatternLinesStable(lines, palette, m.clockPhase, m.currentClockPattern(), m.themeRoleColor("primary", "#7aa2f7"), shadow)
}

func renderClockOnlyGlyphGrid(width, height int, d0, d1, d2, d3 []string, fixedColW int) []string {
	if width < 1 || height < 1 {
		return []string{""}
	}
	row1H := max(len(d0), len(d1))
	row2H := max(len(d2), len(d3))
	rowGap := 1
	if row1H+rowGap+row2H > height {
		rowGap = 0
	}

	col1W := max(maxLineRunes(d0), maxLineRunes(d2))
	col2W := max(maxLineRunes(d1), maxLineRunes(d3))
	if fixedColW > 0 {
		col1W = max(col1W, fixedColW)
		col2W = max(col2W, fixedColW)
	}
	colGap := 2
	if col1W+colGap+col2W > width {
		colGap = 1
	}

	contentW := col1W + colGap + col2W
	contentH := row1H + rowGap + row2H
	startX := max(0, (width-contentW+1)/2)
	startY := max(0, (height-contentH)/2)

	canvas := make([][]rune, height)
	for y := 0; y < height; y++ {
		canvas[y] = []rune(strings.Repeat(" ", width))
	}

	placeGlyphAligned(canvas, d0, startX, startY, col1W, "left")
	placeGlyphAligned(canvas, d1, startX+col1W+colGap, startY, col2W, "right")
	placeGlyphAligned(canvas, d2, startX, startY+row1H+rowGap, col1W, "left")
	placeGlyphAligned(canvas, d3, startX+col1W+colGap, startY+row1H+rowGap, col2W, "right")

	out := make([]string, 0, height)
	for y := 0; y < height; y++ {
		out = append(out, string(canvas[y]))
	}
	return out
}

func placeGlyphAligned(canvas [][]rune, glyph []string, x, y, colW int, align string) {
	if len(canvas) == 0 || len(glyph) == 0 || colW <= 0 {
		return
	}
	baseX := x
	gw := maxLineRunes(glyph)
	switch align {
	case "right":
		baseX = x + max(0, colW-gw)
	case "center":
		baseX = x + max(0, (colW-gw)/2)
	}
	for gy, line := range glyph {
		yy := y + gy
		if yy < 0 || yy >= len(canvas) {
			continue
		}
		r := []rune(line)
		for gx, ch := range r {
			xx := baseX + gx
			if xx < 0 || xx >= len(canvas[yy]) {
				continue
			}
			if ch != ' ' {
				canvas[yy][xx] = ch
			}
		}
	}
}

func renderDashboardDigitGlyph(d rune, glyphs map[rune][]string, w, h int) []string {
	glyph := buildClockLinesWithSpacing(string(d), glyphs, 0)
	if len(glyph) == 0 {
		glyph = buildClockLinesWithSpacing(string(d), nil, 0)
	}
	return glyph
}

func clockGlyphMetrics(glyphs map[rune][]string) (int, int) {
	if len(glyphs) == 0 {
		return 11, 8
	}
	maxW := 0
	maxH := 0
	for d := '0'; d <= '9'; d++ {
		g := buildClockLinesWithSpacing(string(d), glyphs, 0)
		if len(g) == 0 {
			continue
		}
		if w := maxLineRunes(g); w > maxW {
			maxW = w
		}
		if len(g) > maxH {
			maxH = len(g)
		}
	}
	if maxW < 1 {
		maxW = 11
	}
	if maxH < 1 {
		maxH = 8
	}
	return maxW, maxH
}

func clockGlyphMetricsNormalized(glyphs map[rune][]string) (int, int) {
	if len(glyphs) == 0 {
		return 11, 8
	}
	maxW := 0
	maxH := 0
	for d := '0'; d <= '9'; d++ {
		g := buildClockLinesWithSpacing(string(d), glyphs, 0)
		if len(g) == 0 {
			continue
		}
		g = normalizeGlyphLines(g)
		if w := maxLineRunes(g); w > maxW {
			maxW = w
		}
		if len(g) > maxH {
			maxH = len(g)
		}
	}
	if maxW < 1 {
		maxW = 11
	}
	if maxH < 1 {
		maxH = 8
	}
	return maxW, maxH
}

func trimGlyphLinesRightAll(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		out[i] = strings.TrimRight(ln, " ")
	}
	return out
}

func normalizeGlyphLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	lines = trimGlyphLinesRightAll(lines)

	minLead := -1
	for _, ln := range lines {
		r := []rune(ln)
		i := 0
		for i < len(r) && r[i] == ' ' {
			i++
		}
		if i >= len(r) {
			continue
		}
		if minLead < 0 || i < minLead {
			minLead = i
		}
	}
	if minLead <= 0 {
		return lines
	}

	out := make([]string, len(lines))
	for i, ln := range lines {
		r := []rune(ln)
		if minLead >= len(r) {
			out[i] = ""
			continue
		}
		out[i] = string(r[minLead:])
	}
	return out
}

func clockOnlyRightTrimCols(fontName string) int {
	if strings.EqualFold(strings.TrimSpace(fontName), "edges") {
		return 2
	}
	return 1
}

func trimGlyphLinesRight(lines []string, cols int) []string {
	if cols <= 0 || len(lines) == 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		r := []rune(ln)
		trimmed := 0
		for len(r) > 0 && trimmed < cols && r[len(r)-1] == ' ' {
			r = r[:len(r)-1]
			trimmed++
		}
		out[i] = string(r)
	}
	return out
}

func desiredClockPanelWidth(usableW, topH, glyphW, glyphH int) int {
	gapX := max(2, usableW/14)
	quadMinW := max(6, glyphW+2)
	innerNeeded := quadMinW*2 + gapX
	panelNeeded := innerNeeded + 4

	// If font is short, allow narrower; if tall, reserve extra width to reduce distortion.
	if glyphH >= 10 {
		panelNeeded += 2
	}
	maxAllowed := max(22, usableW-24)
	if panelNeeded > maxAllowed {
		panelNeeded = maxAllowed
	}
	return panelNeeded
}

func desiredClockPanelHeight(glyphH int) int {
	// 2 glyph rows + inter-row gap + top/bottom padding + panel frame/padding allowance.
	base := max(8, (glyphH*2)+4)
	return base
}

func blitSlotAnchored(canvas [][]rune, slot []string, ox, oy, qw, qh int, anchor string, padX, padY int) {
	if len(canvas) == 0 || len(slot) == 0 {
		return
	}
	if qw < 1 || qh < 1 {
		return
	}
	gw := maxLineRunes(slot)
	gh := len(slot)
	if gw > qw {
		gw = qw
	}
	if gh > qh {
		gh = qh
	}
	x := ox + padX
	y := oy + padY
	switch anchor {
	case "bottom-right":
		x = ox + qw - gw - padX
		y = oy + qh - gh - padY
	case "bottom-left":
		x = ox + padX
		y = oy + qh - gh - padY
	case "top-right":
		x = ox + qw - gw - padX
		y = oy + padY
	case "top-left":
		x = ox + padX
		y = oy + padY
	}
	if x < ox {
		x = ox
	}
	if y < oy {
		y = oy
	}
	maxX := ox + qw - gw
	maxY := oy + qh - gh
	if x > maxX {
		x = maxX
	}
	if y > maxY {
		y = maxY
	}
	blitSlot(canvas, slot, x, y)
}

func blitSlot(canvas [][]rune, slot []string, x, y int) {
	if len(canvas) == 0 || len(slot) == 0 {
		return
	}
	for yy := 0; yy < len(slot) && y+yy < len(canvas); yy++ {
		runes := []rune(slot[yy])
		for xx := 0; xx < len(runes) && x+xx < len(canvas[y+yy]); xx++ {
			if runes[xx] != ' ' {
				canvas[y+yy][x+xx] = runes[xx]
			}
		}
	}
}

func applyVerticalCenter(lines []string, targetHeight int) []string {
	if targetHeight <= 0 {
		return nil
	}
	if len(lines) >= targetHeight {
		return lines[:targetHeight]
	}
	padTop := (targetHeight - len(lines)) / 2
	padBottom := targetHeight - len(lines) - padTop
	out := make([]string, 0, targetHeight)
	for i := 0; i < padTop; i++ {
		out = append(out, "")
	}
	out = append(out, lines...)
	for i := 0; i < padBottom; i++ {
		out = append(out, "")
	}
	return out
}

func (m model) renderUsageProgressBar(width int, icon string, percent float64, gradientFn func(float64) string) string {
	if width < 1 {
		width = 1
	}
	prefix := " " + icon + " "
	barW := width - lipgloss.Width(prefix) - 1
	if barW < 4 {
		barW = 4
	}
	p := clampPct(percent)
	fill := int(math.Round((p / 100.0) * float64(barW)))
	if fill < 0 {
		fill = 0
	}
	if fill > barW {
		fill = barW
	}
	filledRunes := make([]rune, barW)
	filledColors := make([]string, barW)
	for i := 0; i < barW; i++ {
		if i < fill {
			filledRunes[i] = '█'
			t := 0.0
			if barW > 1 {
				t = float64(i) / float64(barW-1)
			}
			filledColors[i] = gradientFn(t)
		} else {
			filledRunes[i] = '.'
			filledColors[i] = m.themeRoleColor("on_surface", "#565f89")
		}
	}
	bar := renderRunesWithColors(filledRunes, filledColors) + ansiColorSeq(m.themeRoleColor("primary", "#7aa2f7"))
	return prefix + bar + " "
}

func (m model) renderUptimeLine(width int, uptime string) string {
	if width < 1 {
		width = 1
	}
	body := strings.TrimSpace(uptime)
	if body == "" {
		body = "--"
	}
	return cutPad(" 󱕌 : "+body+" ", width)
}

func (m model) cpuBarGradientColor(t float64) string {
	return gradientFromStops(t, []string{
		m.themeRoleColor("secondary", "#94e2d5"),
		blendHexColor(m.themeRoleColor("secondary", "#94e2d5"), m.themeRoleColor("primary", "#89b4fa"), 0.45),
		m.themeRoleColor("primary", "#89b4fa"),
		blendHexColor(m.themeRoleColor("primary", "#89b4fa"), m.themeRoleColor("tertiary", "#cba6f7"), 0.52),
		blendHexColor(m.themeRoleColor("tertiary", "#cba6f7"), m.themeRoleColor("error", "#f38ba8"), 0.45),
		m.themeRoleColor("error", "#f38ba8"),
	})
}

func (m model) ramBarGradientColor(t float64) string {
	return gradientFromStops(t, []string{
		m.themeRoleColor("primary", "#89b4fa"),
		blendHexColor(m.themeRoleColor("primary", "#89b4fa"), m.themeRoleColor("secondary", "#94e2d5"), 0.50),
		m.themeRoleColor("secondary", "#94e2d5"),
		blendHexColor(m.themeRoleColor("secondary", "#94e2d5"), m.themeRoleColor("tertiary", "#cba6f7"), 0.58),
		m.themeRoleColor("tertiary", "#cba6f7"),
		m.themeRoleColor("error", "#f38ba8"),
	})
}

func (m model) storageBarGradientColor(t float64) string {
	muted := m.themeRoleColor("on_surface", "#565f89")
	return gradientFromStops(t, []string{
		blendHexColor(muted, "#000000", 0.15),
		blendHexColor(muted, m.themeRoleColor("surface_variant", "#1f2335"), 0.35),
		blendHexColor(m.themeRoleColor("surface_variant", "#1f2335"), m.themeRoleColor("primary", "#89b4fa"), 0.18),
		blendHexColor(m.themeRoleColor("primary", "#89b4fa"), "#ffffff", 0.12),
	})
}

func (m model) themeRoleColor(role, fallback string) string {
	if m.selectedHexes != nil {
		if c := strings.TrimSpace(m.selectedHexes[role]); c != "" {
			return normalizeHexColor(c)
		}
	}
	return normalizeHexColor(fallback)
}

func ensureReadableTextColor(bg, preferred, fallback string) string {
	bg = normalizeHexColor(bg)
	preferred = normalizeHexColor(preferred)
	fallback = normalizeHexColor(fallback)
	if contrastRatioHex(preferred, bg) >= 4.5 {
		return preferred
	}
	if contrastRatioHex(fallback, bg) >= 4.5 {
		return fallback
	}
	if contrastRatioHex("#ffffff", bg) >= 4.5 {
		return "#ffffff"
	}
	if contrastRatioHex("#000000", bg) >= 4.5 {
		return "#000000"
	}
	best := fallback
	bestRatio := contrastRatioHex(fallback, bg)
	for _, cand := range []string{preferred, "#ffffff", "#000000"} {
		r := contrastRatioHex(cand, bg)
		if r > bestRatio {
			best = cand
			bestRatio = r
		}
	}
	return best
}

func (m model) clockPalette() []string {
	primary := m.themeRoleColor("primary", "#7aa2f7")
	secondary := m.themeRoleColor("secondary", "#7dcfff")
	tertiary := m.themeRoleColor("tertiary", "#bb9af7")
	errorC := m.themeRoleColor("error", "#ff5f5f")
	muted := m.themeRoleColor("on_surface", "#565f89")
	return []string{
		blendHexColor(muted, primary, 0.45),
		primary,
		secondary,
		tertiary,
		errorC,
	}
}

func gradientFromStops(t float64, stops []string) string {
	if len(stops) == 0 {
		return "#cd7f32"
	}
	if len(stops) == 1 {
		return normalizeHexColor(stops[0])
	}
	if t <= 0 {
		return normalizeHexColor(stops[0])
	}
	if t >= 1 {
		return normalizeHexColor(stops[len(stops)-1])
	}
	segments := len(stops) - 1
	scaled := t * float64(segments)
	i := int(math.Floor(scaled))
	if i >= segments {
		i = segments - 1
	}
	local := scaled - float64(i)
	local = smoothstep(local)
	return blendHexColor(stops[i], stops[i+1], local)
}

func smoothstep(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func (m *model) startHomeIntro() {
	now := m.now
	if now.IsZero() {
		now = time.Now()
	}
	m.introUntil = now.Add(1400 * time.Millisecond)
	seed := float64(now.UnixNano()%1000000) / 1000000.0
	m.introSeed = seed
	m.introMode = int(now.UnixNano() % 3)
}

func (m model) introActive(now time.Time) bool {
	return !m.introUntil.IsZero() && now.Before(m.introUntil)
}

func (m model) introTargets(now time.Time) (cpu, ram, stor float64) {
	until := m.introUntil
	total := 1.4
	elapsed := total - until.Sub(now).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > total {
		elapsed = total
	}
	p := elapsed / total
	seed := m.introSeed
	switch m.introMode {
	case 1:
		// Full-length sweep with slight stagger.
		cpu = 100 * triWave(p+seed*0.08)
		ram = 100 * triWave(p+0.12+seed*0.05)
		stor = 100 * triWave(p+0.24+seed*0.03)
	case 2:
		// Knight Rider-style bounce sweep.
		cpu = 100 * triWave((p*1.6)+(seed*0.15))
		ram = 100 * triWave((p*1.6)+0.22+(seed*0.1))
		stor = 100 * triWave((p*1.6)+0.44+(seed*0.06))
	default:
		// Pulse wave but spanning full range.
		cpu = 100 * wavePulse(p, 0.00, 0.85)
		ram = 100 * wavePulse(p, 0.08, 0.85)
		stor = 100 * wavePulse(p, 0.16, 0.85)
	}
	return clampPct(cpu), clampPct(ram), clampPct(stor)
}

func introWeight(now, until time.Time) float64 {
	total := 1.4
	elapsed := total - until.Sub(now).Seconds()
	if elapsed <= 0 {
		return 1.0
	}
	if elapsed >= total {
		return 0
	}
	p := elapsed / total
	if p < 0.18 {
		return 1.0
	}
	f := (p - 0.18) / 0.82
	return 1.0 - smoothstep(f)
}

func wavePulse(progress, start, width float64) float64 {
	if width <= 0 {
		return 0
	}
	x := (progress - start) / width
	if x <= 0 || x >= 1 {
		return 0
	}
	return math.Sin(math.Pi * x)
}

func triWave(x float64) float64 {
	x = math.Mod(x, 1.0)
	if x < 0 {
		x += 1.0
	}
	if x < 0.5 {
		return x * 2.0
	}
	return (1.0 - x) * 2.0
}

func blendPercent(realTarget, showcaseTarget, showcaseWeight float64) float64 {
	w := showcaseWeight
	if w < 0 {
		w = 0
	}
	if w > 1 {
		w = 1
	}
	return clampPct(realTarget*(1.0-w) + showcaseTarget*w)
}

func readTooieResources(timeout time.Duration) (cpuPct float64, memUsed uint64, memTotal uint64, ok bool) {
	base, token, found := readTooieEndpointToken()
	if !found {
		return 0, 0, 0, false
	}
	res, found := tooieJSONRequest(base, token, "/v1/system/resources", timeout)
	if !found {
		return 0, 0, 0, false
	}
	cpuPct = numberFromAny(res["cpuPercent"])
	memUsed = uint64(numberFromAny(res["memUsedBytes"]))
	memTotal = uint64(numberFromAny(res["memTotalBytes"]))
	if cpuPct <= 0 && memTotal == 0 {
		if m, okMem := res["memory"].(map[string]any); okMem {
			memUsed = uint64(numberFromAny(m["usedBytes"]))
			memTotal = uint64(numberFromAny(m["totalBytes"]))
		}
	}
	if cpuPct < 0 {
		cpuPct = 0
	}
	if cpuPct > 100 {
		cpuPct = 100
	}
	if memTotal == 0 {
		return cpuPct, memUsed, memTotal, false
	}
	return cpuPct, memUsed, memTotal, true
}

func readTooieEndpointToken() (string, string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", "", false
	}
	endpointData, err := os.ReadFile(filepath.Join(home, ".launcherctl", "endpoint"))
	if err != nil {
		return "", "", false
	}
	tokenData, err := os.ReadFile(filepath.Join(home, ".launcherctl", "token"))
	if err != nil {
		return "", "", false
	}
	base := strings.TrimSpace(string(endpointData))
	token := strings.TrimSpace(string(tokenData))
	if base == "" || token == "" {
		return "", "", false
	}
	return base, token, true
}

func tooieJSONRequest(base, token, path string, timeout time.Duration) (map[string]any, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	url := strings.TrimRight(base, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, false
	}
	return out, true
}

func numberFromAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case uint64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

func detectClockLocation() *time.Location {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	cmd := exec.Command("getprop", "persist.sys.timezone")
	out, err := cmd.Output()
	if err == nil {
		if tz := strings.TrimSpace(string(out)); tz != "" {
			if loc, err := time.LoadLocation(tz); err == nil {
				return loc
			}
		}
	}
	return time.Local
}

func clockSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "tooie-clock-settings.json"
	}
	return filepath.Join(home, ".config", "tooie", "clock-settings.json")
}

func shellSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "tooie-shell-settings.json"
	}
	return filepath.Join(home, ".config", "tooie", "shell-settings.json")
}

func loadClockSettings() persistedClockSettings {
	path := clockSettingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return persistedClockSettings{}
	}
	var s persistedClockSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return persistedClockSettings{}
	}
	return s
}

func (m *model) persistClockSettings() {
	path := clockSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	out := persistedClockSettings{
		Pattern: m.currentClockPattern(),
	}
	if m.clockFontIdx >= 0 && m.clockFontIdx < len(m.clockFontDefs) {
		out.Font = m.clockFontDefs[m.clockFontIdx].Name
	}
	if m.calFontIdx >= 0 && m.calFontIdx < len(m.calFontDefs) {
		out.CalFont = m.calFontDefs[m.calFontIdx].Name
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func loadClockGlyphSet(fontDefs []clockFontDef, idx int) map[rune][]string {
	if len(fontDefs) == 0 {
		return nil
	}
	if idx < 0 || idx >= len(fontDefs) {
		idx = 0
	}
	fd := fontDefs[idx]
	return loadClockTextFont(fd.Dir, fd.Name)
}

func discoverClockFonts() []clockFontDef {
	candidates := []string{
		filepath.Join(homeDir, ".config", "tooie", "fonts"),
		filepath.Join(homeDir, "files", "tooie", "fonts"),
	}
	var out []clockFontDef
	seen := map[string]bool{}
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "mousetrap", "retropixelthick", "pixelzone":
				continue
			}
			if !hasClockTXTGlyphSet(filepath.Join(dir, name)) {
				continue
			}
			key := dir + "/" + name
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, clockFontDef{Name: name, Dir: dir})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func discoverCalendarFonts() []clockFontDef {
	candidates := []string{
		filepath.Join(homeDir, ".config", "tooie", "fonts"),
		filepath.Join(homeDir, "files", "tooie", "fonts"),
	}
	var out []clockFontDef
	seen := map[string]bool{}
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := strings.TrimSpace(e.Name())
			if !strings.HasPrefix(strings.ToLower(name), "cal-") {
				continue
			}
			fontPath := filepath.Join(dir, name)
			if !hasCalendarTXTGlyphSet(fontPath) {
				continue
			}
			key := fontPath
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, clockFontDef{Name: name, Dir: dir})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func hasClockTXTGlyphSet(dir string) bool {
	required := []string{"0.txt", "1.txt", "2.txt", "3.txt", "4.txt", "5.txt", "6.txt", "7.txt", "8.txt", "9.txt", "colon.txt"}
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

func hasCalendarTXTGlyphSet(dir string) bool {
	for d := 1; d <= 31; d++ {
		plain := filepath.Join(dir, fmt.Sprintf("%d.txt", d))
		padded := filepath.Join(dir, fmt.Sprintf("%02d.txt", d))
		if _, err := os.Stat(plain); err != nil {
			if _, err2 := os.Stat(padded); err2 != nil {
				return false
			}
		}
	}
	return true
}

func resolveCalendarGlyphFile(fontPath string, day int) string {
	plain := filepath.Join(fontPath, fmt.Sprintf("%d.txt", day))
	if _, err := os.Stat(plain); err == nil {
		return plain
	}
	padded := filepath.Join(fontPath, fmt.Sprintf("%02d.txt", day))
	if _, err := os.Stat(padded); err == nil {
		return padded
	}
	return plain
}

func loadCalendarTextFont(fontDir, fontName string) map[int][]string {
	fontPath := filepath.Join(fontDir, fontName)
	out := make(map[int][]string)
	for d := 1; d <= 31; d++ {
		file := resolveCalendarGlyphFile(fontPath, d)
		data, err := os.ReadFile(file)
		if err != nil {
			return nil
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		out[d] = lines
	}
	return out
}

func loadCalendarGlyphSet(fontDefs []clockFontDef, idx int) map[int][]string {
	if len(fontDefs) == 0 {
		return nil
	}
	if idx < 0 || idx >= len(fontDefs) {
		idx = 0
	}
	fd := fontDefs[idx]
	return loadCalendarTextFont(fd.Dir, fd.Name)
}

func loadClockTextFont(fontDir, fontName string) map[rune][]string {
	fontPath := filepath.Join(fontDir, fontName)
	glyphMap := map[rune]string{
		'0': "0.txt", '1': "1.txt", '2': "2.txt", '3': "3.txt", '4': "4.txt",
		'5': "5.txt", '6': "6.txt", '7': "7.txt", '8': "8.txt", '9': "9.txt",
		':': "colon.txt",
	}
	out := make(map[rune][]string)
	for ch, file := range glyphMap {
		data, err := os.ReadFile(filepath.Join(fontPath, file))
		if err != nil {
			return nil
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		out[ch] = lines
	}
	return out
}

var bigDigits = map[rune][]string{
	'0': {" ██████ ", "██    ██", "██    ██", "██    ██", "██    ██", "██    ██", " ██████ "},
	'1': {"   ██   ", " ████   ", "   ██   ", "   ██   ", "   ██   ", "   ██   ", " ██████ "},
	'2': {" ██████ ", "██    ██", "      ██", " ██████ ", "██      ", "██      ", "████████"},
	'3': {" ██████ ", "██    ██", "      ██", "  █████ ", "      ██", "██    ██", " ██████ "},
	'4': {"██   ██ ", "██   ██ ", "██   ██ ", "████████", "     ██ ", "     ██ ", "     ██ "},
	'5': {"████████", "██      ", "██      ", "██████  ", "      ██", "██    ██", " ██████ "},
	'6': {" ██████ ", "██      ", "██      ", "██████  ", "██    ██", "██    ██", " ██████ "},
	'7': {"████████", "     ██ ", "    ██  ", "   ██   ", "  ██    ", " ██     ", "██      "},
	'8': {" ██████ ", "██    ██", "██    ██", " ██████ ", "██    ██", "██    ██", " ██████ "},
	'9': {" ██████ ", "██    ██", "██    ██", " ███████", "      ██", "      ██", " ██████ "},
	':': {"  ", "██", "██", "  ", "██", "██", "  "},
}

func buildClockLinesWithSpacing(text string, glyphs map[rune][]string, spacing int) []string {
	textRunes := []rune(text)
	if len(textRunes) == 0 {
		return nil
	}
	lookup := glyphs
	if lookup == nil {
		lookup = bigDigits
	}
	height := 0
	parts := make([][]string, 0, len(textRunes))
	for _, ch := range textRunes {
		g, ok := lookup[ch]
		if !ok || len(g) == 0 {
			g = []string{string(ch)}
		}
		parts = append(parts, g)
		if len(g) > height {
			height = len(g)
		}
	}
	if height <= 0 {
		return nil
	}
	lines := make([]string, height)
	sep := strings.Repeat(" ", spacing)
	for row := 0; row < height; row++ {
		var b strings.Builder
		for i, g := range parts {
			if row < len(g) {
				b.WriteString(g[row])
			}
			if i != len(parts)-1 {
				b.WriteString(sep)
			}
		}
		lines[row] = b.String()
	}
	return lines
}

func applyClockPatternLinesStable(lines []string, palette []string, phase float64, pattern, restoreColor, shadowColor string) []string {
	prevShadow := activeShadowColor
	activeShadowColor = shadowColor
	defer func() { activeShadowColor = prevShadow }()

	out := make([]string, len(lines))
	for i, line := range lines {
		colored := applyClockPattern(line, palette, phase, i, pattern)
		if strings.TrimSpace(line) == "" {
			out[i] = line
			continue
		}
		out[i] = colored + ansiColorSeq(restoreColor)
	}
	return out
}

func normalizeClockPattern(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "wave", "stripes", "pulse", "solid", "outline", "sweep", "neon", "calm", "shimmer":
		return strings.ToLower(strings.TrimSpace(name))
	default:
		return "wave"
	}
}

func applyClockPattern(line string, palette []string, phase float64, lineIndex int, pattern string) string {
	switch normalizeClockPattern(pattern) {
	case "wave":
		return gradientLine(line, palette, phase+float64(lineIndex)*0.25)
	case "stripes":
		return stripesLine(line, palette, phase+float64(lineIndex)*0.12)
	case "pulse":
		return pulseLine(line, palette, phase, lineIndex)
	case "solid":
		return pulseLine(line, palette, 0, lineIndex)
	case "outline":
		return outlineLine(line, palette, phase+float64(lineIndex)*0.15)
	case "sweep":
		return sweepLine(line, palette, phase, lineIndex)
	case "neon":
		return neonLine(line, palette, phase, lineIndex)
	case "calm":
		return calmLine(line, palette, phase)
	case "shimmer":
		return shimmerLine(line, palette, phase, lineIndex)
	default:
		return gradientLine(line, palette, phase+float64(lineIndex)*0.25)
	}
}

func gradientLine(text string, palette []string, phase float64) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	runes := []rune(text)
	colors := make([]string, len(runes))
	count := float64(max(len(runes), 1))
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		t := (float64(i/2)/count)*1.35 + phase
		colors[i] = sampleGradientColor(palette, t)
	}
	return renderRunesWithColors(runes, colors)
}

func stripesLine(text string, palette []string, phase float64) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	shift := int(math.Floor(phase*6.0)) % len(palette)
	if shift < 0 {
		shift += len(palette)
	}
	runes := []rune(text)
	colors := make([]string, len(runes))
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		colors[i] = palette[((i/2)+shift)%len(palette)]
	}
	return renderRunesWithColors(runes, colors)
}

func pulseLine(text string, palette []string, phase float64, lineIndex int) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	runes := []rune(text)
	colors := make([]string, len(runes))
	n := max(len(runes), 1)
	basePhase := phase*0.32 + float64(lineIndex)*0.035
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		x := float64(i/2) / float64(n)
		grad := basePhase + x*0.22
		c0 := sampleGradientColor(palette, grad)
		c1 := sampleGradientColor(palette, grad+0.12)
		blend := 0.5 + 0.5*math.Sin(phase*0.9+float64(i/2)*0.07+float64(lineIndex)*0.13)
		colors[i] = blendHexColor(c0, c1, blend*0.35)
	}
	return renderRunesWithColors(runes, colors)
}

func outlineLine(text string, palette []string, phase float64) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	base := palette[0]
	accent := sampleGradientColor(palette, phase*0.35+0.15)
	runes := []rune(text)
	colors := make([]string, len(runes))
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		prevSpace := i == 0 || runes[i-1] == ' '
		nextSpace := i == len(runes)-1 || runes[i+1] == ' '
		if prevSpace || nextSpace {
			colors[i] = accent
		} else {
			colors[i] = base
		}
	}
	return renderRunesWithColors(runes, colors)
}

func sweepLine(text string, palette []string, phase float64, lineIndex int) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return text
	}
	colors := make([]string, n)
	highlight := sampleGradientColor(palette, phase*0.22+0.31)
	center := math.Mod(phase*9.0+float64(lineIndex)*0.9, float64(n)+14.0) - 7.0
	const sigma = 2.2
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		base := sampleGradientColor(palette, phase*0.15+float64(i/2)/float64(max(n, 1))*0.45)
		d := float64(i) - center
		glow := math.Exp(-(d * d) / (2 * sigma * sigma))
		colors[i] = blendHexColor(base, highlight, 0.85*glow)
	}
	return renderRunesWithColors(runes, colors)
}

func neonLine(text string, palette []string, phase float64, lineIndex int) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	runes := []rune(text)
	colors := make([]string, len(runes))
	base := palette[0]
	ringA := sampleGradientColor(palette, phase*0.42+0.11)
	ringB := sampleGradientColor(palette, phase*0.73+0.57)
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		prevSpace := i == 0 || runes[i-1] == ' '
		nextSpace := i == len(runes)-1 || runes[i+1] == ' '
		c := base
		if prevSpace || nextSpace {
			a := 0.5 + 0.5*math.Sin(phase*4.0+float64(i/2)*0.45+float64(lineIndex)*0.71)
			b := 0.5 + 0.5*math.Sin(phase*2.8-float64(i/2)*0.31+float64(lineIndex)*1.13+1.27)
			mix := 0.55*a + 0.45*b
			edgeBase := blendHexColor(base, ringA, 0.62)
			c = blendHexColor(edgeBase, ringB, 0.65*mix)
		}
		colors[i] = c
	}
	return renderRunesWithColors(runes, colors)
}

func calmLine(text string, palette []string, phase float64) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	c := sampleGradientColor(palette, phase*0.08+0.12)
	runes := []rune(text)
	colors := make([]string, len(runes))
	for i, r := range runes {
		if r != ' ' {
			colors[i] = c
		}
	}
	return renderRunesWithColors(runes, colors)
}

func shimmerLine(text string, palette []string, phase float64, lineIndex int) string {
	if text == "" || len(palette) == 0 {
		return text
	}
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return text
	}
	colors := make([]string, n)
	base := sampleGradientColor(palette, phase*0.22+float64(lineIndex)*0.09)
	spark := blendHexColor(sampleGradientColor(palette, phase*0.51+0.27), "#ffffff", 0.30)
	center := math.Mod(phase*13.0+float64(lineIndex)*1.7, float64(n)+16.0) - 8.0
	const sigma = 1.35
	for i, r := range runes {
		if r == ' ' {
			continue
		}
		d := float64(i) - center
		glow := math.Exp(-(d * d) / (2 * sigma * sigma))
		colors[i] = blendHexColor(base, spark, 0.95*glow)
	}
	return renderRunesWithColors(runes, colors)
}

func sampleGradientColor(palette []string, t float64) string {
	if len(palette) == 0 {
		return "#cd7f32"
	}
	if len(palette) == 1 {
		return normalizeHexColor(palette[0])
	}
	tt := math.Mod(t, 1.0)
	if tt < 0 {
		tt += 1.0
	}
	n := len(palette)
	scaled := tt * float64(n)
	i0 := int(math.Floor(scaled)) % n
	i1 := (i0 + 1) % n
	fr := scaled - float64(i0)
	return blendHexColor(palette[i0], palette[i1], fr)
}

var activeShadowColor string
var colorStyleCache = map[string]lipgloss.Style{}

func renderRunesWithColors(runes []rune, colors []string) string {
	if len(runes) == 0 {
		return ""
	}
	var b strings.Builder
	for i, r := range runes {
		c := ""
		if i < len(colors) {
			c = colors[i]
		}
		if isShadowRune(r) && strings.TrimSpace(activeShadowColor) != "" {
			c = blendHexColor(activeShadowColor, "#000000", 0.45)
		}
		if strings.TrimSpace(c) == "" {
			b.WriteRune(r)
			continue
		}
		key := normalizeHexColor(c)
		st, ok := colorStyleCache[key]
		if !ok {
			st = lipgloss.NewStyle().Foreground(lipgloss.Color(key))
			colorStyleCache[key] = st
		}
		b.WriteString(st.Render(string(r)))
	}
	return b.String()
}

func isShadowRune(r rune) bool {
	switch r {
	case '░', '▒', '▓', '.', '·', '▪', '▫':
		return true
	default:
		return false
	}
}

func ansiColorSeq(color string) string {
	r, g, b := parseHexColor(color)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

func blendHexColor(a, b string, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab := parseHexColor(a)
	br, bg, bb := parseHexColor(b)
	r := int(math.Round(float64(ar) + (float64(br-ar) * t)))
	g := int(math.Round(float64(ag) + (float64(bg-ag) * t)))
	bl := int(math.Round(float64(ab) + (float64(bb-ab) * t)))
	r = clamp255(r)
	g = clamp255(g)
	bl = clamp255(bl)
	return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
}

func boostPalette(palette []string, boost float64) []string {
	if len(palette) == 0 || boost <= 0 {
		return palette
	}
	if boost > 0.45 {
		boost = 0.45
	}
	factor := 1.0 + boost
	out := make([]string, len(palette))
	for i, c := range palette {
		r, g, b := parseHexColor(c)
		r = clamp255(int(math.Round(float64(r) * factor)))
		g = clamp255(int(math.Round(float64(g) * factor)))
		b = clamp255(int(math.Round(float64(b) * factor)))
		out[i] = fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	return out
}

func normalizeHexColor(c string) string {
	r, g, b := parseHexColor(c)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func terminalColorHex(c string) string {
	switch strings.TrimSpace(strings.ToLower(c)) {
	case "0":
		return "#111111"
	case "1":
		return "#ff5f5f"
	case "2":
		return "#8bd450"
	case "3":
		return "#ffd75f"
	case "4":
		return "#5fafff"
	case "5":
		return "#d787ff"
	case "6":
		return "#5fd7d7"
	case "7":
		return "#d7d7d7"
	case "8":
		return "#6c7086"
	case "9":
		return "#ff6b6b"
	case "10":
		return "#a6e36e"
	case "11":
		return "#ffe082"
	case "12":
		return "#89b4fa"
	case "13":
		return "#f5c2e7"
	case "14":
		return "#94e2d5"
	case "15":
		return "#f5f5f8"
	default:
		if strings.HasPrefix(c, "#") {
			return normalizeHexColor(c)
		}
		return "#d7d7d7"
	}
}

func contrastRatioHex(a, b string) float64 {
	return contrastLuminance(relativeLuminanceHex(a), relativeLuminanceHex(b))
}

func relativeLuminanceHex(c string) float64 {
	r, g, b := parseHexColor(c)
	return 0.2126*linearizeChannel(r) + 0.7152*linearizeChannel(g) + 0.0722*linearizeChannel(b)
}

func linearizeChannel(v int) float64 {
	x := float64(v) / 255.0
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

func contrastLuminance(a, b float64) float64 {
	hi := math.Max(a, b)
	lo := math.Min(a, b)
	return (hi + 0.05) / (lo + 0.05)
}

func parseHexColor(c string) (int, int, int) {
	c = strings.TrimSpace(strings.TrimPrefix(c, "#"))
	if len(c) == 3 {
		c = fmt.Sprintf("%c%c%c%c%c%c", c[0], c[0], c[1], c[1], c[2], c[2])
	}
	if len(c) != 6 {
		return 205, 127, 50
	}
	var r, g, b int
	if _, err := fmt.Sscanf(c, "%02x%02x%02x", &r, &g, &b); err != nil {
		return 205, 127, 50
	}
	return r, g, b
}

func clamp255(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func centerText(s string, w int) string {
	s = clip(s, w)
	pad := w - runeLen(s)
	if pad <= 0 {
		return s
	}
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func cutPad(s string, w int) string {
	s = clip(s, w)
	pad := w - runeLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w])
}

func runeLen(s string) int {
	return len([]rune(s))
}

func applyProgressPath() string {
	return filepath.Join(tooieConfigDir, "apply-progress.json")
}

func readApplyProgressState() (applyProgressState, bool) {
	raw, err := os.ReadFile(applyProgressPath())
	if err != nil || len(raw) == 0 {
		return applyProgressState{}, false
	}
	var st applyProgressState
	if err := json.Unmarshal(raw, &st); err != nil {
		return applyProgressState{}, false
	}
	if st.Progress < 0 {
		st.Progress = 0
	}
	if st.Progress > 1 {
		st.Progress = 1
	}
	return st, true
}

func (m *model) loadPreviewColors() {
	m.selectedHexes = map[string]string{}
	if len(m.backups) == 0 {
		return
	}
	meta := m.backups[m.backupIndex].Meta
	for _, item := range []struct {
		Role string
		Key  string
	}{
		{"background", "effective_background"},
		{"surface", "effective_surface"},
		{"on_surface", "effective_on_surface"},
		{"outline", "effective_outline"},
		{"primary", "effective_primary"},
		{"secondary", "effective_secondary"},
		{"tertiary", "effective_tertiary"},
		{"error", "effective_error"},
	} {
		if c := strings.TrimSpace(meta[item.Key]); c != "" {
			m.selectedHexes[item.Role] = strings.ToLower(c)
		}
	}
	for k, v := range meta {
		if !strings.HasPrefix(k, "effective_role_") {
			continue
		}
		role := strings.TrimSpace(strings.TrimPrefix(k, "effective_role_"))
		hex := strings.ToLower(strings.TrimSpace(v))
		if role == "" || !internaltheme.IsHexColor(hex) {
			continue
		}
		m.selectedHexes[role] = hex
	}
	if len(m.selectedHexes) > 12 {
		return
	}
	p := filepath.Join(backupRoot, m.backups[m.backupIndex].ID, "matugen.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var data matugenJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	for k, v := range data.Colors {
		if _, ok := m.selectedHexes[k]; ok {
			continue
		}
		m.selectedHexes[k] = strings.TrimSpace(v.Default.Color)
	}
}

func panelStyle(w, h int, borderColor string) lipgloss.Style {
	if w < 28 {
		w = 28
	}
	if h < 4 {
		h = 4
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1).
		Width(w - 2).
		Height(h - 2)
}

func framedPanel(w, h int, borderColor, body, topLabel, topAlign, bottomLabel, bottomAlign string) string {
	if w < 28 {
		w = 28
	}
	if h < 3 {
		h = 3
	}
	border := lipgloss.RoundedBorder()
	innerW := max(1, w-4)
	innerH := max(1, h-2)
	lines := strings.Split(body, "\n")

	top := framedBorderLine(w, borderColor, border.TopLeft, border.Top, border.TopRight, topLabel, topAlign)
	bottom := framedBorderLine(w, borderColor, border.BottomLeft, border.Bottom, border.BottomRight, bottomLabel, bottomAlign)
	sideStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	contentStyle := lipgloss.NewStyle().Width(innerW)

	out := make([]string, 0, h)
	out = append(out, top)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out,
			sideStyle.Render(border.Left)+" "+
				contentStyle.Render(line)+" "+
				sideStyle.Render(border.Right),
		)
	}
	out = append(out, bottom)
	return strings.Join(out, "\n")
}

func framedBorderLine(w int, borderColor, left, horiz, right, label, align string) string {
	sideStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	innerW := max(1, w-2)
	if strings.TrimSpace(label) == "" {
		return sideStyle.Render(left + strings.Repeat(horiz, innerW) + right)
	}

	labelW := lipgloss.Width(label) + 2
	if labelW >= innerW {
		return sideStyle.Render(left + strings.Repeat(horiz, innerW) + right)
	}

	inset := 2
	if inset+labelW > innerW {
		inset = 1
	}
	start := inset
	switch align {
	case "right":
		start = innerW - labelW - inset
	case "center":
		start = (innerW - labelW) / 2
	}
	if start < 0 {
		start = 0
	}
	rightFill := innerW - start - labelW
	if rightFill < 0 {
		rightFill = 0
	}

	return sideStyle.Render(left) +
		sideStyle.Render(strings.Repeat(horiz, start)) +
		" " + label + " " +
		sideStyle.Render(strings.Repeat(horiz, rightFill)) +
		sideStyle.Render(right)
}

func joinLR(left, right string, totalWidth int) string {
	avail := totalWidth
	if avail <= 0 {
		avail = lipgloss.Width(left) + lipgloss.Width(right) + 1
	}
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	if lw+rw+1 >= avail {
		return left + " " + right
	}
	return left + strings.Repeat(" ", avail-lw-rw) + right
}

func (m model) renderApplyStatus(totalWidth int) string {
	barW := max(18, totalWidth-10)
	p := m.applyProgress
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	filled := int(math.Round(p * float64(barW)))
	if filled < 0 {
		filled = 0
	}
	if filled > barW {
		filled = barW
	}
	percent := int(math.Round(p * 100))
	if percent > 99 && m.applying {
		percent = 99
	}
	label := strings.TrimSpace(m.applyLabel)
	if label == "" {
		label = "Applying theme"
	}
	text := fmt.Sprintf("%s %d%%", label, percent)
	bar := m.renderApplyStatusBar(barW, filled, text)
	return "status: " + bar
}

func (m model) renderApplyStatusBar(barW, filled int, text string) string {
	if barW < 8 {
		barW = 8
	}
	if filled < 0 {
		filled = 0
	}
	if filled > barW {
		filled = barW
	}
	textRunes := []rune(cutPad(strings.TrimSpace(text), barW))
	start := 0
	if len(textRunes) < barW {
		start = (barW - len(textRunes)) / 2
	}
	mutedBg := blendHexColor(m.themeRoleColor("surface_container", "#1f2335"), m.themeRoleColor("background", "#11131c"), 0.35)
	emptyFg := m.themeRoleColor("on_surface", "#7f849c")
	fillTextFg := ensureReadableTextColor(m.themeRoleColor("background", "#11131c"), m.themeRoleColor("on_primary", "#0b0f16"), m.themeRoleColor("primary", "#89b4fa"))
	out := strings.Builder{}
	for i := 0; i < barW; i++ {
		ch := ' '
		if idx := i - start; idx >= 0 && idx < len(textRunes) {
			ch = textRunes[idx]
		}
		if i < filled {
			t := 0.0
			if barW > 1 {
				t = math.Mod((float64(i)/float64(barW-1))+(m.clockPhase*0.10), 1.0)
			}
			bg := gradientFromStops(t, []string{
				m.themeRoleColor("primary", "#89b4fa"),
				m.themeRoleColor("secondary", "#94e2d5"),
				m.themeRoleColor("tertiary", "#cba6f7"),
				m.themeRoleColor("primary", "#89b4fa"),
			})
			out.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fillTextFg)).
				Render(string(ch)))
			continue
		}
		out.WriteString(lipgloss.NewStyle().
			Background(lipgloss.Color(mutedBg)).
			Foreground(lipgloss.Color(emptyFg)).
			Render(string(ch)))
	}
	return out.String()
}

func loadBackups() []backup {
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return nil
	}
	type backupEntry struct {
		id      string
		modTime time.Time
	}
	items := make([]backupEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, backupEntry{id: e.Name(), modTime: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].modTime.Equal(items[j].modTime) {
			return items[i].id > items[j].id
		}
		return items[i].modTime.After(items[j].modTime)
	})
	out := make([]backup, 0, len(items))
	for _, item := range items {
		out = append(out, backup{
			ID:   item.id,
			Meta: readMeta(filepath.Join(backupRoot, item.id, "meta.env")),
		})
	}
	return out
}

func readMeta(path string) map[string]string {
	out := map[string]string{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxLineRunes(lines []string) int {
	w := 0
	for _, ln := range lines {
		if lw := len([]rune(ln)); lw > w {
			w = lw
		}
	}
	return w
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(runCLI(os.Args[1:]))
	}
	os.Exit(runUICommand(nil))
}

func runMiniTUI(showClock, showCal bool) int {
	var model model
	switch {
	case showClock && showCal:
		model = initialClockCalModel()
	case showCal:
		model = initialCalModel()
	default:
		model = initialClockModel()
	}
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithFPS(24))
	_, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie mini mode: %v\n", err)
		return 1
	}
	return 0
}
