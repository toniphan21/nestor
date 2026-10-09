package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func spaces(w int) string {
	return strings.Repeat(" ", max(0, w))
}

func widthOf(s string) int {
	return lipgloss.Width(s)
}

func heightOf(s string) int {
	return lipgloss.Height(s)
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-sw)
}

func fitLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return strings.Repeat(" ", w-sw) + s
}

func pad(s string, n int) string {
	if n <= 0 {
		return s
	}
	return s + strings.Repeat(" ", n)
}

func padLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	return strings.Repeat(" ", n) + s
}
