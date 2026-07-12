package cli

import (
	"strings"

	"github.com/nasij/nasij/internal/ui"
)

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func renderProgressBar(pct float64, width int) string {
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	if pct < 1.0 {
		return ui.StyleAccent.Render(bar)
	}
	return ui.StyleSuccess.Render(bar)
}
