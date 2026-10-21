package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func displayVersion(v string) string {
	if v == "dev" {
		return v
	}
	return "v" + v
}

func selectedRow(text string) string {
	return pterm.NewStyle(pterm.BgGray).Sprint(text)
}

var extraSep = pterm.Gray(" · ")

func mountTypeDisplay(t nestor.MountType) string {
	switch t {
	case nestor.MountTypeGitWorktree:
		return "worktree"
	case nestor.MountTypeDirect:
		return "direct"
	default:
		return string(t)
	}
}

var secretKeyPhrases = []string{
	"token", "secret", "api_key", "apikey", "authorization", "password", "auth",
}

func isSecretKey(key string) bool {
	key = strings.ToLower(key)
	for _, v := range secretKeyPhrases {
		if strings.Contains(key, v) {
			return true
		}
	}
	return false
}

func send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

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
