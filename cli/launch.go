package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type launchInfo struct {
	spec string
	path string
	run  bool
}

func collectLaunchInfo(api nestor.API, args LaunchArgs) (*launchInfo, error) {
	var spec nestor.SandboxSpec
	if v, err := collectSpec(api, args.SandboxSpec); err != nil || v == nil {
		return nil, err
	} else {
		spec = *v
	}

	var selectedPath string
	if v, err := collectPath(spec, args.Path); err != nil || v == nil {
		return nil, err
	} else {
		selectedPath = *v
	}
	return &launchInfo{spec: spec.Name, path: selectedPath, run: true}, nil
}

type LaunchArgs struct {
	SandboxSpec string
	Path        string
	Session     string
}

func Launch(api nestor.API, args LaunchArgs) error {
	info, err := collectLaunchInfo(api, args)
	if err != nil {
		return err
	}
	if !info.run {
		fmt.Println(pterm.Yellow("nothing to launch"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := api.Runtime().Logger
	lease, err := api.Acquire(ctx, info.spec, info.path)
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
	defer func() {
		cancel()
		wg.Wait()

		if rErr := lease.Release(context.WithoutCancel(ctx)); rErr != nil {
			err = errors.Join(err, fmt.Errorf("release lease: %w", rErr))
		} else if err == nil {
			argv := []string{
				"--spec=" + escapeCLIFlagValue(info.spec),
				"--path=" + escapeCLIFlagValue(info.path),
				"--session=" + escapeCLIFlagValue(runningSessionID),
			}

			cmd := "  nestor launch " + strings.Join(argv, " ")

			fmt.Printf("\nTo resume this session:\n\n%s\n\n", pterm.Blue(cmd))

			fmt.Println(pterm.Green("done"))
		}
	}()

	selectedSessionID, title, err := selectSession(lease, args.Session)
	if err != nil {
		return err
	}
	if selectedSessionID != "" {
		runningSessionID = selectedSessionID
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
		SessionID: selectedSessionID,
		Title:     title,
	})
	return err
}

func selectSession(lease *nestor.Lease, filteredSession string) (string, string, error) {
	sessions := lease.ListSessions()
	var selectedID, title string

	if filteredSession != "" {
		for _, v := range sessions {
			if v.ID != filteredSession {
				continue
			}
			return v.ID, v.Title, nil
		}
		return "", "", fmt.Errorf("%w: session %q", nestor.ErrNotFound, filteredSession)
	}

	if len(sessions) > 0 {
		sessions = slices.Insert(sessions, 0, nestor.HarnessSession{
			Title: "New session",
		})
		dt := fmt.Sprintf("Select sessions (%d available)", len(sessions))
		r, err := Select(sessions, dt, func(i int, s nestor.HarnessSession) string {
			if i == 0 {
				return fmt.Sprintf("%d. %s", i+1, s.Title)
			}
			return fmt.Sprintf("%d. %s - %s", i+1, s.ID, s.Title)
		})
		if err != nil {
			return selectedID, title, nil
		}

		selectedID = r.Value.ID
	}

	if selectedID == "" {
		t := "Session title (optional — leave blank for an auto-generated name)"
		v, err := pterm.DefaultInteractiveTextInput.WithDefaultText(t).Show()
		if err != nil {
			return "", "", nil
		}
		title = v
	}
	return selectedID, title, nil
}
