package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxPinnedApps = 7
const pinnedSixelColAdjust = -3
const pinnedSixelRowAdjust = 1

type pinnedRowLayout struct {
	pinnedStart int
	slotWidths  []int
	iconCellW   []int
	rowH        int
	iconWidth   int
	iconHeight  int
	iconTopPad  int
}

func loadHomeAppsCmd(refresh bool) tea.Cmd {
	return func() tea.Msg {
		apps, _, err := getLaunchableApps(defaultAppsCacheTTL, refresh)
		return homeAppsLoadedMsg{apps: apps, err: err}
	}
}

func launchHomeAppCmd(app launchableApp) tea.Cmd {
	return func() tea.Msg {
		err := launchComponent(app.ComponentName)
		label := app.Label
		if strings.TrimSpace(label) == "" {
			label = app.PackageName
		}
		return homeLaunchDoneMsg{label: label, err: err}
	}
}

func warmPinnedIconsCmd(pkgs []string) tea.Cmd {
	pkgs = normalizePinnedPackages(pkgs)
	if len(pkgs) == 0 {
		return nil
	}
	return func() tea.Msg {
		for _, pkg := range pkgs {
			_, _ = ensureAppIconCached(pkg)
		}
		return homeIconsWarmedMsg{packages: pkgs}
	}
}

func (m model) homeRedrawCmd(refreshMetrics bool) tea.Cmd {
	cmds := []tea.Cmd{
		tea.ClearScreen,
		loadHomeAppsCmd(false),
		warmPinnedIconsCmd(m.pinnedPackages),
		loadSystemInfoCmd(),
	}
	if refreshMetrics && !m.metricsPaused {
		cmds = append(cmds, pollMetrics())
	}
	return tea.Batch(cmds...)
}

func loadPinnedApps() []string {
	path, err := pinnedAppsPath()
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var pkgs []string
	if err := json.Unmarshal(raw, &pkgs); err != nil {
		return nil
	}
	return normalizePinnedPackages(pkgs)
}

func savePinnedApps(pkgs []string) {
	path, err := pinnedAppsPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	raw, err := json.MarshalIndent(normalizePinnedPackages(pkgs), "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func pinnedAppsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("unable to resolve home directory")
	}
	return filepath.Join(home, ".config", "tooie", "pinned-apps.json"), nil
}

func normalizePinnedPackages(pkgs []string) []string {
	out := make([]string, 0, len(pkgs))
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		out = append(out, pkg)
		if len(out) >= maxPinnedApps {
			break
		}
	}
	return out
}

func defaultPinnedPackages(apps []launchableApp) []string {
	preferred := []string{
		"com.termux",
		"com.openai.chatgpt",
		"com.android.chrome",
		"com.google.android.apps.maps",
		"com.android.settings",
		"moe.shizuku.privileged.api",
		"com.whatsapp",
	}
	byPkg := make(map[string]launchableApp, len(apps))
	for _, app := range apps {
		byPkg[app.PackageName] = app
	}
	out := make([]string, 0, maxPinnedApps)
	for _, pkg := range preferred {
		if _, ok := byPkg[pkg]; ok {
			out = append(out, pkg)
		}
		if len(out) >= maxPinnedApps {
			return out
		}
	}
	for _, app := range apps {
		if app.SystemApp {
			continue
		}
		out = append(out, app.PackageName)
		if len(out) >= maxPinnedApps {
			break
		}
	}
	return normalizePinnedPackages(out)
}

func (m *model) refreshPinnedApps() {
	byPkg := make(map[string]launchableApp, len(m.apps))
	for _, app := range m.apps {
		byPkg[app.PackageName] = app
	}
	out := make([]launchableApp, 0, len(m.pinnedPackages))
	for _, pkg := range normalizePinnedPackages(m.pinnedPackages) {
		if app, ok := byPkg[pkg]; ok {
			out = append(out, app)
		}
	}
	m.pinnedApps = out
	if m.pinnedIndex >= len(m.pinnedApps) {
		m.pinnedIndex = max(0, len(m.pinnedApps)-1)
	}
	for i := range m.pinnedApps {
		if path, ok := cachedIconPath(m.pinnedApps[i].PackageName); ok {
			m.pinnedApps[i].IconCachePath = path
		}
	}
}

