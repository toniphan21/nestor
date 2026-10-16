package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli/claude"
	"nhatp.com/go/nestor/cli/opencode"
	"nhatp.com/go/nestor/infra/fs"
)

type PromptArgs struct {
	SandboxSpec string
	Path        string
	Sandbox     string
	Session     string
	Model       string
	DryRun      bool
}

func Prompt(api nestor.API, args PromptArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	model, err := collectModel(api, spec, "", args.Model)
	if err != nil {
		return err
	}

	var sandboxes []nestor.Sandbox
	if args.Sandbox != "" {
		sandboxes, err = collectFreeSandboxes(api, spec, path, args.Sandbox)
	} else {
		sandboxes, err = freeSandboxes(api, spec, path)
	}

	if err != nil {
		return err
	}

	sc := sessionCollector{
		sandboxes:          sandboxes,
		path:               path,
		displaySandboxName: true,
		timeout:            30 * time.Second,
		filtered:           args.Session,
		selectLabel:        "Select session",
		titleInputLabel:    "Session title (optional — leave blank for an auto-generated name)",
	}
	sandbox, sessionID, sessionTitle, err := sc.collect()
	if err != nil {
		return err
	}

	content, err := collectPromptFromEditor(sandbox, path)
	if err != nil {
		return err
	}

	if args.DryRun {
		return promptDryRun(sandbox, path, model, sessionID, sessionTitle, content)
	}
	return prompt(api, sandbox, path, model, sessionID, sessionTitle, content)
}

type PromptDryRunData struct {
	Spec         string `json:"spec"`
	Path         string `json:"path"`
	Model        string `json:"model"`
	Sandbox      string `json:"sandbox"`
	SessionID    string `json:"sessionId"`
	SessionTitle string `json:"sessionTitle"`
	PromptFile   string `json:"promptFile"`
}

func promptDryRun(sandbox nestor.Sandbox, path, model, sessionID, sessionTitle, prompt string) error {
	f, err := os.CreateTemp("", "nestor-prompt-*.md")
	if err != nil {
		return err
	}
	fp := f.Name()
	if err = fs.WriteFile(fp, []byte(prompt), 0o644); err != nil {
		return err
	}

	data := PromptDryRunData{
		Spec:         sandbox.Spec().Name,
		Path:         path,
		Model:        model,
		Sandbox:      sandbox.ID(),
		SessionID:    sessionID,
		SessionTitle: sessionTitle,
		PromptFile:   fp,
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

func prompt(api nestor.API, sandbox nestor.Sandbox, path, model, sessionID, sessionTitle, prompt string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log := api.Runtime().Logger
	lease, err := sandbox.Acquire(ctx, path)
	if err != nil {
		return fmt.Errorf("acquire lease: %w", err)
	}
	if !lease.Sandbox().IsRunning(ctx) {
		_ = lease.Sandbox().Start(ctx)
	}

	var stdout io.Writer
	switch lease.Sandbox().Harness().Name() {
	case string(nestor.HarnessClaudeCode):
		stdout = claude.NewWriter()
	case string(nestor.HarnessOpenCode):
		stdout = opencode.NewWriter()
	default:
		stdout = os.Stdout
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return

			case <-t.C:
				log.Debug("extend lease in headless mode", slog.String("lease", lease.ID()))
				if err := lease.Extend(ctx); err != nil {
					if !errors.Is(err, context.Canceled) {
						log.Warn("cannot extend lease while running harness", slog.Any("error", err))
					}
				}
			}
		}
	})

	runningSessionID := sessionID

	defer func() {
		cancel()
		wg.Wait()

		if rErr := lease.Release(ctx); err != nil {
			err = errors.Join(err, fmt.Errorf("release lease: %w", rErr))
		} else if err == nil {
			argv := []string{
				"--spec=" + escapeCLIFlagValue(sandbox.Spec().Name),
				"--path=" + escapeCLIFlagValue(path),
				"--name=" + escapeCLIFlagValue(sandbox.ID()),
				"--session=" + escapeCLIFlagValue(runningSessionID),
			}

			cmd := "  nestor launch " + strings.Join(argv, " ")

			fmt.Printf("\nTo resume this session:\n\n%s\n\n", pterm.Blue(cmd))

			fmt.Println(pterm.Green("done"))
		}
	}()

	if err = lease.Extend(ctx); err != nil {
		return fmt.Errorf("extend lease: %w", err)
	}

	lease.OnAuthProxyStarted(func(addr string) { fmt.Printf("opened auth proxy %s\n", addr) })
	lease.OnAuthProxyStopped(func(addr string) { fmt.Printf("closed auth proxy %s\n", addr) })
	lease.OnProxyStarted(func(addr string) { fmt.Printf("opened proxy %s\n", addr) })
	lease.OnProxyStopped(func(addr string) { fmt.Printf("closed proxy %s\n", addr) })

	_, err = lease.Run(ctx, nestor.Headless{
		Prompt:    prompt,
		SessionID: sessionID,
		Title:     sessionTitle,
		Model:     model,
		Stdout:    stdout,
		OnFinished: func(sess string) {
			runningSessionID = sess
		},
	})
	return err
}
