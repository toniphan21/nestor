package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type LaunchArgs struct {
	SandboxSpec string
	Path        string
	Sandbox     string
	Session     string
}

func Launch(api nestor.API, args LaunchArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	sandboxes, err := collectFreeSandboxes(api, spec, path, args.Sandbox)
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
	sandbox, sessionID, title, err := sc.collect()
	if err != nil {
		return err
	}
	return launch(api, sandbox, path, sessionID, title)
}

func launch(api nestor.API, sandbox nestor.Sandbox, path, sessionID, sessionTitle string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := api.Runtime().Logger
	lease, err := sandbox.Acquire(ctx, path)
	if err != nil {
		return fmt.Errorf("acquire lease: %w", err)
	}

	if !lease.Sandbox().IsRunning(ctx) {
		_ = lease.Sandbox().Start(ctx)
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
				log.Debug("extend lease in interactive mode", slog.String("lease", lease.ID()))
				if err := lease.Extend(ctx); err != nil {
					if !errors.Is(err, context.Canceled) {
						log.Warn("cannot extend lease while running harness", slog.Any("error", err))
					}
				}
			}
		}
	})

	var runningSessionID string
	if sessionID != "" {
		runningSessionID = sessionID
	}

	defer func() {
		cancel()
		wg.Wait()

		if rErr := lease.Release(context.WithoutCancel(ctx)); rErr != nil {
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

	if err != nil {
		return err
	}

	if err = lease.Extend(ctx); err != nil {
		return fmt.Errorf("extend lease: %w", err)
	}

	lease.OnAuthProxyStarted(func(addr string) { fmt.Printf("opened auth proxy %s\n", addr) })
	lease.OnAuthProxyStopped(func(addr string) { fmt.Printf("closed auth proxy %s\n", addr) })
	lease.OnProxyStarted(func(addr string) { fmt.Printf("opened proxy %s\n", addr) })
	lease.OnProxyStopped(func(addr string) { fmt.Printf("closed proxy %s\n", addr) })

	_, err = lease.Run(ctx, nestor.Interactive{
		OnInit: func() {
			fmt.Println("initializing")
		},
		OnSessionEstablished: func(sess string) {
			if sess == "" {
				return
			}

			runningSessionID = sess
			fmt.Printf("started session %s\n", sess)
		},
		SessionID: sessionID,
		Title:     sessionTitle,
	})
	return err
}
