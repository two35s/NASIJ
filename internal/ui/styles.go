// Package ui provides NASIJ's terminal rendering layer.
//
// All colours and styles are defined as package-level variables here —
// never inline colour codes in component files. This single source of truth
// ensures visual consistency across all commands.
package ui

import "github.com/charmbracelet/lipgloss"

// Colour palette — designed for dark terminal backgrounds.
var (
	ColorPrimary = lipgloss.Color("#00D9FF") // electric cyan  — primary brand colour
	ColorAccent  = lipgloss.Color("#A78BFA") // soft violet    — secondary accents
	ColorSuccess = lipgloss.Color("#39D353") // vivid green    — checks / ok states
	ColorWarning = lipgloss.Color("#F59E0B") // amber          — warnings
	ColorDanger  = lipgloss.Color("#FF4D6D") // red-pink       — errors / fails
	ColorMuted   = lipgloss.Color("#9CA3AF") // warm grey      — labels / metadata
	ColorDim     = lipgloss.Color("#374151") // dark grey      — borders / dividers
	ColorWhite   = lipgloss.Color("#F9FAFB") // near-white     — primary text
)

// Base text styles
var (
	StylePrimary = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	StyleAccent  = lipgloss.NewStyle().Foreground(ColorAccent)
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	StyleDanger  = lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleDim     = lipgloss.NewStyle().Foreground(ColorDim)
	StyleBold    = lipgloss.NewStyle().Bold(true)
	StyleWhite   = lipgloss.NewStyle().Foreground(ColorWhite)
)

// Layout styles
var (
	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1)

	StyleSubheader = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	StyleBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorDim).
			Padding(0, 2)

	StyleHighlightBox = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 2)
)

// Status indicator symbols (pre-rendered for performance)
var (
	IconCheck   = StyleSuccess.Render("✔")
	IconCross   = StyleDanger.Render("✘")
	IconWarning = StyleWarning.Render("!")
	IconArrow   = StyleAccent.Render("→")
	IconDot     = StyleMuted.Render("·")
)
