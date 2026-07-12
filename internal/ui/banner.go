package ui

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nasij/nasij/pkg/version"
)

//go:embed logo.txt
var logoFile string

var markupTag = regexp.MustCompile(`<[^>]*>|\[/?\w+[^\]]*\]`)

func asciiLogo() string {
	s := markupTag.ReplaceAllString(logoFile, "")
	s = strings.TrimLeft(s, "\n")
	return s
}

func PrintBanner() {
	art := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Render(asciiLogo())

	tagline := StyleMuted.Render("  Intelligent JavaScript Reconnaissance Framework")

	versionLine := "  " + lipgloss.JoinHorizontal(lipgloss.Left,
		StyleAccent.Render("v"+version.Version),
		StyleMuted.Render("  "+IconDot+"  "),
		StyleMuted.Render(version.Platform()),
		StyleMuted.Render("  "+IconDot+"  "),
		StyleMuted.Render(version.GoVersion()),
	)

	fmt.Println(art)
	fmt.Println(tagline)
	fmt.Println(versionLine)
	fmt.Println()
}
