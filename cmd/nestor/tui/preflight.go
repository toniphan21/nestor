package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
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

	busy              bool
	apiErr            error
	invalidConfigFile string
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
		if p.busy {
			return p, nil
		}

		return p, p.check()

	case preflightDataMsg:
		if msg.done {
			return p, p.done()
		}
		if msg.apiErr != nil {
			p.apiErr = msg.apiErr
			if pe, ok := errors.AsType[*nestor.ParseError](msg.apiErr); ok {
				p.invalidConfigFile = pe.File
			}
			return p, p.tick()
		}

		return p, p.done()

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

		case "e":
			if p.invalidConfigFile == "" {
				return p, nil
			}

			content, err := fs.AtomicReadFile(p.invalidConfigFile)
			if err != nil {
				return p, nil
			}
			p.busy = true
			return p, openEditorWithInitial(0, "edit-invalid-config", "*.yml", string(content))
		}

	case editorDoneMsg:
		p.busy = false
		if p.invalidConfigFile == "" {
			return p, nil
		}

		if msg.tag != "edit-invalid-config" {
			return p, nil
		}
		_ = fs.AtomicWriteFile(p.invalidConfigFile, []byte(msg.content), 0o644)

		p.apiErr = nil
		p.invalidConfigFile = ""
		return p, p.check()

	}
	return p, nil
}

func (p Preflight) View() tea.View {
	if p.width == 0 {
		return tea.NewView("")
	}

	var bc strings.Builder
	if p.apiErr != nil {
		bc.WriteString(p.apiErr.Error())
	}
	if p.invalidConfigFile != "" {
		fmt.Fprintf(&bc, "\npress %s to edit the config file", pterm.Blue("e"))
	}

	p.body.SetContent(bc.String())
	content := lipgloss.JoinVertical(lipgloss.Left, p.header.View(), p.body.View())

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}
