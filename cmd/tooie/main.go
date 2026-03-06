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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

const (
	homeDir        = "/data/data/com.termux/files/home"
	tooieConfigDir = homeDir + "/.config/tooie"
	backupRoot     = tooieConfigDir + "/backups"
	applyScript    = tooieConfigDir + "/apply-material.sh"
	restoreScript  = tooieConfigDir + "/restore-material.sh"
	defaultWall    = homeDir + "/.termux/background/background.jpeg"
	defaultMode    = "dark"
	defaultPalette = "default"
	defaultPreset  = "balanced"
	pageTheme      = 0
	pageHome       = 1
)

var stylePresets = []string{"balanced", "tokyonight", "catppuccin", "gruvbox", "rose-pine", "pure-matugen"}

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
	uptimeText  string
	err         error
}

type applyDoneMsg struct {
	label string
	err   error
	out   string
}

type homeAppsLoadedMsg struct {
	apps []launchableApp
	err  error
}

type homeLaunchDoneMsg struct {
	label string
	err   error
}

type homeIconsWarmedMsg struct {
	packages []string
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

type model struct {
	width            int
	height           int
	page             int
	now              time.Time
	cpuPercent       float64
	cpuFiltered      float64
	ramUsedGB        float64
	ramTotalGB       float64
	ramFiltered      float64
	storUsedGB       float64
	storTotalGB      float64
	storPercent      float64
	storFiltered     float64
	uptimeText       string
	cpuViz           float64
	ramViz           float64
	storViz          float64
	cpuVel           float64
	ramVel           float64
	storVel          float64
	lastTick         time.Time
	barSpring        harmonica.Spring
	clockPhase       float64
	introUntil       time.Time
	introMode        int
	introSeed        float64
	clockFontDefs    []clockFontDef
	clockFontIdx     int
	clockGlyphs      map[rune][]string
	clockPatterns    []string
	patternIndex     int
	clockLoc         *time.Location
	noticeText       string
	noticeKind       string
	noticeStart      time.Time
	noticeUntil      time.Time
	backups          []backup
	backupIndex      int
	settingIndex     int
	customIndex      int
	mode             string
	palette          string
	stylePreset      string
	textColor        string
	cursorColor      string
	ansiRed          string
	ansiGreen        string
	ansiYellow       string
	ansiBlue         string
	ansiMagenta      string
	ansiCyan         string
	lastStatus       string
	pickerTarget     string
	pickerIndex      int
	customizing      bool
	showHints        bool
	showBackups      bool
	selectedHexes    map[string]string
	metricsErr       error
	applying         bool
	applyProgress    float64
	applyVel         float64
	applyTarget      float64
	applyLabel       string
	apps             []launchableApp
	appsLoaded       bool
	appLoadErr       error
	pinnedPackages   []string
	pinnedApps       []launchableApp
	pinnedIndex      int
	showAppSearch    bool
	appSearchQuery   string
	appSearchIndex   int
	appSearchResults []launchableApp
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
		page:          pageHome,
		backups:       bs,
		now:           now,
		lastTick:      now,
		uptimeText:    "--",
		mode:          defaultMode,
		palette:       defaultPalette,
		stylePreset:   defaultPreset,
		lastStatus:    "Ready",
		textColor:     "",
		cursorColor:   "",
		ansiRed:       "",
		ansiGreen:     "",
		ansiYellow:    "",
		ansiBlue:      "",
		ansiMagenta:   "",
		ansiCyan:      "",
		barSpring:     harmonica.NewSpring(harmonica.FPS(20), 4.6, 0.90),
		clockFontDefs: fontDefs,
		clockFontIdx:  clockFontIdx,
		clockPatterns: []string{"wave", "stripes", "pulse", "solid", "outline", "sweep", "neon", "calm", "shimmer"},
		clockLoc:      clockLoc,
	}
	m.clockGlyphs = loadClockGlyphSet(m.clockFontDefs, m.clockFontIdx)
	if strings.TrimSpace(savedClock.Pattern) != "" {
		for i, p := range m.clockPatterns {
			if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(savedClock.Pattern)) {
				m.patternIndex = i
				break
			}
		}
	}
	m.loadPreviewColors()
	m.pinnedPackages = loadPinnedApps()
	m.refreshAppSearchResults()
	m.startHomeIntro()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickClock(), tickMetrics(), pollMetrics(), loadHomeAppsCmd(false), warmPinnedIconsCmd(m.pinnedPackages))
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
			memUsedBytes = vm.Used
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
			uptimeText:  fmt.Sprintf("%dd %dh", d, h),
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		now := time.Time(msg)
		if m.clockLoc != nil {
			now = now.In(m.clockLoc)
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
		return m, tickClock()
	case metricsTickMsg:
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
	case applyTickMsg:
		if !m.applying {
			return m, nil
		}
		if m.applyTarget < 0.92 {
			m.applyTarget += 0.06
			if m.applyTarget > 0.92 {
				m.applyTarget = 0.92
			}
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
			m.lastStatus = msg.label + " completed"
		}
		m.backups = loadBackups()
		if m.backupIndex >= len(m.backups) {
			m.backupIndex = max(0, len(m.backups)-1)
		}
		m.loadPreviewColors()
		return m, nil
	case homeAppsLoadedMsg:
		m.appsLoaded = true
		m.appLoadErr = msg.err
		if msg.err == nil {
			m.apps = msg.apps
			if len(m.pinnedPackages) == 0 {
				m.pinnedPackages = defaultPinnedPackages(m.apps)
				savePinnedApps(m.pinnedPackages)
			}
			m.refreshPinnedApps()
			m.refreshAppSearchResults()
			return m, warmPinnedIconsCmd(m.pinnedPackages)
		}
		return m, nil
	case homeLaunchDoneMsg:
		if msg.err != nil {
			m.lastStatus = msg.label + " failed: " + msg.err.Error()
		} else {
			m.lastStatus = "Launched " + msg.label
		}
		return m, nil
	case homeIconsWarmedMsg:
		m.refreshPinnedApps()
		m.refreshAppSearchResults()
		return m, nil
	case statusMsg:
		m.lastStatus = string(msg)
		m.backups = loadBackups()
		if m.backupIndex >= len(m.backups) {
			m.backupIndex = max(0, len(m.backups)-1)
		}
		m.loadPreviewColors()
		return m, nil
	case tea.KeyMsg:
		if m.canSwitchPage() {
			switch msg.String() {
			case "tab", "right", "l":
				m.page = (m.page + 1) % 2
				if m.page == pageHome {
					m.startHomeIntro()
					return m, pollMetrics()
				}
				return m, nil
			case "left", "h":
				m.page = (m.page + 1) % 2
				if m.page == pageHome {
					m.startHomeIntro()
					return m, pollMetrics()
				}
				return m, nil
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

		if m.showHints {
			switch msg.String() {
			case "?", "esc", "enter":
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
			case "r":
				m.backups = loadBackups()
				if m.backupIndex >= len(m.backups) {
					m.backupIndex = max(0, len(m.backups)-1)
				}
				m.loadPreviewColors()
				m.lastStatus = "Backups refreshed"
				return m, nil
			case "enter":
				if len(m.backups) == 0 {
					m.lastStatus = "No backups to restore"
					return m, nil
				}
				id := m.backups[m.backupIndex].ID
				cmd := exec.Command(restoreScript, id)
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
			if m.settingIndex < len(m.settings())-1 {
				m.settingIndex++
			}
			return m, nil
		case "r":
			m.backups = loadBackups()
			if m.backupIndex >= len(m.backups) {
				m.backupIndex = max(0, len(m.backups)-1)
			}
			m.loadPreviewColors()
			m.lastStatus = "Backups refreshed"
			return m, nil
		case "g":
			quoted := fmt.Sprintf("%q", defaultWall)
			shellCmd := "if command -v chafa >/dev/null 2>&1; then chafa -f sixel --animate=off " + quoted +
				"; elif command -v img2sixel >/dev/null 2>&1; then img2sixel " + quoted +
				"; else echo 'Install chafa or img2sixel for sixel preview'; fi; printf '\\nPress Enter to continue...'; read _"
			cmd := exec.Command("sh", "-lc", shellCmd)
			m.lastStatus = "Launching sixel preview..."
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				if err != nil {
					return statusMsg("Sixel preview failed: " + err.Error())
				}
				return statusMsg("Sixel preview finished")
			})
		case "enter":
			return m.activateSetting()
		}
	}
	return m, nil
}