func (m *model) refreshAppSearchResults() {
	query := strings.TrimSpace(strings.ToLower(m.appSearchQuery))
	if len(m.apps) == 0 {
		m.appSearchResults = nil
		m.appSearchIndex = 0
		return
	}
	results := make([]launchableApp, 0, len(m.apps))
	for _, app := range m.apps {
		if path, ok := cachedIconPath(app.PackageName); ok {
			app.IconCachePath = path
		}
		label := strings.ToLower(strings.TrimSpace(app.Label))
		pkg := strings.ToLower(strings.TrimSpace(app.PackageName))
		component := strings.ToLower(strings.TrimSpace(app.ComponentName))
		if query == "" || strings.Contains(label, query) || strings.Contains(pkg, query) || strings.Contains(component, query) {
			results = append(results, app)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(results[i].Label))
		lj := strings.ToLower(strings.TrimSpace(results[j].Label))
		if li == lj {
			return results[i].PackageName < results[j].PackageName
		}
		return li < lj
	})
	m.appSearchResults = results
	if m.appSearchIndex >= len(results) {
		m.appSearchIndex = max(0, len(results)-1)
	}
}

func (m *model) togglePinnedApp(pkg string) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return
	}
	next := make([]string, 0, maxPinnedApps)
	found := false
	for _, cur := range m.pinnedPackages {
		if cur == pkg {
			found = true
			continue
		}
		next = append(next, cur)
	}
	if !found {
		next = append([]string{pkg}, next...)
	}
	m.pinnedPackages = normalizePinnedPackages(next)
	savePinnedApps(m.pinnedPackages)
	m.refreshPinnedApps()
}

func (m model) currentSearchApp() (launchableApp, bool) {
	if m.appSearchIndex < 0 || m.appSearchIndex >= len(m.appSearchResults) {
		return launchableApp{}, false
	}
	return m.appSearchResults[m.appSearchIndex], true
}

func (m model) currentPinnedApp() (launchableApp, bool) {
	if m.pinnedIndex < 0 || m.pinnedIndex >= len(m.pinnedApps) {
		return launchableApp{}, false
	}
	return m.pinnedApps[m.pinnedIndex], true
}

func (m model) updateHomePage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showAppSearch {
		switch msg.Type {
		case tea.KeyRunes:
			switch msg.String() {
			case "/":
				return m, nil
			default:
				m.appSearchQuery += msg.String()
				m.refreshAppSearchResults()
				return m, nil
			}
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.appSearchQuery) > 0 {
				runes := []rune(m.appSearchQuery)
				m.appSearchQuery = string(runes[:len(runes)-1])
				m.refreshAppSearchResults()
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.showAppSearch = false
			m.appSearchQuery = ""
			m.refreshAppSearchResults()
			return m, nil
		case "up", "k":
			if m.appSearchIndex > 0 {
				m.appSearchIndex--
			}
			return m, nil
		case "down", "j":
			if m.appSearchIndex < len(m.appSearchResults)-1 {
				m.appSearchIndex++
			}
			return m, nil
		case "enter":
			app, ok := m.currentSearchApp()
			if !ok {
				return m, nil
			}
			m.lastStatus = "Launching " + app.Label + "..."
			return m, launchHomeAppCmd(app)
		case "ctrl+p":
			app, ok := m.currentSearchApp()
			if !ok {
				return m, nil
			}
			m.togglePinnedApp(app.PackageName)
			m.refreshAppSearchResults()
			m.lastStatus = "Updated pins"
			return m, warmPinnedIconsCmd(m.pinnedPackages)
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHints = !m.showHints
		return m, nil
	case "esc":
		m.showHints = false
		return m, nil
	case "f":
		m.cycleClockFont()
		return m, nil
	case "a":
		m.cycleClockPattern()
		return m, nil
	case "p":
		m.metricsPaused = !m.metricsPaused
		if m.metricsPaused {
			m.lastStatus = "System stats polling paused"
			m.showHomeNotice("system stats: paused", "poll")
			return m, nil
		}
		m.lastStatus = "System stats polling resumed"
		m.showHomeNotice("system stats: unpaused", "poll")
		return m, tea.Batch(pollMetrics(), tickMetrics())
	case "/":
		m.showAppSearch = true
		m.appSearchQuery = ""
		m.refreshAppSearchResults()
		return m, nil
	case "left", "h":
		if m.pinnedIndex > 0 {
			m.pinnedIndex--
		}
		return m, nil
	case "right", "l":
		if m.pinnedIndex < len(m.pinnedApps)-1 {
			m.pinnedIndex++
		}
		return m, nil
	case "r":
		clearPinnedSixelCache()
		m.lastStatus = "Redrawing..."
		return m, m.homeRedrawCmd(true)
	case "enter":
		app, ok := m.currentPinnedApp()
		if !ok {
			m.showAppSearch = true
			m.refreshAppSearchResults()
			return m, nil
		}
		m.lastStatus = "Launching " + app.Label + "..."
		return m, launchHomeAppCmd(app)
	}

	if len(msg.String()) == 1 {
		ch := msg.String()[0]
		if ch >= '1' && ch <= '9' {
			idx := int(ch - '1')
			if idx >= 0 && idx < len(m.pinnedApps) {
				app := m.pinnedApps[idx]
				m.lastStatus = "Launching " + app.Label + "..."
				return m, launchHomeAppCmd(app)
			}
		}
	}
	return m, nil
}

