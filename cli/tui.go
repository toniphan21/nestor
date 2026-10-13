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

type LaunchSandboxArgs struct {
	Spec    string
	Sandbox string
	Path    string
}

func LaunchSandbox(api nestor.API, args LaunchSandboxArgs) error {
	spec, path, sandbox, err := validateTUISandboxArgs(api, args.Spec, args.Path, args.Sandbox)
	if err != nil {
		return err
	}

	w := 25
	label := func(txt string) string {
		return fmt.Sprintf("%*s", w, txt)
	}

	fmt.Printf(
		"%s: %s - harness %s - profile %s\n",
		label("Spec"), pterm.Blue(spec.Name), pterm.Blue(sandbox.Harness().Name()), pterm.Blue(sandbox.Profile().Name),
	)
	fmt.Printf("%s: %s\n", label("Sandbox"), pterm.Green(sandbox.ID()))
	fmt.Printf("%s: %s\n", label("Path"), pterm.Magenta(path))
	wd, hwd, _ := sandbox.ResolveWorkDir(path)
	if wd != hwd {
		fmt.Printf("%s: %s\n", label("Container Path"), sandbox.ID())
		fmt.Printf("%s: %s\n", label("Host Work Path"), hwd)
	} else {
		fmt.Printf("%s: %s\n", label("Work Path"), hwd)
	}

	sessionID, title := selectSandboxSession(spec, path, sandbox, label)

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

	defer func() {
		cancel()
		wg.Wait()

		if rErr := lease.Release(context.WithoutCancel(ctx)); rErr != nil {
			err = errors.Join(err, fmt.Errorf("release lease: %w", rErr))
		}
	}()

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
			fmt.Printf("started session %s\n", sess)
		},
		SessionID: sessionID,
		Title:     title,
	})
	return err
}

func selectSandboxSession(spec nestor.SandboxSpec, path string, sandbox nestor.Sandbox, label func(string) string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessions := sandbox.Harness().ListSessions(ctx, path, sandbox)
	var selectedID, title string

	if len(sessions) == 0 {
		t := label("Session Title (optional)")
		v, err := pterm.DefaultInteractiveTextInput.WithDefaultText(t).Show()
		if err != nil {
			return "", ""
		}
		title = v
	} else {
		sessions = slices.Insert(sessions, 0, nestor.HarnessSession{
			Title: "New session without title auto-generated name",
		})
		sessions = slices.Insert(sessions, 0, nestor.HarnessSession{
			Title: "New session",
		})

		printer := pterm.DefaultInteractiveSelect.
			WithDefaultText(" Session").
			WithMaxHeight(len(sessions) + 2)

		r, err := SingleSelect(sessions, printer, func(i int, s nestor.HarnessSession) string {
			if i == 0 || i == 1 {
				return fmt.Sprintf("%d. %s", i+1, s.Title)
			}

			t := s.Title
			if t == "" {
				t = "last opened: " + s.UpdatedAt.Local().Format("2006-01-02 15:04")
			}
			return fmt.Sprintf("%d. %s - %s", i+2, s.ID, t)
		})
		if err != nil {
			return selectedID, title
		}

		if r.Index == 0 {
			t := label("Session title (optional)")
			v, err := pterm.DefaultInteractiveTextInput.WithDefaultText(t).Show()
			if err != nil {
				return "", ""
			}
			title = v
			return "", title
		}

		if r.Index == 1 {
			return "", ""
		}

		return r.Value.ID, ""
	}
	return selectedID, title
}

func validateTUISandboxArgs(api nestor.API, specName string, path string, sandboxName string) (nestor.SandboxSpec, string, nestor.Sandbox, error) {
	spec, have := api.Runtime().Registry.SandboxSpec(specName)
	if !have {
		return spec, "", nil, fmt.Errorf("%w: sandbox spec %q", nestor.ErrNotFound, specName)
	}

	var selectedPath string
	for _, v := range spec.Paths() {
		if v != path {
			continue
		}
		selectedPath = v
		break
	}
	if selectedPath == "" {
		return spec, selectedPath, nil, fmt.Errorf("%w: path %q", nestor.ErrNotFound, path)
	}

	var sandbox nestor.Sandbox
	sandboxes, err := spec.ListSandboxes(api.Runtime())
	if err != nil {
		return spec, selectedPath, nil, err
	}
	for _, v := range sandboxes {
		if v.ID() != sandboxName {
			continue
		}
		sandbox = v
		break
	}
	if sandbox == nil {
		return spec, selectedPath, nil, fmt.Errorf("%w: sandbox %q", nestor.ErrNotFound, sandboxName)
	}

	if !sandbox.IsFree(path) {
		return spec, selectedPath, nil, fmt.Errorf("%w: path %q in sandbox %q", nestor.ErrNotAvailable, path, sandbox.ID())
	}
	return spec, selectedPath, sandbox, nil
}