func (m model) settings() []string {
	return []string{
		"Apply Theme",
		"Update Colors",
		"Mode: " + m.mode,
		"Style Preset: " + m.stylePreset,
		"Customize Colors...",
		"Backups...",
		"Refresh Backups",
		"Quit",
	}
}

func (m model) activateSetting() (tea.Model, tea.Cmd) {
	if m.applying {
		return m, nil
	}
	switch m.settingIndex {
	case 0:
		return m.startApply("Apply", true)
	case 1:
		return m.startApply("Update colors", false)
	case 2:
		if m.mode == "dark" {
			m.mode = "light"
		} else {
			m.mode = "dark"
		}
		return m, nil
	case 3:
		m.stylePreset = nextStylePreset(m.stylePreset)
		return m, nil
	case 4:
		m.customizing = true
		m.customIndex = 0
		return m, nil
	case 5:
		m.showBackups = true
		return m, nil
	case 6:
		m.backups = loadBackups()
		if m.backupIndex >= len(m.backups) {
			m.backupIndex = max(0, len(m.backups)-1)
		}
		m.loadPreviewColors()
		m.lastStatus = "Backups refreshed"
		return m, nil
	case 7:
		return m, tea.Quit
	}
	return m, nil
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

	const outerPad = 1
	innerW := max(20, m.width-(outerPad*2))
	innerH := max(8, m.height-(outerPad*2))

	title := headerChip("Tooie", "12")
	if m.page == pageTheme {
		title = headerChip("Tooie / Theme", "12")
	}
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	if strings.Contains(strings.ToLower(m.lastStatus), "failed") {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	}
	statusText := "status: " + m.lastStatus
	if m.applying {
		statusText = m.renderApplyStatus(innerW)
	}
	status := statusText
	if !m.applying {
		status = statusStyle.Render(statusText)
	}
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("[? hints]")
	topBar := joinLR(status, hints, innerW)

	panelH := max(4, innerH-2)
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
		overlays = m.homePinnedSixelOverlays(innerW, panelH, outerPad)
	}
	rendered := lipgloss.NewStyle().Padding(outerPad, outerPad).Render(body)
	if len(overlays) > 0 {
		rendered += renderSixelOverlays(overlays)
	}
	return rendered
}

