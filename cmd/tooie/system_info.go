package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/host"
)

type systemInfo struct {
	User     string
	OS       string
	Kernel   string
	Arch     string
	Terminal string
	Shell    string
	Device   string
	CPU      string
	GPU      string
}

type systemInfoMsg struct {
	info systemInfo
}

type systemInfoRow struct {
	Icon     string
	Label    string
	Value    string
	Priority int
}

func loadSystemInfoCmd() tea.Cmd {
	return func() tea.Msg {
		return systemInfoMsg{info: collectSystemInfo()}
	}
}

func collectSystemInfo() systemInfo {
	info := systemInfo{
		User:     detectUserName(),
		OS:       detectAndroidVersion(),
		Kernel:   detectKernelVersion(),
		Arch:     detectArch(),
		Terminal: detectTerminalName(),
		Shell:    detectShellName(),
		Device:   detectDeviceName(),
		CPU:      detectCPUSummary(),
		GPU:      detectGPUSummary(),
	}
	if strings.TrimSpace(info.User) == "" {
		info.User = "--"
	}
	return info
}

func detectUserName() string {
	for _, key := range []string{"USER", "LOGNAME"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	if out := runCommandText("id", "-un"); out != "" {
		return out
	}
	return "--"
}

func detectAndroidVersion() string {
	release := getpropValue("ro.build.version.release")
	if release == "" {
		if hi, err := host.Info(); err == nil {
			release = strings.TrimSpace(hi.PlatformVersion)
		}
	}
	if release == "" {
		return "Android"
	}
	return "Android " + release
}

func detectKernelVersion() string {
	if hi, err := host.Info(); err == nil {
		if v := strings.TrimSpace(hi.KernelVersion); v != "" {
			return simplifyKernelVersion(v)
		}
	}
	if out := runCommandText("uname", "-r"); out != "" {
		return simplifyKernelVersion(out)
	}
	return "--"
}

func detectArch() string {
	if hi, err := host.Info(); err == nil {
		if arch := normalizeArchName(hi.KernelArch); arch != "" {
			return arch
		}
	}
	return normalizeArchName(runtime.GOARCH)
}

func normalizeArchName(arch string) string {
	switch strings.TrimSpace(strings.ToLower(arch)) {
	case "arm64":
		return "aarch64"
	case "amd64":
		return "x86_64"
	default:
		return strings.TrimSpace(arch)
	}
}

func detectTerminalName() string {
	if strings.TrimSpace(os.Getenv("TMUX")) != "" {
		if out := runCommandText("tmux", "-V"); out != "" {
			return out
		}
		return "tmux"
	}
	program := strings.TrimSpace(os.Getenv("TERM_PROGRAM"))
	version := strings.TrimSpace(os.Getenv("TERM_PROGRAM_VERSION"))
	if program != "" && version != "" {
		return program + " " + version
	}
	if program != "" {
		return program
	}
	if term := strings.TrimSpace(os.Getenv("TERM")); term != "" {
		return term
	}
	return "--"
}

func detectShellName() string {
	shellPath := strings.TrimSpace(os.Getenv("SHELL"))
	base := filepath.Base(shellPath)
	if base == "." || base == string(filepath.Separator) {
		base = ""
	}
	if base == "" {
		base = "shell"
	}
	if shellPath != "" {
		if out := runCommandText(shellPath, "--version"); out != "" {
			return sanitizeShellVersion(base, out)
		}
	}
	return base
}

func sanitizeShellVersion(name, versionText string) string {
	line := firstLine(versionText)
	if line == "" {
		return name
	}
	line = strings.ReplaceAll(line, ",", "")
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "version "):
		idx := strings.Index(lower, "version ")
		return name + " " + strings.TrimSpace(line[idx+len("version "):])
	case strings.HasPrefix(lower, strings.ToLower(name)+" "):
		return line
	default:
		return name + " " + line
	}
}

