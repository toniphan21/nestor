package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type TabHome struct{}

func (t TabHome) Init() tea.Cmd {
	return nil
}

func (t TabHome) Update(tea.Msg) (TabHome, tea.Cmd) {
	return t, nil
}

func (t TabHome) View() string {
	sb := strings.Builder{}
	for i := range 200 {
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t TabHome) busy() bool {
	return false
}