func (m model) homeHintsLine(width int) string {
	muted := blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#000000", 0.32)
	keyTab := blendHexColor(muted, m.themeRoleColor("primary", "#89b4fa"), 0.35)
	keyFont := m.themeRoleColor("primary", "#89b4fa")
	keyAnim := m.themeRoleColor("secondary", "#94e2d5")
	keyApps := m.themeRoleColor("secondary", "#94e2d5")
	keySearch := m.themeRoleColor("tertiary", "#cba6f7")
	keyRedraw := blendHexColor(m.themeRoleColor("primary", "#89b4fa"), muted, 0.18)
	keyQuit := m.themeRoleColor("error", "#f38ba8")

	styleMuted := lipgloss.NewStyle().Foreground(lipgloss.Color(muted))
	tab := lipgloss.NewStyle().Foreground(lipgloss.Color(keyTab)).Render("tab/h/l")
	font := lipgloss.NewStyle().Foreground(lipgloss.Color(keyFont)).Render("f") + styleMuted.Render("ont")
	anim := lipgloss.NewStyle().Foreground(lipgloss.Color(keyAnim)).Render("p") + styleMuted.Render("attern")
	appsText := "1-0 Apps"
	if len(m.pinnedApps) > 0 {
		appsText = fmt.Sprintf("1-%d", len(m.pinnedApps))
	}
	apps := lipgloss.NewStyle().Foreground(lipgloss.Color(keyApps)).Render(appsText) + styleMuted.Render(" Apps")
	search := lipgloss.NewStyle().Foreground(lipgloss.Color(keySearch)).Render("/") + styleMuted.Render(" search")
	redraw := lipgloss.NewStyle().Foreground(lipgloss.Color(keyRedraw)).Render("r") + styleMuted.Render(" redraw")
	quit := lipgloss.NewStyle().Foreground(lipgloss.Color(keyQuit)).Render("q") + styleMuted.Render("uit")
	sp := styleMuted.Render("    ")
	line := tab + sp + font + sp + anim + sp + apps + sp + search + sp + redraw + sp + quit
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

func (m model) renderMain(usableW, contentH int) string {
	if m.page == pageHome {
		return m.renderHomePage(usableW, contentH)
	}

	usableW = max(20, usableW)
	contentH = max(4, contentH)
	topRequired := max(8, len(m.settings())+2)
	if m.hasActiveOverlay() {
		topRequired = max(topRequired, m.interactionLineCount()+2)
	}
	detailsMin := 2
	topH := topRequired
	if contentH-topH < detailsMin {
		topH = max(3, contentH-detailsMin)
	}
	detailsH := max(detailsMin, contentH-topH)

	var topRow string
	if m.hasActiveOverlay() {
		leftW := int(float64(usableW) * 0.44)
		if leftW < 28 {
			leftW = 28
		}
		rightW := usableW - leftW
		if rightW < 20 {
			rightW = 20
			leftW = usableW - rightW
		}
		left := panelStyle(leftW, topH, "12").Render(m.settingsBlock(topH - 2))
		right := panelStyle(rightW, topH, m.interactionBorderColor()).Render(m.interactionBlock(topH - 2))
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	} else {
		topRow = panelStyle(usableW, topH, "12").Render(m.settingsBlock(topH - 2))
	}

	details := panelStyle(usableW, detailsH, "10").Render(m.detailsBlock(usableW - 4))
	return topRow + "\n" + details
}

func (m model) interactionLineCount() int {
	if m.pickerTarget != "" {
		return 10
	}
	if m.customizing {
		return 10
	}
	if m.showHints {
		return 8
	}
	if m.showBackups {
		return 10
	}
	return 4
}

func (m model) settingsBlock(limit int) string {
	lines := []string{headerChip("Settings", "12")}
	items := m.settings()
	visible := max(1, limit-1)
	start, end := listWindow(len(items), m.settingIndex, visible)
	for i := start; i < end; i++ {
		s := items[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.settingIndex {
			prefix = "▶ "
			style = style.Foreground(lipgloss.Color("11")).Bold(true)
		}
		lines = append(lines, style.Render(prefix+s))
	}
	return strings.Join(lines, "\n")
}

func (m model) detailsBlock(totalWidth int) string {
	left := []string{
		headerChip("Details", "10"),
		"",
		headerChip("Current Theme", "8"),
		"  mode: " + m.mode,
		"  style preset: " + m.stylePreset,
		"  status palette: " + m.palette,
	}

	if strings.TrimSpace(m.textColor) != "" {
		left = append(left, "  text override: "+m.textColor)
	} else {
		left = append(left, "  text override: auto")
	}
	if strings.TrimSpace(m.cursorColor) != "" {
		left = append(left, "  cursor override: "+m.cursorColor)
	} else {
		left = append(left, "  cursor override: auto")
	}

	right := []string{headerChip("Palette", "11"), ""}
	right = append(right, m.palettePreviewLines()...)

	return renderTwoColumns(left, right, totalWidth)
}

func (m model) renderHomePage(usableW, contentH int) string {
	usableW = max(28, usableW)
	contentH = max(8, contentH)

	topH := (contentH * 70) / 100
	if topH < 6 {
		topH = 6
	}
	metricW, metricH := clockGlyphMetrics(m.clockGlyphs)
	clockMinH := desiredClockPanelHeight(metricH)
	switch m.currentClockFontName() {
	case "fivebyfive", "squaresounds":
		clockMinH = max(6, clockMinH-3)
	}
	topH = max(topH, clockMinH)
	bottomH := contentH - topH
	if bottomH < 3 {
		bottomH = 3
		topH = max(5, contentH-bottomH)
	}

	leftW := desiredClockPanelWidth(usableW, topH, metricW, metricH)
	if leftW < 18 {
		leftW = 18
	}
	rightW := usableW - leftW
	if rightW < 24 {
		rightW = 24
		leftW = usableW - rightW
	}

	rowH := max(6, topH)
	clockLines := m.renderDashboardVerticalClockTest(max(1, leftW-4), max(1, rowH-2))
	clockPanel := panelStyle(leftW, rowH, "13").Render(strings.Join(clockLines, "\n"))
	sysPanel := panelStyle(rightW, rowH, "10").Render(m.homeSystemBlock(rightW-4, rowH-2))
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, clockPanel, sysPanel)
	bottomH = max(3, contentH-rowH)
	bottomRow := panelStyle(usableW, bottomH, "8").Render(m.renderHomeLauncherBlock(usableW-4, bottomH-2))
	return topRow + "\n" + bottomRow
}

func (m model) homeSystemBlock(innerW, limit int) string {
	if innerW < 20 {
		innerW = 20
	}
	lines := []string{
		"",
		m.renderUsageProgressBar(innerW, "", m.cpuViz, m.cpuBarGradientColor),
		"",
		m.renderUsageProgressBar(innerW, "", m.ramViz, m.ramBarGradientColor),
		"",
		m.renderUsageProgressBar(innerW, "󰋊", m.storViz, m.storageBarGradientColor),
		"",
		m.renderUptimeLine(innerW, m.uptimeText),
	}
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}

func (m model) canSwitchPage() bool {
	return m.pickerTarget == "" && !m.customizing && !m.showBackups && !m.showAppSearch
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
	if m.customizing {
		return m.customizeBlock(limit)
	}
	if m.showHints {
		return strings.Join([]string{
			headerChip("Hints", "13"),
			"  up/down or j/k: move",
			"  enter: select action",
			"  up/down in color picker",
			"  enter to choose color",
			"  g: sixel preview",
			"  b: backups menu",
			"  r: refresh backups",
			"  q: quit",
			"  esc or ?: close",
		}, "\n")
	}
	if m.showBackups {
		lines := []string{headerChip("Backups", "8"), "  enter=restore, r=refresh, esc=close"}
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
				if v, ok := b.Meta["status_palette"]; ok && v != "" {
					line += " [" + v + "]"
				}
				if v, ok := b.Meta["style_preset"]; ok && v != "" {
					line += " {" + v + "}"
				}
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func (m model) palettePreviewLines() []string {
	keys := []string{"primary", "secondary", "tertiary", "error", "surface", "on_surface"}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		hex := m.selectedHexes[k]
		if hex == "" {
			continue
		}
		sw := lipgloss.NewStyle().Background(lipgloss.Color(hex)).Foreground(lipgloss.Color("#000000")).Render("  ")
		out = append(out, fmt.Sprintf("  %s %s", sw, hex+" "+k))
	}
	if len(out) == 0 {
		out = append(out, "  (no matugen.json in selected backup)")
	}
	manual := []struct {
		Label string
		Hex   string
	}{
		{"text", m.textColor},
		{"cursor", m.cursorColor},
		{"ansi red", m.ansiRed},
		{"ansi green", m.ansiGreen},
		{"ansi yellow", m.ansiYellow},
		{"ansi blue", m.ansiBlue},
		{"ansi magenta", m.ansiMagenta},
		{"ansi cyan", m.ansiCyan},
	}
	anyManual := false
	for _, item := range manual {
		if strings.TrimSpace(item.Hex) == "" {
			continue
		}
		if !anyManual {
			out = append(out, "", "  manual overrides")
			anyManual = true
		}
		sw := lipgloss.NewStyle().Background(lipgloss.Color(item.Hex)).Foreground(lipgloss.Color("#000000")).Render("  ")
		out = append(out, fmt.Sprintf("  %s %s %s", sw, item.Hex, item.Label))
	}
	return out
}

func (m model) hasActiveOverlay() bool {
	return m.showHints || m.showBackups || m.pickerTarget != "" || m.customizing
}

func headerChip(text, color string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Render(text)
}

func (m model) colorPickerOptions(target string) []colorOption {
	opts := []colorOption{{Label: "Auto", Hex: ""}}
	if strings.HasPrefix(target, "ansi_") {
		family := strings.TrimPrefix(target, "ansi_")
		familyOpts := m.familyColorOptions(family)
		if len(familyOpts) > 0 {
			return append(opts, familyOpts...)
		}
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
			label = swatch
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
	return strings.ReplaceAll(role, "_", " ")
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
		{Label: "Apply Theme", Target: "apply"},
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
		case "apply", "back":
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
	case "apply":
		return m.startApply("Apply", true)
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
	args := []string{"-m", m.mode, "-w", defaultWall, "--status-palette", m.palette, "--style-preset", m.stylePreset}
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
	return args
}

func runApplyCommand(args []string, label string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(applyScript, args...)
		out, err := cmd.CombinedOutput()
		return applyDoneMsg{
			label: label,
			err:   err,
			out:   strings.TrimSpace(string(out)),
		}
	}
}

func (m model) startApply(label string, includeOverrides bool) (tea.Model, tea.Cmd) {
	if m.applying {
		return m, nil
	}
	args := m.applyArgs(includeOverrides)
	m.applying = true
	m.applyLabel = label
	m.applyProgress = 0
	m.applyVel = 0
	m.applyTarget = 0.12
	m.lastStatus = label + " in progress..."
	return m, tea.Batch(tickApply(), runApplyCommand(args, label))
}

func nextStylePreset(cur string) string {
	if len(stylePresets) == 0 {
		return cur
	}
	for i, p := range stylePresets {
		if p == cur {
			return stylePresets[(i+1)%len(stylePresets)]
		}
	}
	return stylePresets[0]
}

func renderTwoColumns(left, right []string, totalWidth int) string {
	if totalWidth < 56 {
		joined := make([]string, 0, len(left)+len(right)+2)
		joined = append(joined, left...)
		joined = append(joined, "")
		joined = append(joined, right...)
		return strings.Join(joined, "\n")
	}

	gap := "   "
	leftW := (totalWidth - len(gap)) / 2
	rightW := totalWidth - len(gap) - leftW
	if leftW < 20 || rightW < 20 {
		joined := make([]string, 0, len(left)+len(right)+2)
		joined = append(joined, left...)
		joined = append(joined, "")
		joined = append(joined, right...)
		return strings.Join(joined, "\n")
	}

	leftBlock := lipgloss.NewStyle().Width(leftW).Render(strings.Join(left, "\n"))
	rightBlock := lipgloss.NewStyle().Width(rightW).Render(strings.Join(right, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, gap, rightBlock)
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

	// 4 invisible quadrants:
	// q1 top-left  -> bottom-right
	// q2 top-right -> bottom-left
	// q3 bottom-left -> top-right
	// q4 bottom-right -> top-left
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
			filledRunes[i] = '░'
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
	p := m.clockPalette()
	warmA := blendHexColor(p[len(p)-1], p[3], 0.55)
	warmB := p[len(p)-1]
	return gradientFromStops(t, []string{
		blendHexColor(warmA, "#f6d365", 0.42),
		warmA,
		blendHexColor(warmA, warmB, 0.68),
		blendHexColor(warmB, "#7a0000", 0.25),
	})
}

func (m model) ramBarGradientColor(t float64) string {
	pal := m.clockPalette()
	coolA := pal[2]
	coolB := pal[1]
	coolC := blendHexColor(coolB, "#3ba7ff", 0.4)
	return gradientFromStops(t, []string{
		blendHexColor(coolA, "#bff8ff", 0.25),
		blendHexColor(coolA, coolB, 0.55),
		coolC,
		blendHexColor(coolC, "#1f4fff", 0.20),
	})
}

func (m model) storageBarGradientColor(t float64) string {
	pal := m.clockPalette()
	muted := m.themeRoleColor("on_surface", "#565f89")
	storeA := blendHexColor(muted, pal[3], 0.35)
	storeB := pal[3]
	storeC := blendHexColor(pal[3], pal[2], 0.22)
	return gradientFromStops(t, []string{
		blendHexColor(muted, "#000000", 0.15),
		storeA,
		blendHexColor(storeA, storeB, 0.65),
		blendHexColor(storeC, "#ffffff", 0.08),
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
	endpointData, err := os.ReadFile(filepath.Join(home, ".tooie", "endpoint"))
	if err != nil {
		return "", "", false
	}
	tokenData, err := os.ReadFile(filepath.Join(home, ".tooie", "token"))
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

func hasClockTXTGlyphSet(dir string) bool {
	required := []string{"0.txt", "1.txt", "2.txt", "3.txt", "4.txt", "5.txt", "6.txt", "7.txt", "8.txt", "9.txt", "colon.txt"}
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
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

func (m *model) loadPreviewColors() {
	m.selectedHexes = map[string]string{}
	if len(m.backups) == 0 {
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
	barW := 16
	if totalWidth > 84 {
		barW = 22
	}
	if totalWidth > 108 {
		barW = 28
	}
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
	done := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(strings.Repeat("█", filled))
	todo := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("░", barW-filled))
	percent := int(math.Round(p * 100))
	if percent > 99 && m.applying {
		percent = 99
	}
	return fmt.Sprintf("status: %s [%s%s] %d%%", m.applyLabel, done, todo, percent)
}

func loadBackups() []backup {
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	out := make([]backup, 0, len(ids))
	for _, id := range ids {
		out = append(out, backup{
			ID:   id,
			Meta: readMeta(filepath.Join(backupRoot, id, "meta.env")),
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
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithFPS(60))
	_, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tooie error: %v\n", err)
		os.Exit(1)
	}
}