func detectDeviceName() string {
	brand := cleanDeviceToken(getpropValue("ro.product.brand"))
	model := cleanDeviceToken(getpropValue("ro.product.model"))
	if brand == "" && model == "" {
		return "--"
	}
	if brand == "" {
		return model
	}
	if model == "" {
		return brand
	}
	if strings.HasPrefix(strings.ToLower(model), strings.ToLower(brand)+" ") {
		return model
	}
	return brand + " " + model
}

func cleanDeviceToken(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return strings.ToUpper(v)
	}
	return strings.Title(strings.ToLower(v))
}

func detectCPUSummary() string {
	parts := make([]string, 0, 2)
	if maker := cleanCPUToken(getpropValue("ro.soc.manufacturer")); maker != "" {
		parts = append(parts, maker)
	}
	if model := cleanCPUToken(getpropValue("ro.soc.model")); model != "" {
		parts = append(parts, model)
	}
	name := strings.TrimSpace(strings.Join(parts, " "))
	if name == "" {
		name = strings.TrimSpace(getpropValue("ro.board.platform"))
	}
	if name == "" {
		name = "CPU"
	}
	cores := runtime.NumCPU()
	freq := detectCPUFreqGHz()
	if freq != "" {
		return fmt.Sprintf("%s (%d) @ %s", name, cores, freq)
	}
	return fmt.Sprintf("%s (%d)", name, cores)
}

func cleanCPUToken(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	switch strings.ToLower(v) {
	case "qti":
		return "Qualcomm"
	default:
		return v
	}
}

func detectCPUFreqGHz() string {
	paths := []string{
		"/sys/devices/system/cpu/cpu7/cpufreq/cpuinfo_max_freq",
		"/sys/devices/system/cpu/cpu7/cpufreq/scaling_max_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq",
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq",
	}
	var bestKHz int64
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		v, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
		if err != nil || v <= 0 {
			continue
		}
		if v > bestKHz {
			bestKHz = v
		}
	}
	if bestKHz <= 0 {
		return ""
	}
	return fmt.Sprintf("%.2f GHz", float64(bestKHz)/1_000_000.0)
}

func detectGPUSummary() string {
	for _, path := range []string{
		"/sys/class/kgsl/kgsl-3d0/gpu_model",
		"/sys/class/kgsl/kgsl-3d0/device/gpu_model",
		"/sys/devices/platform/kgsl-3d0.0/kgsl/kgsl-3d0/gpu_model",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if v := strings.TrimSpace(string(raw)); v != "" {
			return tidyGPUName(v)
		}
	}
	if vulkan := cleanCPUToken(getpropValue("ro.hardware.vulkan")); vulkan != "" {
		return tidyGPUName(vulkan)
	}
	return "--"
}

func tidyGPUName(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "--"
	}
	if strings.Contains(strings.ToLower(v), "adreno") {
		return strings.ReplaceAll(v, "Adreno730", "Adreno 730")
	}
	return v
}

func getpropValue(key string) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	return runCommandText("getprop", key)
}

func runCommandText(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func formatDetailedUptime(seconds uint64) string {
	if seconds == 0 {
		return "--"
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, pluralize(int(days), "day"))
	}
	if hours > 0 {
		parts = append(parts, pluralize(int(hours), "hour"))
	}
	if len(parts) == 0 {
		parts = append(parts, "less than 1 hour")
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, ", ")
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

var kernelVersionPattern = regexp.MustCompile(`\d+(?:\.\d+){1,3}`)

func simplifyKernelVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "--"
	}
	if match := kernelVersionPattern.FindString(v); match != "" {
		return match
	}
	return v
}

