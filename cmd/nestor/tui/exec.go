package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
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

	return tea.Exec(altScreenCmd{cmd: exec.Command(self, args...)}, func(err error) tea.Msg {
		return execDoneMsg{tab: tab, tag: tag, err: err}
	})
}

const (
	enterAltScreen = "\x1b[?1049h"
	exitAltScreen  = "\x1b[?1049l"
	cursorHome     = "\x1b[H"
)

const minDisplay = 2 * time.Second

// altScreenCmd runs a command on its own alternate screen and waits for
// enter before returning, so output is readable but never lands on the
// user's shell screen.
type altScreenCmd struct {
	cmd *exec.Cmd
}

func (c altScreenCmd) SetStdin(r io.Reader)  { c.cmd.Stdin = r }
func (c altScreenCmd) SetStdout(w io.Writer) { c.cmd.Stdout = w }
func (c altScreenCmd) SetStderr(w io.Writer) { c.cmd.Stderr = w }

func (c altScreenCmd) Run() error {
	out := c.cmd.Stdout
	if out == nil {
		out = os.Stdout
	}
	in := c.cmd.Stdin
	if in == nil {
		in = os.Stdin
	}

	_, _ = fmt.Fprint(out, enterAltScreen+cursorHome)
	defer func() {
		_, _ = fmt.Fprint(out, exitAltScreen)
	}()

	start := time.Now()
	err := c.cmd.Run()
	if err != nil {
		_, _ = fmt.Fprintf(out, "\nerror: %v\npress any key to return…", err)
		waitKey(in, 0) // errors: wait until the user has read it
		return err
	}

	if remaining := minDisplay - time.Since(start); remaining > 0 {
		_, _ = fmt.Fprint(out, "\npress any key to return")
		waitKey(in, remaining)
	}
	return nil
}

// waitKey returns on any key press or after d. d <= 0 waits for a key only.
func waitKey(in io.Reader, d time.Duration) {
	f, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		if d > 0 {
			time.Sleep(d)
		}
		return
	}

	fd := int(f.Fd())
	if state, err := term.MakeRaw(fd); err == nil {
		defer term.Restore(fd, state) // runs last
	}

	cr, err := cancelreader.NewReader(f)
	if err != nil {
		if d > 0 {
			time.Sleep(d)
		}
		return
	}
	defer cr.Close()

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		cr.Read(buf) // returns on a key, or with an error after Cancel
		close(done)
	}()

	var timeout <-chan time.Time // nil = no timeout
	if d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		timeout = t.C
	}

	select {
	case <-done:
	case <-timeout:
	}
	cr.Cancel()
	<-done // reader goroutine has exited before the terminal goes back
}
