package tui

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
)

type jobState int

const (
	jobRunning jobState = iota
	jobSucceeded
	jobFailed
	jobCancelled
)

func (s jobState) String() string {
	switch s {
	case jobRunning:
		return "running"
	case jobSucceeded:
		return "done"
	case jobFailed:
		return "failed"
	case jobCancelled:
		return "cancelled"
	}
	return "unknown"
}

// job is a background process whose stdout and stderr go to one log file.
type job struct {
	tab      tabID
	tag      string
	id       string
	pid      int
	logPath  string
	metadata map[string]string
	state    jobState
	cmd      *exec.Cmd // nil once the process has been waited for
}

type (
	jobStartedMsg struct {
		tab tabID
		tag string
		job job
	}
	jobStartFailedMsg struct {
		tab tabID
		tag string
		id  string
		err error
	}
	jobExitedMsg struct {
		tab tabID
		tag string
		id  string
		err error
	}
	jobStopDeadlineMsg struct {
		tab tabID
		tag string
		id  string
	}
	jobSignalErrMsg struct {
		tab tabID
		tag string
		id  string
		err error
	}
)

// startJob starts name+args in its own process group, output appended to logPath.
func startJob(tab tabID, tag, id, logPath, cmd string, metadata map[string]string, args ...string) tea.Cmd {
	return func() tea.Msg {
		logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return jobStartFailedMsg{tab: tab, tag: tag, id: id, err: err}
		}
		defer func() {
			_ = logf.Close() // the child keeps its own copy of the fd
		}()

		cmd := exec.Command(cmd, args...)
		cmd.Stdout = logf
		cmd.Stderr = logf
		// Stdin nil → /dev/null
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own group: kill -pid hits all children

		if err := cmd.Start(); err != nil {
			return jobStartFailedMsg{tab: tab, tag: tag, id: id, err: err}
		}
		return jobStartedMsg{
			tab: tab,
			tag: tag,
			job: job{
				tab:      tab,
				tag:      tag,
				id:       id,
				pid:      cmd.Process.Pid,
				logPath:  logPath,
				metadata: metadata,
				state:    jobRunning,
				cmd:      cmd,
			},
		}
	}
}

// waitJob blocks until the job exits. Call it exactly once per started job.
func waitJob(j job) tea.Cmd {
	tab, tag, cmd, id := j.tab, j.tag, j.cmd, j.id
	return func() tea.Msg {
		return jobExitedMsg{tab: tab, tag: tag, id: id, err: cmd.Wait()}
	}
}

// stopJob sends SIGTERM to the job's process group and schedules a deadline.
// If the job is still running at the deadline, follow up with killJob.
func stopJob(j job, grace time.Duration) tea.Cmd {
	tab, tag, pid, id := j.tab, j.tag, j.pid, j.id
	return tea.Batch(
		signalCmd(tab, tag, id, pid, syscall.SIGTERM),
		tea.Tick(grace, func(time.Time) tea.Msg { return jobStopDeadlineMsg{tab: tab, tag: tag, id: id} }),
	)
}

// killJob sends SIGKILL to the job's process group.
func killJob(j job) tea.Cmd {
	return signalCmd(j.tab, j.tag, j.id, j.pid, syscall.SIGKILL)
}

func signalCmd(tab tabID, tag, id string, pid int, sig syscall.Signal) tea.Cmd {
	return func() tea.Msg {
		err := syscall.Kill(-pid, sig) // negative pid = whole process group
		if err == nil || errors.Is(err, syscall.ESRCH) {
			return nil // sent, or already gone
		}
		return jobSignalErrMsg{tab: tab, tag: tag, id: id, err: err}
	}
}