func (m model) renderHomeLauncherBlock(innerW, innerH int) string {
	innerW = max(24, innerW)
	innerH = max(3, innerH)

	if m.showAppSearch {
		return m.renderSearchPopup(innerW, innerH)
	}

	pinned := m.renderPinnedAppsStrip(innerW, innerH)
	if len(pinned) > innerH {
		pinned = pinned[:innerH]
	}

	lines := make([]string, 0, innerH)
	blank := strings.Repeat(" ", innerW)
	lines = append(lines, pinned...)
	for len(lines) < innerH {
		lines = append(lines, blank)
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	return strings.Join(lines, "\n")
}

func (m model) computePinnedRowLayout(innerW, innerH int) pinnedRowLayout {
	slotWidths := distributedSlotWidths(innerW, len(m.pinnedApps), 8)
	rowH := max(4, min(innerH, 5))
	pinnedStart := 0
	iconCellW := make([]int, len(slotWidths))
	minCellW := 5
	iconWidth := 5
	iconHeight := 3
	iconTopPad := 2
	for i, slotW := range slotWidths {
		cellW := max(minCellW, slotW-2)
		maxAllowed := max(minCellW, slotW-1)
		if cellW > maxAllowed {
			cellW = maxAllowed
		}
		iconCellW[i] = cellW
	}
	if len(iconCellW) > 0 {
		narrowest := iconCellW[0]
		for _, w := range iconCellW[1:] {
			if w < narrowest {
				narrowest = w
			}
		}
		iconWidth = max(4, narrowest-1)
		if iconWidth > 7 {
			iconWidth = 7
		}
	}
	return pinnedRowLayout{
		pinnedStart: pinnedStart,
		slotWidths:  slotWidths,
		iconCellW:   iconCellW,
		rowH:        rowH,
		iconWidth:   iconWidth,
		iconHeight:  iconHeight,
		iconTopPad:  iconTopPad,
	}
}

func (m model) renderPinnedAppsStrip(innerW, innerH int) []string {
	if len(m.pinnedApps) == 0 {
		empty := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  no pinned apps yet")
		return []string{empty, empty, empty}
	}
	layout := m.computePinnedRowLayout(innerW, innerH)
	slotWidths := layout.slotWidths
	cellWs := layout.iconCellW
	useSixel := m.canUseSixelPinnedIcons()
	lines := make([]string, layout.rowH)
	for i, app := range m.pinnedApps {
		slotW := slotWidths[i]
		cellW := cellWs[i]
		if cellW < 4 {
			cellW = 4
		}
		bodyRows := make([]string, layout.rowH)
		bodyInnerW := cellW
		iconLines := []string{"", ""}
		if useSixel && strings.TrimSpace(app.IconCachePath) != "" {
			blank := lipgloss.NewStyle().Width(layout.iconWidth).Render(strings.Repeat(" ", layout.iconWidth))
			iconLines = []string{blank, blank}
		} else {
			iconLines = strings.Split(renderPinnedAppIcon(app, layout.iconWidth, layout.iconHeight), "\n")
		}
		for len(iconLines) < layout.iconHeight {
			iconLines = append(iconLines, "")
		}
		for row := 0; row < layout.iconHeight && layout.iconTopPad+row < layout.rowH; row++ {
			bodyRows[layout.iconTopPad+row] = lipgloss.NewStyle().
				Width(bodyInnerW).
				Align(lipgloss.Center).
				Render(iconLines[row])
		}
		for row := 0; row < len(bodyRows); row++ {
			if bodyRows[row] == "" {
				bodyRows[row] = strings.Repeat(" ", bodyInnerW)
			}
		}
		slotPadLeft := max(0, (slotW-cellW)/2)
		slotPadRight := max(0, slotW-cellW-slotPadLeft)
		for row := 0; row < layout.rowH; row++ {
			lines[row] += strings.Repeat(" ", slotPadLeft) + bodyRows[row] + strings.Repeat(" ", slotPadRight)
		}
	}
	return lines
}

func distributedSlotWidths(totalWidth, count, minWidth int) []int {
	if count <= 0 {
		return nil
	}
	if minWidth < 1 {
		minWidth = 1
	}
	widths := make([]int, count)
	base := totalWidth / count
	if base < minWidth {
		base = minWidth
	}
	for i := range widths {
		widths[i] = base
	}
	used := base * count
	remaining := totalWidth - used
	for i := 0; i < count && remaining > 0; i++ {
		widths[i]++
		remaining--
	}
	return widths
}

func (m model) renderSearchBarLines(innerW int) []string {
	prompt := "Search apps..."
	if m.showAppSearch {
		prompt = "/" + m.appSearchQuery
		if prompt == "/" {
			prompt = "/"
		}
	}
	color := lipgloss.Color("8")
	if m.showAppSearch {
		color = lipgloss.Color("12")
	}
	style := lipgloss.NewStyle().
		Foreground(color).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(1, 1).
		Width(max(12, innerW-2))
	return strings.Split(style.Render(prompt), "\n")
}

func (m model) renderSearchPromptLine(width int) string {
	prompt := "/" + m.appSearchQuery
	if prompt == "/" {
		prompt = "/"
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Width(width)
	return style.Render(cutPad(prompt, width))
}

func (m model) renderSearchPopup(innerW, innerH int) string {
	popupW := max(28, min(innerW-4, (innerW*2)/3))
	prompt := m.renderSearchPromptLine(popupW)
	resultsAvail := max(2, innerH-3)
	results := m.renderSearchResults(popupW, resultsAvail)
	lines := []string{prompt}
	lines = append(lines, results...)
	popupH := min(innerH, len(lines)+2)
	if popupH < 4 {
		popupH = 4
	}
	bodyLimit := max(1, popupH-2)
	if len(lines) > bodyLimit {
		lines = lines[:bodyLimit]
	}
	for len(lines) < bodyLimit {
		lines = append(lines, strings.Repeat(" ", popupW))
	}
	popup := framedPanel(popupW+2, popupH, m.themeRoleColor("primary", "#89b4fa"), strings.Join(lines, "\n"), "Search", "left", "", "left")
	popupLines := strings.Split(popup, "\n")
	out := make([]string, 0, innerH)
	topPad := max(0, (innerH-len(popupLines))/2)
	blank := strings.Repeat(" ", innerW)
	for i := 0; i < topPad; i++ {
		out = append(out, blank)
	}
	for _, line := range popupLines {
		out = append(out, placeCenterStyled(line, innerW))
	}
	for len(out) < innerH {
		out = append(out, blank)
	}
	if len(out) > innerH {
		out = out[:innerH]
	}
	return strings.Join(out, "\n")
}

func (m model) renderSearchResults(innerW, limit int) []string {
	if m.appLoadErr != nil {
		return []string{lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("  " + m.appLoadErr.Error())}
	}
	if !m.appsLoaded {
		return []string{lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  loading apps...")}
	}
	if len(m.appSearchResults) == 0 {
		return []string{lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  no matches")}
	}
	visible := max(1, limit)
	start, end := listWindow(len(m.appSearchResults), m.appSearchIndex, visible)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		app := m.appSearchResults[i]
		prefix := "  "
		style := lipgloss.NewStyle().Width(innerW)
		if i == m.appSearchIndex {
			prefix = "▶ "
			style = style.Foreground(lipgloss.Color("11")).Bold(true)
		} else {
			style = style.Foreground(lipgloss.Color("7"))
		}
		pin := " "
		for _, pkg := range m.pinnedPackages {
			if pkg == app.PackageName {
				pin = "●"
				break
			}
		}
		text := fmt.Sprintf("%s%s %s", prefix, pin, app.Label)
		lines = append(lines, style.Render(clip(text, innerW)))
	}
	return lines
}

func (m model) canUseSixelPinnedIcons() bool {
	if len(m.pinnedApps) == 0 {
		return false
	}
	if _, ok := sixelCellGeometry(); !ok {
		return false
	}
	for _, app := range m.pinnedApps {
		if strings.TrimSpace(app.IconCachePath) != "" {
			return true
		}
	}
	return false
}

func (m model) homePinnedSixelOverlays(totalInnerW, panelH, outerPad int) []sixelOverlay {
	if m.showAppSearch || !m.canUseSixelPinnedIcons() {
		return nil
	}
	geom, ok := sixelCellGeometry()
	if !ok {
		return nil
	}

	contentH := max(8, panelH)
	topH := (contentH * 70) / 100
	if topH < 6 {
		topH = 6
	}
	_, metricH := clockGlyphMetrics(m.clockGlyphs)
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
	rowH := max(6, topH)
	bottomPanelTopBodyRow := 2 + rowH
	bottomPanelInnerTopBodyRow := bottomPanelTopBodyRow + 1
	bottomPanelInnerLeftBodyCol := 4

	layout := m.computePinnedRowLayout(totalInnerW, bottomH-2)
	overlays := make([]sixelOverlay, 0, len(m.pinnedApps))
	x := bottomPanelInnerLeftBodyCol
	for i, app := range m.pinnedApps {
		if strings.TrimSpace(app.IconCachePath) == "" {
			x += layout.slotWidths[i]
			continue
		}
		res, ok := renderPinnedAppSixel(app, layout.iconWidth, layout.iconHeight, geom)
		if !ok || strings.TrimSpace(res.data) == "" {
			x += layout.slotWidths[i]
			continue
		}
		slotW := layout.slotWidths[i]
		cellW := layout.iconCellW[i]
		cellLeftPad := max(0, (slotW-cellW)/2)
		sixelWidthCells := max(1, pixelToCellCeil(res.widthPx, geom.width))
		sixelHeightCells := max(1, pixelToCellCeil(res.heightPx, geom.height))
		offsetX := cellLeftPad + max(0, (cellW-sixelWidthCells)/2)
		if offsetX < 0 {
			offsetX = 0
		}
		offsetY := layout.iconTopPad
		if layout.iconHeight > sixelHeightCells {
			offsetY += (layout.iconHeight - sixelHeightCells) / 2
		}
		if offsetY < 0 {
			offsetY = 0
		}
		overlays = append(overlays, sixelOverlay{
			row:  max(1, outerPad+bottomPanelInnerTopBodyRow+layout.pinnedStart+offsetY+pinnedSixelRowAdjust),
			col:  max(1, outerPad+x+offsetX+pinnedSixelColAdjust),
			data: res.data,
		})
		x += layout.slotWidths[i]
	}
	return overlays
}
