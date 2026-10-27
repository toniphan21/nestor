// Package tui ...
package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"nhatp.com/go/nestor"
)

const cronInterval = 5 * time.Minute

type cronTickMsg struct{}

type cronDataMsg struct {
	lastestVersion string
}

func newTUI(provider *APIProvider, upgradeCommand string) tea.Model {
	t := &TUI{
		footer: footer{margin: 1},
		pages: map[tabID]page{
			tabHome:    &homePage{basePage: &basePage{}, provider: provider},
			tabDocker:  &dockerPage{basePage: &basePage{}, provider: provider},
			tabSpec:    &specPage{basePage: &basePage{}, provider: provider},
			tabProfile: &profilePage{basePage: &basePage{}, provider: provider},
			tabMCP:     &mcpPage{basePage: &basePage{}, provider: provider},
		},
	}
	t.header.upgradeCommand = upgradeCommand
	t.initCmd = tea.Batch(t.pages[t.header.ActiveTab()].Focus(), t.cronJob())

	return t
}

var _ tea.Model = (TUI{})

type TUI struct {
	header header
	body   body
	footer footer

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

func (t TUI) cronTick() tea.Cmd {
	return tea.Tick(cronInterval, func(time.Time) tea.Msg {
		return cronTickMsg{}
	})
}

func (t TUI) cronJob() tea.Cmd {
	return func() tea.Msg {
		if nestor.Version == "dev" {
			return cronDataMsg{}
		}
		latestVersion, isNew := nestor.CheckUpdate(nestor.LatestReleaseURL)
		if !isNew {
			latestVersion = ""
		}
		return cronDataMsg{latestVersion}
	}
}

func (t TUI) Init() tea.Cmd {
	return t.initCmd
}

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	passMsgToAllChildren := func(cmd ...tea.Cmd) (tea.Model, tea.Cmd) {
		cmds := append([]tea.Cmd(nil), cmd...)
		for k, v := range t.pages {
			var c tea.Cmd
			t.pages[k], c = v.Update(msg)
			cmds = append(cmds, c)

			if k == t.header.ActiveTab() {
				t.body.SetContent(t.pages[k].View())
			}
		}
		return t, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
		t.header.SetWidth(t.width)

		hh := t.header.Height()
		fh := t.footer.Height()
		bh := t.height - hh - fh

		t.body.SetSize(t.width, bh)
		for _, v := range t.pages {
			v.SetSize(t.width-2, bh) // scroll take 2 on the right
		}
		return t, nil

	case setFooterMsg:
		t.footer.SetMessage(msg.message)
		hh := t.header.Height()
		fh := t.footer.Height()
		bh := t.height - hh - fh

		t.body.SetSize(t.width, bh)
		for _, v := range t.pages {
			v.SetSize(t.width-2, bh) // scroll take 2 on the right
		}
		return t, nil

	case cronTickMsg:
		return t, t.cronJob()

	case cronDataMsg:
		if msg.lastestVersion != "" {
			t.header.SetLatestVersion(msg.lastestVersion)
		}
		return t, t.cronTick()

	case tea.MouseWheelMsg:
		if t.isBusy() {
			return t, nil
		}

		var cmd tea.Cmd
		t.body, cmd = t.body.Update(msg)
		return t, cmd

	case tea.MouseClickMsg:
		if t.isBusy() {
			return t, nil
		}

		switch m := msg.Mouse(); m.Button {
		case uv.MouseLeft:
			tab := t.header.TabPosition(m.X, m.Y)
			if tab != nil {
				return t, t.switchPage(*tab)
			}
		}
		return t, nil

	case tea.KeyPressMsg:
		if t.isBusy() {
			return t, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "ctrl+d":
			if !t.header.HasJobs() {
				return t, tea.Quit
			}
			t.header.SetJobsWarning(true)
			return t, nil

		case "H":
			if t.header.ActiveTab() != tabHome {
				return t, t.switchPage(tabHome)
			}

		case "D":
			if t.header.ActiveTab() != tabDocker {
				return t, t.switchPage(tabDocker)
			}

		case "S":
			return t, t.switchPage(tabSpec)

		case "P":
			return t, t.switchPage(tabProfile)

		case "M":
			return t, t.switchPage(tabMCP)
		}

	case setActionsMsg:
		t.header.SetActions(msg.actions)
		return t, nil

	case jobStartedMsg:
		t.header.SetJobStarted(msg.job)
		return passMsgToAllChildren(waitJob(msg.job))

	case jobExitedMsg:
		t.header.SetJobEnded(msg.id)
		t.header.SetJobsWarning(false)
		return passMsgToAllChildren()
	}

	// everything else -> all children
	return passMsgToAllChildren()
}

func (t TUI) View() tea.View {
	if t.width == 0 {
		return tea.NewView("")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, t.header.View(), t.body.View(), t.footer.View())

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (t *TUI) isBusy() bool {
	var busy bool
	for _, v := range t.pages {
		if v.Busy() {
			busy = true
			break
		}
	}
	return busy
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
	t.footer.SetMessage("")

	return cmd
}