func truncateText(s string, width int) string {
	s = strings.TrimSpace(s)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	cur := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cur+rw+1 > width {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	out := strings.TrimRight(b.String(), " ")
	if out == "" {
		return "…"
	}
	return out + "…"
}

func (m model) systemInfoRows() []systemInfoRow {
	return []systemInfoRow{
		{Icon: "", Label: "User", Value: m.systemInfo.User, Priority: 0},
		{Icon: "󱦟", Label: "Uptime", Value: formatDetailedUptime(m.uptimeSeconds), Priority: 0},
		{Icon: "", Label: "OS", Value: m.systemInfo.OS, Priority: 0},
		{Icon: "", Label: "Kernel", Value: m.systemInfo.Kernel, Priority: 1},
		{Icon: "󰻠", Label: "Arch", Value: m.systemInfo.Arch, Priority: 2},
		{Icon: "", Label: "Terminal", Value: m.systemInfo.Terminal, Priority: 2},
		{Icon: "", Label: "Shell", Value: m.systemInfo.Shell, Priority: 2},
		{Icon: "", Label: "Device", Value: m.systemInfo.Device, Priority: 1},
		{Icon: "", Label: "CPU", Value: m.systemInfo.CPU, Priority: 3},
		{Icon: "󰢮", Label: "GPU", Value: m.systemInfo.GPU, Priority: 3},
	}
}

func (m model) renderSystemInfoRow(width int, row systemInfoRow, labelW int) string {
	iconColor := m.themeRoleColor("primary", "#89b4fa")
	labelColor := blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#ffffff", 0.14)
	sepColor := m.themeRoleColor("outline", "#565f89")
	valueColor := m.themeRoleColor("on_surface", "#cdd6f4")

	icon := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor)).Render(row.Icon)
	label := lipgloss.NewStyle().Foreground(lipgloss.Color(labelColor)).Render(cutPad(row.Label, labelW))
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(sepColor)).Render("")
	prefix := " " + icon + " " + label + " " + sep + " "
	valueW := max(1, width-lipgloss.Width(prefix))
	value := lipgloss.NewStyle().
		Foreground(lipgloss.Color(valueColor)).
		Render(truncateText(row.Value, valueW))
	return prefix + value
}

func (m model) renderSystemMetric(width int, icon, label, summary string, percent float64, gradientFn func(float64) string) []string {
	titleColor := blendHexColor(m.themeRoleColor("on_surface", "#7f849c"), "#ffffff", 0.14)
	iconColor := m.themeRoleColor("secondary", "#94e2d5")
	borderColor := blendHexColor(iconColor, m.themeRoleColor("outline", "#565f89"), 0.32)
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(iconColor)).Render(icon) + " " +
		lipgloss.NewStyle().Foreground(lipgloss.Color(titleColor)).Render(label)
	body := m.renderSystemMetricBar(max(1, width-4), percent, summary, gradientFn)
	return strings.Split(framedPanel(width, 3, borderColor, body, title, "left", "", "left"), "\n")
}

func (m model) systemInfoFooter(width int) string {
	top, bottom := m.renderFooterPills(width)
	_ = bottom
	return placeCenterStyled(top, width)
}

func (m model) renderFooterPills(width int) (string, string) {
	if width < 8 {
		return "", ""
	}
	palette := []string{
		m.themeRoleColor("primary", "#89b4fa"),
		m.themeRoleColor("secondary", "#94e2d5"),
		m.themeRoleColor("tertiary", "#cba6f7"),
		blendHexColor(m.themeRoleColor("primary", "#89b4fa"), m.themeRoleColor("secondary", "#94e2d5"), 0.50),
		blendHexColor(m.themeRoleColor("secondary", "#94e2d5"), m.themeRoleColor("tertiary", "#cba6f7"), 0.50),
		m.themeRoleColor("error", "#f38ba8"),
	}
	total := max(6, width-2)
	topRunes := make([]rune, total)
	bottomRunes := make([]rune, total)
	colors := make([]string, total)
	for i := 0; i < total; i++ {
		topRunes[i] = ' '
		bottomRunes[i] = ' '
	}

	if m.introActive(m.now) {
		totalDur := 1.4
		elapsed := totalDur - m.introUntil.Sub(m.now).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		if elapsed > totalDur {
			elapsed = totalDur
		}
		p := elapsed / totalDur
		if p < 0.52 {
			scanP := p / 0.52
			head := triWave(scanP) * float64(max(0, total-2))
			start := int(math.Round(head))
			for i := start; i < min(total, start+2); i++ {
				pos := 0.0
				if total > 1 {
					pos = float64(i) / float64(total-1)
				}
				c := blendHexColor(sampleGradientColor(palette, pos), "#ffffff", 0.28)
				topRunes[i] = '▀'
				bottomRunes[i] = '▄'
				colors[i] = c
			}
			return renderRunesWithColors(topRunes, colors), renderRunesWithColors(bottomRunes, colors)
		}

		revealP := (p - 0.52) / 0.48
		if revealP < 0 {
			revealP = 0
		}
		if revealP > 1 {
			revealP = 1
		}
		reveal := int(math.Round(revealP * float64(total)))
		for i := 0; i < reveal; i++ {
			pos := 0.0
			if total > 1 {
				pos = float64(i) / float64(total-1)
			}
			glow := 0.5 + 0.5*math.Sin((revealP*math.Pi)+pos*2.4)
			c := blendHexColor(sampleGradientColor(palette, pos), "#ffffff", 0.16*glow)
			topRunes[i] = '▀'
			bottomRunes[i] = '▄'
			colors[i] = c
		}
		return renderRunesWithColors(topRunes, colors), renderRunesWithColors(bottomRunes, colors)
	}

	for i := 0; i < total; i++ {
		pos := 0.0
		if total > 1 {
			pos = float64(i) / float64(total-1)
		}
		pulse := 0.5 + 0.5*math.Sin(m.clockPhase*3.2+pos*4.8)
		colorPos := math.Mod(pos*0.92-m.clockPhase*0.16+pulse*0.08, 1.0)
		topRunes[i] = '▀'
		bottomRunes[i] = '▄'
		colors[i] = sampleGradientColor(palette, colorPos)
	}
	return renderRunesWithColors(topRunes, colors), renderRunesWithColors(bottomRunes, colors)
}

