// Display helpers: simple ANSI-coloured banners + table rendering.
package utils

import (
	"fmt"
	"strings"
)

const (
	colorReset  = "\x1b[0m"
	colorBold   = "\x1b[1m"
	colorDim    = "\x1b[2m"
	colorRed    = "\x1b[31m"
	colorGreen  = "\x1b[32m"
	colorYellow = "\x1b[33m"
	colorBlue   = "\x1b[34m"
	colorCyan   = "\x1b[36m"
)

// Header prints a bold cyan title with a divider line.
func Header(title string) {
	fmt.Println()
	fmt.Println(colorCyan + colorBold + title + colorReset)
	fmt.Println(colorDim + strings.Repeat("─", len(title)) + colorReset)
}

// Success prints a green success line.
func Success(msg string) { fmt.Println(colorGreen + "✓ " + msg + colorReset) }

// Warn prints a yellow warning line.
func Warn(msg string) { fmt.Println(colorYellow + "⚠ " + msg + colorReset) }

// Error prints a red error line.
func Error(msg string) { fmt.Println(colorRed + "✗ " + msg + colorReset) }

// Info prints a blue informational line.
func Info(msg string) { fmt.Println(colorBlue + msg + colorReset) }

// Table renders a simple ASCII table. cols is the header row; rows is N rows
// matching cols length. Columns auto-size to widest entry per column.
func Table(cols []string, rows [][]string) {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	for _, r := range rows {
		for i, cell := range r {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	render := func(cells []string, bold bool) {
		parts := make([]string, len(cells))
		for i, c := range cells {
			parts[i] = padRight(c, widths[i])
		}
		line := strings.Join(parts, "  ")
		if bold {
			line = colorBold + line + colorReset
		}
		fmt.Println(line)
	}
	render(cols, true)
	sep := make([]string, len(cols))
	for i := range cols {
		sep[i] = strings.Repeat("─", widths[i])
	}
	fmt.Println(colorDim + strings.Join(sep, "  ") + colorReset)
	for _, r := range rows {
		render(r, false)
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
