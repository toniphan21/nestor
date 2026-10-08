// Package tui ...
package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"nhatp.com/go/nestor"
)

type TUI struct {
	header header
	body   body

	pages map[tabID]page

	width  int
	height int

	initCmd tea.Cmd
}

type page interface {
	Busy() bool
	Focus() tea.Cmd
	Blur()
	SetSize(w, h int)
	Update(tea.Msg) (page, tea.Cmd)
	View() string
}

func (t TUI) Init() tea.Cmd {
	return t.initCmd
}

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.header.SetWidth(t.width)

		hh := t.header.Height()
		t.body.SetSize(t.width, t.height-hh)
		for _, v := range t.pages {
			v.SetSize(t.width, t.height-hh)
		}
		return t, nil

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		t.body, cmd = t.body.Update(msg)
		return t, cmd

	case tea.KeyPressMsg:
		var busy bool
		for _, v := range t.pages {
			if v.Busy() {
				busy = true
				break
			}
		}
		if busy {
			return t, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "ctrl+d":
			return t, tea.Quit

		case "h":
			return t, t.switchPage(tabHome)

		case "d":
			return t, t.switchPage(tabDocker)

		case "s":
			return t, t.switchPage(tabSpec)

		case "p":
			return t, t.switchPage(tabProfile)

		case "m":
			return t, t.switchPage(tabMCP)
		}
	}

	// everything else -> all children
	var cmds []tea.Cmd
	for k, v := range t.pages {
		var c tea.Cmd
		t.pages[k], c = v.Update(msg)
		cmds = append(cmds, c)

		if k == t.header.ActiveTab() {
			t.body.SetContent(v.View())
		}
	}

	return t, tea.Batch(cmds...)
}

func (t TUI) View() tea.View {
	if t.width == 0 {
		return tea.NewView("")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, t.header.View(), t.body.View())

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (t *TUI) switchPage(id tabID) tea.Cmd {
	if id == t.header.ActiveTab() {
		return nil
	}

	o := t.pages[t.header.ActiveTab()]
	if o != nil {
		o.Blur()
	}

	p := t.pages[id]
	if p == nil {
		return nil
	}
	t.header.SetActiveTab(id)
	cmd := p.Focus()

	t.body.SetContent(p.View())
	t.body.GotoTop()

	return cmd
}

func Run(api nestor.API) error {
	t := &TUI{
		pages: map[tabID]page{
			tabHome:   &homePage{basePage: &basePage{}},
			tabDocker: &dockerPage{basePage: &basePage{}, api: api},
		},
	}
	t.initCmd = t.pages[t.header.ActiveTab()].Focus()

	if _, err := tea.NewProgram(t).Run(); err != nil {
		return err
	}
	return nil
}
