package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
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
	active    tabID
	actionBar actionBar
	width     int
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

func (h *header) SetActions(actions []action) {
	h.actionBar = actionBar{actions: actions}
}

func (h header) View() string {
	if h.width < 0 {
		return ""
	}

	self := fmt.Sprintf("%s v%s %s %s\n\n", nestor.BinaryName, nestor.Version, pterm.Gray(nestor.GoOS), pterm.Gray(nestor.GoArch))

	tabs := []string{
		" ",
		h.tab("Home", 'h', h.active == tabHome),
		h.tab("Docker", 'd', h.active == tabDocker),
		h.tab("Spec", 's', h.active == tabSpec),
		h.tab("Profile", 'p', h.active == tabProfile),
		h.tab("MCP", 'm', h.active == tabMCP),
	}

	left := strings.Join(tabs, " ")

	width := h.width - widthOf(left) - 1
	h.actionBar.SetWidth(width)

	return self + left + fitLeft(h.actionBar.View(), width) + "\n"
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

type action struct {
	keys         []string
	desc         string
	priority     int
	order        int
	keySeparator string
}

type setActionsMsg struct {
	actions []action
}

var (
	actionQuit              = action{keys: []string{pterm.Red("q")}, desc: "quit", priority: -1}
	actionMove              = action{keys: []string{pterm.Blue("j"), pterm.Blue("k")}, desc: "move", priority: 0}
	actionMoveUp            = action{keys: []string{pterm.Blue("↑"), pterm.Blue("k")}, desc: "up", priority: 0}
	actionMoveDown          = action{keys: []string{pterm.Blue("↓"), pterm.Blue("j")}, desc: "down", priority: 0}
	actionEdit              = action{keys: []string{pterm.Blue("e")}, desc: "edit", priority: 1}
	actionEditDockerfile    = action{keys: []string{pterm.Blue("e")}, desc: "edit dockerfile", priority: 1}
	actionBuild             = action{keys: []string{pterm.Blue("b")}, desc: "build", priority: 1}
	actionContainerDown     = action{keys: []string{pterm.Blue("D")}, desc: "down", priority: 1}
	actionContainerUp       = action{keys: []string{pterm.Blue("u")}, desc: "up", priority: 1}
	actionContainerShell    = action{keys: []string{pterm.Blue("enter")}, desc: "shell", priority: 1}
	actionViewSecret        = action{keys: []string{pterm.Blue("enter")}, desc: "view", priority: 1}
	actionHideSecret        = action{keys: []string{pterm.Blue("enter")}, desc: "hide", priority: 1}
	actionTool              = action{keys: []string{pterm.Blue("t")}, desc: "tools", priority: 1}
	actionEditConfiguration = action{keys: []string{pterm.Blue("e")}, desc: "edit configuration", priority: 1}
)

func useActions(actions ...action) tea.Cmd {
	var out []action
	for i, v := range actions {
		v.order = i
		v.keySeparator = pterm.Gray("/")
		out = append(out, v)
	}
	return send(setActionsMsg{actions: out})
}

type actionBar struct {
	width     int
	actions   []action
	separator string
}

func (b *actionBar) SetWidth(w int) {
	b.width = max(0, w)
}

func (b *actionBar) View() string {
	if b.width <= 0 || len(b.actions) == 0 {
		return ""
	}

	sep := b.separator
	if sep == "" {
		sep = pterm.Gray(" • ")
	}

	rendered := make([]string, len(b.actions))
	visible := make([]bool, len(b.actions))
	for i, a := range b.actions {
		rendered[i] = b.RenderAction(a)
		visible[i] = true
	}

	totalWidth := func() int {
		w := 0
		first := true
		for i := range b.actions {
			if !visible[i] {
				continue
			}
			if !first {
				w += widthOf(sep)
			}
			first = false
			w += widthOf(rendered[i])
		}
		return w
	}

	// Hide lowest-priority actions first; within the same priority, hide
	// the ones with the lowest order (i.e. the leftmost ones) first, since
	// the bar is right-aligned.
	hideOrder := make([]int, len(b.actions))
	for i := range hideOrder {
		hideOrder[i] = i
	}
	sort.Slice(hideOrder, func(i, j int) bool {
		ai, aj := b.actions[hideOrder[i]], b.actions[hideOrder[j]]
		if ai.priority != aj.priority {
			return ai.priority < aj.priority
		}
		return ai.order < aj.order
	})

	for _, idx := range hideOrder {
		if totalWidth() <= b.width {
			break
		}
		visible[idx] = false
	}

	displayOrder := make([]int, 0, len(b.actions))
	for i := range b.actions {
		if visible[i] {
			displayOrder = append(displayOrder, i)
		}
	}
	sort.Slice(displayOrder, func(i, j int) bool {
		return b.actions[displayOrder[i]].order < b.actions[displayOrder[j]].order
	})

	parts := make([]string, len(displayOrder))
	for i, idx := range displayOrder {
		parts[i] = rendered[idx]
	}
	return strings.Join(parts, sep)
}

func (b *actionBar) RenderAction(a action) string {
	kp := a.keySeparator
	if kp == "" {
		kp = "/"
	}
	return strings.Join(a.keys, kp) + " " + a.desc
}
