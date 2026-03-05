package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	homeDir        = "/data/data/com.termux/files/home"
	themeDir       = homeDir + "/files/theme"
	backupRoot     = themeDir + "/backups"
	applyScript    = themeDir + "/apply-material.sh"
	restoreScript  = themeDir + "/restore-material.sh"
	defaultWall    = homeDir + "/.termux/background/background.jpeg"
	defaultMode    = "dark"
	defaultPalette = "default"
)

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
	width         int
	height        int
	backups       []backup
	backupIndex   int
	settingIndex  int
	customIndex   int
	mode          string
	palette       string
	textColor     string
	cursorColor   string
	ansiRed       string
	ansiGreen     string
	ansiYellow    string
	ansiBlue      string
	ansiMagenta   string
	ansiCyan      string
	lastStatus    string
	pickerTarget  string
	pickerIndex   int
	customizing   bool
	showHints     bool
	showBackups   bool
	selectedHexes map[string]string
}

func initialModel() model {
	bs := loadBackups()
	m := model{
		backups:     bs,
		mode:        defaultMode,
		palette:     defaultPalette,
		lastStatus:  "Ready",
		textColor:   "",
		cursorColor: "",
		ansiRed:     "",
		ansiGreen:   "",
		ansiYellow:  "",
		ansiBlue:    "",
		ansiMagenta: "",
		ansiCyan:    "",
	}
	m.loadPreviewColors()
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
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
		"Refresh Theme",
		"Mode: " + m.mode,
		"Customize Colors...",
		"Backups...",
		"Refresh Backups",
		"Quit",
	}
}

func (m model) activateSetting() (tea.Model, tea.Cmd) {
	switch m.settingIndex {
	case 0:
		args := m.applyArgs(true)
		cmd := exec.Command(applyScript, args...)
		m.lastStatus = "Running apply script..."
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return statusMsg("Apply failed: " + err.Error())
			}
			return statusMsg("Apply completed")
		})
	case 1:
		args := m.applyArgs(false)
		cmd := exec.Command(applyScript, args...)
		m.lastStatus = "Refreshing theme..."
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return statusMsg("Refresh failed: " + err.Error())
			}
			return statusMsg("Refresh completed")
		})
	case 2:
		if m.mode == "dark" {
			m.mode = "light"
		} else {
			m.mode = "dark"
		}
		return m, nil
	case 3:
		m.customizing = true
		m.customIndex = 0
		return m, nil
	case 4:
		m.showBackups = true
		return m, nil
	case 5:
		m.backups = loadBackups()
		if m.backupIndex >= len(m.backups) {
			m.backupIndex = max(0, len(m.backups)-1)
		}
		m.loadPreviewColors()
		m.lastStatus = "Backups refreshed"
		return m, nil
	case 6:
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

	title := headerChip("Theme Manager", "12")
	main := m.renderMain()

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	if strings.Contains(strings.ToLower(m.lastStatus), "failed") {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	}
	status := statusStyle.Render("status: " + m.lastStatus)
	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("[? hints]")
	footer := joinLR(hints, status, m.width)

	return fmt.Sprintf("%s\n\n%s\n\n%s", title, main, footer)
}

func (m model) renderMain() string {
	contentH := max(8, m.height-6)
	usableW := max(48, m.width-2)
	topRequired := max(8, len(m.settings())+2)
	if m.hasActiveOverlay() {
		topRequired = max(topRequired, m.interactionLineCount()+2)
	}
	detailsMin := 4
	topH := topRequired
	if contentH-topH < detailsMin {
		topH = max(6, contentH-detailsMin)
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
		args := m.applyArgs(true)
		cmd := exec.Command(applyScript, args...)
		m.lastStatus = "Applying customized theme..."
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return statusMsg("Apply failed: " + err.Error())
			}
			return statusMsg("Apply completed")
		})
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
	args := []string{"-m", m.mode, "-w", defaultWall, "--status-palette", m.palette}
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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "theme-manager error: %v\n", err)
		os.Exit(1)
	}
}
