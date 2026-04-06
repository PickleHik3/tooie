package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

func loadPinnedApps() []string { return nil }

func (m model) homeRedrawCmd(refreshMetrics bool) tea.Cmd {
	cmds := []tea.Cmd{
		tea.ClearScreen,
		loadSystemInfoCmd(),
	}
	if refreshMetrics && !m.metricsPaused {
		cmds = append(cmds, pollMetrics())
	}
	return tea.Batch(cmds...)
}

func (m model) updateHomePage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "r":
		m.lastStatus = "Redrawing..."
		return m, m.homeRedrawCmd(true)
	}
	return m, nil
}