func (m model) systemPanelTitle() string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.themeRoleColor("primary", "#89b4fa"))).
		Render("System Info")
}

func (m model) clockMeridiemLabel() string {
	token := strings.ToUpper(strings.TrimSpace(m.now.Format("PM")))
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.themeRoleColor("secondary", "#94e2d5"))).
		Render(token)
}

func (m model) systemMetricSummary(used, total, pct float64) string {
	if total <= 0 {
		return fmt.Sprintf("%d%%", int(clampPct(pct)+0.5))
	}
	return fmt.Sprintf("%.2f GB / %.2f GB (%d%%)", used, total, int(clampPct(pct)+0.5))
}

func (m model) renderSystemMetricBar(width int, percent float64, text string, gradientFn func(float64) string) string {
	if width < 1 {
		width = 1
	}
	barW := width
	p := clampPct(percent)
	fill := int(float64(barW) * (p / 100.0))
	if p > 0 && fill == 0 {
		fill = 1
	}
	if fill > barW {
		fill = barW
	}
	emptyBG := blendHexColor(m.themeRoleColor("outline", "#565f89"), m.themeRoleColor("surface", "#1f2335"), 0.18)
	text = truncateText(text, max(1, barW-2))
	textRunes := []rune(text)
	textStart := 0
	if len(textRunes) < barW {
		textStart = (barW - len(textRunes)) / 2
	}

	var b strings.Builder
	for i := 0; i < barW; i++ {
		cellBG := emptyBG
		filled := false
		if i < fill {
			t := 0.0
			if barW > 1 {
				t = float64(i) / float64(barW-1)
			}
			cellBG = gradientFn(t)
			filled = true
		}

		textIdx := i - textStart
		if textIdx >= 0 && textIdx < len(textRunes) {
			fg := ensureReadableTextColor(cellBG, "#f5f5f8", "#111111")
			if filled {
				fg = ensureReadableTextColor(cellBG, "#111111", "#f5f5f8")
				b.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color(fg)).
					Background(lipgloss.Color(cellBG)).
					Bold(true).
					Render(string(textRunes[textIdx])))
			} else {
				b.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color(fg)).
					Background(lipgloss.Color(cellBG)).
					Bold(true).
					Render(string(textRunes[textIdx])))
			}
			continue
		}

		b.WriteString(lipgloss.NewStyle().
			Background(lipgloss.Color(cellBG)).
			Render(" "))
	}
	return b.String() + ansiColorSeq(m.themeRoleColor("primary", "#7aa2f7"))
}
