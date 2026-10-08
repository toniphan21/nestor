package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pterm/pterm"
)

type body struct {
	vp viewport.Model
}

func (b *body) SetSize(width, height int) {
	b.vp.SetWidth(max(0, width-1)) // scrollbar
	b.vp.SetHeight(max(0, height))
}

func (b *body) Update(msg tea.Msg) (body, tea.Cmd) {
	var cmd tea.Cmd
	b.vp, cmd = b.vp.Update(msg)

	return *b, cmd
}

func (b *body) SetContent(content string) {
	b.vp.SetContent(content)
}

func (b body) View() string {
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		b.vp.View(),
		b.scrollbar(b.vp),
	)
	return body
}

func (b body) scrollbar(vp viewport.Model) string {
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
