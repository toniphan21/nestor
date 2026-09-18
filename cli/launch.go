package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type launchInfo struct {
	spec    string
	path    string
	sandbox string
	run     bool
}

func collectLaunchInfo(api nestor.API, args []string) (*launchInfo, error) {
	var spec nestor.SandboxSpec
	if v, err := collectSpec(api); err != nil || v == nil {
		return &launchInfo{run: false}, err
	} else {
		spec = *v
	}

	var selectedPath string
	if v, err := collectPath(spec); err != nil || v == nil {
		return &launchInfo{run: false}, err
	} else {
		selectedPath = *v
	}
	return &launchInfo{spec: spec.Name, path: selectedPath, run: true}, nil
}

func Launch(api nestor.API, args []string) error {
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

	selectedSessionID, title := selectSession(lease)

	if err = lease.Extend(ctx); err != nil {
		return fmt.Errorf("extend lease: %w", err)
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

	defer func() {
		cancel()
		wg.Wait()

		if rErr := lease.Release(context.WithoutCancel(ctx)); rErr != nil {
			err = errors.Join(err, fmt.Errorf("release lease: %w", rErr))
		} else if err == nil {
			fmt.Println(pterm.Green("done"))
		}
	}()

	_, err = lease.Run(ctx, nestor.Interactive{
		OnInit: func(proxy string) {
			if proxy == "" {
				fmt.Println("initializing")
			} else {
				fmt.Printf("open proxy %s\n", proxy)
				fmt.Println("initializing")
			}
		},
		OnSessionEstablished: func(sess string) {
			fmt.Printf("start session %s\n", sess)
		},
		SessionID: selectedSessionID,
		Title:     title,
	})
	return err
}

func selectSession(lease *nestor.Lease) (string, string) {
	sessions := lease.ListSessions()
	var selectedId, title string
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
			return selectedId, title
		}

		selectedId = r.Value.ID
	}

	if selectedId == "" {
		t := "Session title (optional — leave blank for an auto-generated name)"
		v, err := pterm.DefaultInteractiveTextInput.WithDefaultText(t).Show()
		if err != nil {
			return "", ""
		}
		title = v
	}
	return selectedId, title
}
