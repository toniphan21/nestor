package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"

	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/infra/fs"
)

type editorDoneMsg struct {
	tab     tabID
	tag     string
	path    string
	content string
	err     error
}

type execDoneMsg struct {
	tab tabID
	tag string
	err error
}

type initPromptDoneMsg struct {
	tab  tabID
	tag  string
	data cli.PromptDryRunData
	err  error
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

type altScreenMod func(*altScreenCmd)

func execCommand(tab tabID, tag string, cmd string, args ...string) tea.Cmd {
	return tea.Exec(altScreenCmd{cmd: exec.Command(cmd, args...)}, func(err error) tea.Msg {
		return execDoneMsg{tab: tab, tag: tag, err: err}
	})
}

func gotoDir(tab tabID, shell, dir, banner string) tea.Cmd {
	cmd := exec.Command(shell)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), nestor.EnvVarNestorShell+"="+shell)

	return tea.Exec(altScreenCmd{cmd: cmd, banner: banner}, func(err error) tea.Msg {
		return execDoneMsg{tab: tab, tag: "goto-dir", err: err}
	})
}

func runExecSelfCommand(tab tabID, tag string, args []string, mod altScreenMod) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return execDoneMsg{tab: tab, tag: tag, err: err} }
	}

	self, err = filepath.EvalSymlinks(self) // handle installed via a symlink
	if err != nil {
		return func() tea.Msg { return execDoneMsg{tab: tab, tag: tag, err: err} }
	}

	asc := altScreenCmd{cmd: exec.Command(self, args...)}
	if mod != nil {
		mod(&asc)
	}
	return tea.Exec(asc, func(err error) tea.Msg {
		return execDoneMsg{tab: tab, tag: tag, err: err}
	})
}

func execSelfCommand(tab tabID, tag string, args ...string) tea.Cmd {
	return runExecSelfCommand(tab, tag, args, func(v *altScreenCmd) {
		v.wait = false
	})
}

func execSelfCommandDelayed(tab tabID, tag string, args ...string) tea.Cmd {
	return runExecSelfCommand(tab, tag, args, func(v *altScreenCmd) {
		v.wait = true
	})
}

// initPrompt perform first step of prompting - collect running information
func initPrompt(tab tabID, tag string, args ...string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return initPromptDoneMsg{tab: tab, tag: tag, err: err} }
	}

	self, err = filepath.EvalSymlinks(self) // handle installed via a symlink
	if err != nil {
		return func() tea.Msg { return initPromptDoneMsg{tab: tab, tag: tag, err: err} }
	}

	f, err := os.CreateTemp("", "nestor-prompt-dry-run-*.json")
	if err != nil {
		return func() tea.Msg { return initPromptDoneMsg{tab: tab, tag: tag, err: err} }
	}

	argv := slices.Clone(args)
	argv = append(argv, "--dry-run", f.Name())
	asc := altScreenCmd{
		cmd: exec.Command(self, argv...),
	}
	return tea.Exec(asc, func(err error) tea.Msg {
		defer func() {
			_ = fs.RemoveFile(f.Name())
		}()

		if err != nil {
			return initPromptDoneMsg{tab: tab, tag: tag, err: err}
		}

		var b []byte
		b, err = fs.AtomicReadFile(f.Name())
		if err != nil {
			return initPromptDoneMsg{tab: tab, tag: tag, err: err}
		}

		var pd cli.PromptDryRunData
		if err = json.Unmarshal(b, &pd); err != nil {
			return initPromptDoneMsg{tab: tab, tag: tag, err: err}
		}
		return initPromptDoneMsg{tab: tab, tag: tag, data: pd, err: err}
	})
}

func dispatchPrompt(tab tabID, tag string, data cli.PromptDryRunData) tea.Cmd {
	timestamp := strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	id := nestor.HashPath(data.Path, 5) + "-" + data.Sandbox + "-" + timestamp
	logPath := filepath.Join(os.TempDir(), "nestor-prompt-"+id+".log")

	metadata := map[string]string{
		"path":    data.Path,
		"sandbox": data.Sandbox,
	}

	cmd, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return jobStartFailedMsg{tab: tab, tag: tag, err: err} }
	}

	cmd, err = filepath.EvalSymlinks(cmd) // handle installed via a symlink
	if err != nil {
		return func() tea.Msg { return jobStartFailedMsg{tab: tab, tag: tag, err: err} }
	}

	args := []string{
		"prompt",
		"-s", data.Spec,
		"-p", data.Path,
		"-m", data.Model,
		"-n", data.Sandbox,
		"-r", data.SessionID,
		"-t", data.SessionTitle,
		"-f", data.PromptFile,
	}
	return startJob(tab, tag, id, logPath, cmd, metadata, args...)
}

func tailFile(tab tabID, tag string, file string) tea.Cmd {
	args := []string{"+F", file}
	return tea.Exec(altScreenCmd{cmd: exec.Command("less", args...)}, func(err error) tea.Msg {
		return execDoneMsg{tab: tab, tag: tag, err: err}
	})
}

const (
	enterAltScreen = "\x1b[?1049h"
	exitAltScreen  = "\x1b[?1049l"
	cursorHome     = "\x1b[H"
)

const minDisplay = 3 * time.Second

// altScreenCmd runs a command on its own alternate screen, so output is
// readable but never lands on the user's shell screen. When wait is true, it
// additionally pauses for a minimum display time (or a key press on error)
// before returning, for commands whose output would otherwise flash by too
// fast to read. When banner is non-empty, it's printed once the alt screen
// is entered, before the command runs.
type altScreenCmd struct {
	cmd           *exec.Cmd
	wait          bool
	pressToReturn bool
	banner        string
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
	if c.banner != "" {
		_, _ = fmt.Fprintln(out, c.banner)
	}
	defer func() {
		_, _ = fmt.Fprint(out, exitAltScreen)
	}()

	start := time.Now()
	err := c.cmd.Run()
	if err != nil {
		if c.wait || c.pressToReturn {
			_, _ = fmt.Fprintf(out, "\nerror: %v\npress any key to return…", err)
			waitKey(in, 0) // errors: wait until the user has read it
		}
		return err
	}

	if c.wait || c.pressToReturn {
		if remaining := minDisplay - time.Since(start); remaining > 0 || c.pressToReturn {
			_, _ = fmt.Fprint(out, "\npress any key to return")
			waitKey(in, remaining)
		}
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
