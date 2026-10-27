package tui

import tea "charm.land/bubbletea/v2"

var _ tea.Model = (Root{})

type Root struct {
	active         tea.Model
	size           tea.WindowSizeMsg
	provider       *APIProvider
	upgradeCommand string
}

func (r Root) Init() tea.Cmd {
	return r.active.Init()
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.size = msg

	case preflightDoneMsg:
		size := r.size
		r.active = newTUI(r.provider, r.upgradeCommand)

		// main model never saw the initial WindowSizeMsg, so replay it
		return r, tea.Batch(r.active.Init(), func() tea.Msg { return size })
	}

	var cmd tea.Cmd
	r.active, cmd = r.active.Update(msg)
	return r, cmd
}

func (r Root) View() tea.View {
	return r.active.View()
}

func Run(provider *APIProvider, upgradeCommand string) error {
	m := Root{
		active:         newPreflight(provider),
		provider:       provider,
		upgradeCommand: upgradeCommand,
	}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		return err
	}
	return nil
}
