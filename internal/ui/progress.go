package ui

import (
	"github.com/charmbracelet/bubbles/progress"
)

// NewProgress returns a Bubble Tea progress bar model styled with the NASIJ
// gradient (cyan → violet).
func NewProgress(width int) progress.Model {
	p := progress.New(
		progress.WithGradient(string(ColorPrimary), string(ColorAccent)),
		progress.WithWidth(width),
	)
	return p
}
