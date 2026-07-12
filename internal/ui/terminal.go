package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

// flagSet is an interface satisfied by both *pflag.FlagSet.
type flagSet interface {
	VisitAll(fn func(*pflag.Flag))
}

func isBoolFlag(f *pflag.Flag) bool {
	_, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && f.Value.(interface{ IsBoolFlag() bool }).IsBoolFlag()
}

// Terminal provides high-level rendering helpers for CLI output.
// All output goes to a configurable io.Writer (defaults to os.Stdout).
type Terminal struct {
	out io.Writer
}

// NewTerminal creates a Terminal that writes to w.
func NewTerminal(w io.Writer) *Terminal {
	return &Terminal{out: w}
}

// Default returns a Terminal writing to os.Stdout.
func Default() *Terminal {
	return NewTerminal(os.Stdout)
}

// --- Output helpers ---

// Success prints a success-styled line prefixed with a checkmark.
func (t *Terminal) Success(msg string) {
	fmt.Fprintln(t.out, "  "+IconCheck+"  "+StyleSuccess.Render(msg))
}

// Error prints an error-styled line prefixed with a cross.
func (t *Terminal) Error(msg string) {
	fmt.Fprintln(t.out, "  "+IconCross+"  "+StyleDanger.Render(msg))
}

// Warning prints a warning-styled line.
func (t *Terminal) Warning(msg string) {
	fmt.Fprintln(t.out, "  "+IconWarning+"  "+StyleWarning.Render(msg))
}

// Info prints a muted informational line with two-space indent.
func (t *Terminal) Info(msg string) {
	fmt.Fprintln(t.out, "  "+StyleMuted.Render(msg))
}

// Header prints a bold section header.
func (t *Terminal) Header(msg string) {
	fmt.Fprintln(t.out, StyleHeader.Render(msg))
}

// Subheader prints a secondary header.
func (t *Terminal) Subheader(msg string) {
	fmt.Fprintln(t.out, StyleSubheader.Render(msg))
}

// KeyValue prints a key: value pair with the key padded to a fixed width.
func (t *Terminal) KeyValue(key, value string) {
	k := StyleMuted.Render(padRight(key, 20))
	v := StyleWhite.Render(value)
	fmt.Fprintln(t.out, "  "+k+"  "+v)
}

// StatusRow prints a doctor-style status row.
//
//	✔  Label                    detail text
//	✘  Label                    error message (styled red)
func (t *Terminal) StatusRow(ok bool, label, detail string) {
	icon := IconCheck
	if !ok {
		icon = IconCross
	}
	labelStr := StyleMuted.Render(padRight(label, 26))
	var detailStr string
	if ok {
		detailStr = StyleWhite.Render(detail)
	} else {
		detailStr = StyleDanger.Render(detail)
	}
	fmt.Fprintln(t.out, "  "+icon+"  "+labelStr+detailStr)
}

// Divider prints a horizontal rule.
func (t *Terminal) Divider() {
	fmt.Fprintln(t.out, "  "+StyleDim.Render(strings.Repeat("─", 58)))
}

// Blank prints a blank line.
func (t *Terminal) Blank() {
	fmt.Fprintln(t.out)
}

// Println prints a plain line to the terminal output.
func (t *Terminal) Println(msg string) {
	fmt.Fprintln(t.out, msg)
}

// Box renders msg inside a rounded border box and prints it.
func (t *Terminal) Box(title, content string) {
	inner := StyleSubheader.Render(title) + "\n" + content
	fmt.Fprintln(t.out, StyleHighlightBox.Render(inner))
}

// Table prints a two-column key/value table.
func (t *Terminal) Table(rows [][2]string) {
	if len(rows) == 0 {
		return
	}
	maxKey := 0
	for _, r := range rows {
		if len(r[0]) > maxKey {
			maxKey = len(r[0])
		}
	}
	for _, r := range rows {
		k := lipgloss.NewStyle().Foreground(ColorMuted).Render(padRight(r[0], maxKey+2))
		fmt.Fprintln(t.out, "  "+k+r[1])
	}
}

// --- writer access ---

// Writer returns the underlying io.Writer for this terminal.
func (t *Terminal) Writer() io.Writer { return t.out }

// PrintFlagUsages renders a pflag.FlagSet to w, styled with muted colors.
func PrintFlagUsages(w io.Writer, fs flagSet, indent string) {
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		var line strings.Builder
		line.WriteString(indent)
		if f.Shorthand != "" {
			line.WriteString(StyleAccent.Render("-" + f.Shorthand + ", "))
		} else {
			line.WriteString(strings.Repeat(" ", 4))
		}
		line.WriteString(StyleAccent.Render("--" + f.Name))
		if isBoolFlag(f) {
			fmt.Fprintln(w, line.String())
			return
		}
		typ := strings.ToUpper(f.Value.Type())
		line.WriteString(StyleDim.Render(" " + typ))
		def := f.DefValue
		if def != "" {
			line.WriteString(StyleMuted.Render("  (default " + def + ")"))
		}
		fmt.Fprintln(w, line.String())

		desc := StyleMuted.Render(indent + strings.Repeat(" ", 10) + f.Usage)
		fmt.Fprintln(w, desc)
	})
}

// --- helpers ---

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
