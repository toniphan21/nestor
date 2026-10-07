// Package tui ...
package tui

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pterm/pterm"
)

type tab int

const (
	tabHome tab = iota
	tabSpec
	tabProfile
	tabMCP
)

type TUI struct {
	tab  tab
	home TabHome

	width  int
	height int
	vp     viewport.Model
}

func (t TUI) Init() tea.Cmd {
	return t.home.Init()
}

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		bodyH := max(0, t.height-lipgloss.Height(t.tabline()))
		bodyW := max(0, t.width-1)

		t.vp.SetHeight(bodyH)
		t.vp.SetWidth(bodyW)
		return t, nil

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		t.vp, cmd = t.vp.Update(msg)
		return t, cmd

	case tea.KeyPressMsg:
		busy := t.home.busy()
		if busy {
			return t, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "ctrl+d":
			return t, tea.Quit

		case "h":
			t.tab = tabHome
			t.home.Update(msg)
			t.vp.SetContent(t.home.View())
			return t, nil

		case "s":
			t.tab = tabSpec
			return t, nil

		case "p":
			t.tab = tabProfile
			return t, nil

		case "m":
			t.tab = tabMCP
			return t, nil

		}
	}

	// everything else -> all children
	var c1 tea.Cmd
	t.home, c1 = t.home.Update(msg)

	return t, tea.Batch(c1)
}

func (t TUI) View() tea.View {
	if t.width == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		t.vp.View(),
		t.scrollbar(t.vp),
	)
	content := lipgloss.JoinVertical(lipgloss.Left, t.tabline(), body)

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (t TUI) tabline() string {
	tabs := []string{
		tabLabel("Home", 'h', inactiveBase, inactiveHot),
		tabLabel("Spec", 's', inactiveBase, inactiveHot),
		tabLabel("Profile", 'p', inactiveBase, inactiveHot),
		tabLabel("MCP", 'm', inactiveBase, inactiveHot),
	}
	switch t.tab {
	case tabHome:
		tabs[0] = tabLabel("Home", 'h', activeBase, activeHot)
	case tabSpec:
		tabs[1] = tabLabel("Spec", 's', activeBase, activeHot)
	case tabProfile:
		tabs[2] = tabLabel("Profile", 'p', activeBase, activeHot)
	case tabMCP:
		tabs[3] = tabLabel("MCP", 'm', activeBase, activeHot)
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
	gap := t.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right + "\n" + strings.Repeat(" ", t.width)
}

func (t TUI) scrollbar(vp viewport.Model) string {
	h, total := vp.Height(), vp.TotalLineCount()
	if h <= 0 {
		return ""
	}
	lines := make([]string, h)
	if total <= h { // everything fits: blank column
		for i := range lines {
			lines[i] = " "
		}
		return strings.Join(lines, "\n")
	}
	thumb := max(1, h*h/total)
	pos := int(vp.ScrollPercent() * float64(h-thumb))
	for i := range lines {
		if i >= pos && i < pos+thumb {
			lines[i] = pterm.White("│")
		} else {
			lines[i] = pterm.Gray("│")
		}
	}
	return strings.Join(lines, "\n")
}

func Run() {
	t := &TUI{}

	if _, err := tea.NewProgram(t).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	activeBase   = pterm.NewStyle(pterm.BgBlue)
	activeHot    = pterm.NewStyle(pterm.BgBlue, pterm.Bold, pterm.Underscore)
	inactiveBase = pterm.NewStyle(pterm.BgGray)
	inactiveHot  = pterm.NewStyle(pterm.BgGray, pterm.Bold, pterm.Underscore)
)

// tabLabel renders " Home " with the hotkey letter in its own style.
// Each segment is styled separately so the background never breaks.
func tabLabel(title string, key rune, base, hot *pterm.Style) string {
	runes := []rune(title)
	idx := -1
	for i, r := range runes {
		if unicode.ToLower(r) == unicode.ToLower(key) {
			idx = i
			break
		}
	}
	if idx < 0 { // key not in title: show it as a prefix
		return base.Sprint(" ") + hot.Sprint(string(key)) + base.Sprint(" "+title+" ")
	}
	return base.Sprint(" "+string(runes[:idx])) +
		hot.Sprint(string(runes[idx])) +
		base.Sprint(string(runes[idx+1:])+" ")
}
