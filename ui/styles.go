package ui

import "github.com/charmbracelet/lipgloss"

var (
	styleHeader   = lipgloss.NewStyle().Bold(true)
	styleColHead  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	styleSelected = lipgloss.NewStyle().Reverse(true)
	styleRunning  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleStopped  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleErr      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	styleDim      = lipgloss.NewStyle().Faint(true)
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

func stateStyle(state string) lipgloss.Style {
	if state == "running" {
		return styleRunning
	}
	return styleStopped
}

func cpuStyle(pct float64) lipgloss.Style {
	switch {
	case pct >= 80:
		return styleErr
	case pct >= 50:
		return styleWarn
	default:
		return lipgloss.NewStyle()
	}
}
