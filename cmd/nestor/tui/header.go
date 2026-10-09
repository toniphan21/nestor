package tui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
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

func (h header) ActiveTab() tabID {
	return h.active
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

	self := fmt.Sprintf("%s v%s %s %s\n\n", nestor.BinaryName, nestor.Version, pterm.Gray(nestor.GoOS), pterm.Gray(nestor.GoArch))

	tabs := []string{
		h.tab("Home", 'h', h.active == tabHome),
		h.tab("Docker", 'd', h.active == tabDocker),
		h.tab("Spec", 's', h.active == tabSpec),
		h.tab("Profile", 'p', h.active == tabProfile),
		h.tab("MCP", 'm', h.active == tabMCP),
	}

	helpText := []string{
		// pterm.Blue("enter"),
		// " shell",
		// pterm.Gray(" • "),
		pterm.Blue("d"),
		" down",
		pterm.Gray(" • "),
		pterm.Blue("e"),
		" edit",
		pterm.Gray(" • "),
		pterm.Blue("b"),
		" build",
		pterm.Gray(" • "),
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
	gap := max(h.width-widthOf(left)-widthOf(right), 0)

	return self + left + strings.Repeat(" ", gap) + right + "\n"
}

func (h header) tab(text string, key rune, isActive bool) string {
	activeBase := pterm.NewStyle(pterm.BgBlue)
	activeHot := pterm.NewStyle(pterm.BgBlue, pterm.Bold, pterm.Underscore)
	inactiveBase := pterm.NewStyle(pterm.BgGray)
	inactiveHot := pterm.NewStyle(pterm.BgGray, pterm.Bold, pterm.Underscore)

	if isActive {
		return h.tabLabel(text, nil, activeBase, activeHot)
	}
	return h.tabLabel(text, new(key), inactiveBase, inactiveHot)
}

func (h header) tabLabel(text string, key *rune, base, hotkey *pterm.Style) string {
	runes := []rune(text)

	idx := -1
	if key != nil {
		for i, r := range runes {
			if unicode.ToLower(r) == unicode.ToLower(*key) {
				idx = i
				break
			}
		}
	}

	if idx < 0 {
		return base.Sprint(" " + text + " ")
	}

	return base.Sprint(" "+string(runes[:idx])) +
		hotkey.Sprint(string(runes[idx])) +
		base.Sprint(string(runes[idx+1:])+" ")
}

type actionBar struct{}

type action struct {
	keys     []string
	desc     string
	priority int
	order    int
}
