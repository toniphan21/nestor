package tui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

type editorDoneMsg struct {
	tab     tabID
	tag     string
	path    string
	content string
	err     error
}

func openEditorWithInitial(tab tabID, tag, template, initial string) tea.Cmd {
	f, err := os.CreateTemp("", template)
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{tab: tab, tag: tag, err: err} }
	}
	path := f.Name()
	_, err = f.WriteString(initial)
	if err != nil {
		if rerr := os.Remove(path); rerr != nil {
			return func() tea.Msg { return editorDoneMsg{tab: tab, tag: tag, err: errors.Join(err, rerr)} }
		}
		return func() tea.Msg { return editorDoneMsg{tab: tab, tag: tag, err: err} }
	}

	if cerr := f.Close(); cerr != nil {
		return func() tea.Msg { return editorDoneMsg{tab: tab, tag: tag, err: cerr} }
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	cmd := exec.Command(editor, path)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer func() {
			if rerr := os.Remove(path); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}()

		if err != nil {
			return editorDoneMsg{tab: tab, tag: tag, err: err}
		}
		b, err := os.ReadFile(path)
		return editorDoneMsg{tab: tab, tag: tag, content: string(b), err: err}
	})
}

func openEditorWithPath(tab tabID, tag string, path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	cmd := exec.Command(editor, path)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return editorDoneMsg{tab: tab, tag: tag, path: path, err: err}
		}
		b, err := os.ReadFile(path)
		return editorDoneMsg{tab: tab, tag: tag, path: path, content: string(b), err: err}
	})
}

type execDoneMsg struct {
	tab tabID
	tag string
	err error
}

func execCmd(tab tabID, tag string, cmd *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return execDoneMsg{tab: tab, tag: tag, err: err}
	})
}

func execSelfCommand(tab tabID, tag string, args ...string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{tab: tab, tag: tag, err: err} }
	}

	self, err = filepath.EvalSymlinks(self) // handle installed via a symlink
	if err != nil {
		return func() tea.Msg { return editorDoneMsg{tab: tab, tag: tag, err: err} }
	}

	return execCmd(tab, tag, exec.Command(self, args...))
}
