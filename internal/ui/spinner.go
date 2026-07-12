package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// NewSpinner returns a Bubble Tea spinner model styled with the NASIJ palette.
// The spinner uses the "dot" style (⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷).
func NewSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)
	return s
}
