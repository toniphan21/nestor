// Package tui ...
package tui

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type TUI struct {
	home homePage

	header header
	body   body

	width  int
	height int
}

func (t TUI) Init() tea.Cmd {
	return t.home.Init()
}

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.header.SetWidth(t.width)
		t.body.SetSize(t.width, t.height-t.header.Height())
		return t, nil

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		t.body, cmd = t.body.Update(msg)
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
			t.header.SetActiveTab(tabHome)
			t.home.Update(msg)
			t.body.SetContent(t.home.View())
			return t, nil

		case "d":
			t.header.SetActiveTab(tabDocker)
			return t, nil

		case "s":
			t.header.SetActiveTab(tabSpec)
			return t, nil

		case "p":
			t.header.SetActiveTab(tabProfile)
			return t, nil

		case "m":
			t.header.SetActiveTab(tabMCP)
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
		return tea.NewView("")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, t.header.View(), t.body.View())

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func Run() {
	t := &TUI{}

	if _, err := tea.NewProgram(t).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
