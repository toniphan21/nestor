package tui

import "strings"

type footer struct {
	margin  int
	message string
}

func (f *footer) SetMessage(msg string) {
	f.message = msg
}

func (f *footer) Height() int {
	return heightOf(f.View())
}

func (f *footer) View() string {
	m := max(f.margin, 0)

	var out strings.Builder
	out.WriteString(strings.Repeat("\n", m))
	out.WriteString(f.message)
	return out.String()
}

type setFooterMsg struct {
	message string
}
