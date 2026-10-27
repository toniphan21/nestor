package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type preflightDoneMsg struct{}

type preflightTickMsg struct{}

type preflightDataMsg struct {
	apiErr error
	done   bool
}

func newPreflight(provider *APIProvider) tea.Model {
	p := Preflight{
		header:   header{onlyBanner: true},
		provider: provider,
	}

	return p
}

var _ tea.Model = (Preflight{})

type Preflight struct {
	header   header
	body     body
	provider *APIProvider
	width    int
	height   int

	apiErr error
}

func (p Preflight) check() tea.Cmd {
	return func() tea.Msg {
		_, err := p.provider.Get()
		if err != nil {
			return preflightDataMsg{
				apiErr: err,
			}
		}
		return preflightDataMsg{}
	}
}

func (p Preflight) tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return preflightTickMsg{}
	})
}

func (p Preflight) done() tea.Cmd {
	return func() tea.Msg {
		return preflightDoneMsg{}
	}
}

func (p Preflight) Init() tea.Cmd {
	return p.check()
}

func (p Preflight) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case preflightTickMsg:
		return p, p.check()

	case preflightDataMsg:
		if msg.done {
			return p, p.done()
		}
		if msg.apiErr != nil {
			p.body.SetContent(msg.apiErr.Error())
		}

		return p, p.tick()

	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		p.header.SetWidth(p.width)

		hh := p.header.Height()
		bh := p.height - hh

		p.body.SetSize(p.width, bh)
		return p, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "ctrl+d":
			return p, tea.Quit
		}
	}
	return p, nil
}

func (p Preflight) View() tea.View {
	if p.width == 0 {
		return tea.NewView("")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, p.header.View(), p.body.View())

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}
