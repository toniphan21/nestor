package tui

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/pterm/pterm"
)

type tabID int

const (
	tabHome tabID = iota
	tabDocker
	tabSpec
	tabProfile
	tabMCP
)

type header struct {
	active tabID
	width  int
}

func (h header) Height() int {
	return lipgloss.Height(h.View())
}

func (h *header) SetActiveTab(id tabID) {
	h.active = id
}

func (h *header) SetWidth(w int) {
	h.width = max(0, w)
}

func (h header) View() string {
	if h.width < 0 {
		return ""
	}

	tabs := []string{
		h.tab("Home", 'h', h.active == tabHome),
		h.tab("Docker", 'd', h.active == tabDocker),
		h.tab("Spec", 's', h.active == tabSpec),
		h.tab("Profile", 'p', h.active == tabProfile),
		h.tab("MCP", 'm', h.active == tabMCP),
	}

	helpText := []string{
		pterm.Blue("↑"), pterm.Gray("/"), pterm.Blue("k"),
		" up",
		pterm.Gray(" • "),
		pterm.Blue("↓"), pterm.Gray("/"), pterm.Blue("j"),
		" down",
		pterm.Gray(" • "),
		pterm.Red("q"),
		" quit ",
	}

	left := " " + strings.Join(tabs, " ")
	right := strings.Join(helpText, "")
	gap := max(h.width-lipgloss.Width(left)-lipgloss.Width(right), 0)

	return left + strings.Repeat(" ", gap) + right + "\n" + strings.Repeat(" ", h.width)
}

func (h header) tab(text string, key rune, isActive bool) string {
	activeBase := pterm.NewStyle(pterm.BgBlue)
	activeHot := pterm.NewStyle(pterm.BgBlue, pterm.Bold, pterm.Underscore)
	inactiveBase := pterm.NewStyle(pterm.BgGray)
	inactiveHot := pterm.NewStyle(pterm.BgGray, pterm.Bold, pterm.Underscore)

	if isActive {
		return h.tabLabel(text, key, activeBase, activeHot)
	}
	return h.tabLabel(text, key, inactiveBase, inactiveHot)
}

func (h header) tabLabel(text string, key rune, base, hotkey *pterm.Style) string {
	runes := []rune(text)

	idx := -1
	for i, r := range runes {
		if unicode.ToLower(r) == unicode.ToLower(key) {
			idx = i
			break
		}
	}

	if idx < 0 {
		return base.Sprint(" " + text + " ")
	}

	return base.Sprint(" "+string(runes[:idx])) +
		hotkey.Sprint(string(runes[idx])) +
		base.Sprint(string(runes[idx+1:])+" ")
}
