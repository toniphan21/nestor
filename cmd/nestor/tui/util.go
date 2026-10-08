package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func pad(w int) string {
	return strings.Repeat(" ", max(0, w))
}

func widthOf(s string) int {
	return lipgloss.Width(s)
}

func heightOf(s string) int {
	return lipgloss.Height(s)
}
