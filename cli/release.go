package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type ReleaseArgs struct {
	SandboxSpec string
	Sandbox     string
	Path        string
}

func Release(api nestor.API, args ReleaseArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sandboxes, err := api.ListSandboxes(ctx, spec.Name)
	if err != nil {
		return err
	}

	for _, sandbox := range sandboxes {
		leases := sandbox.Leases()
		for _, lease := range leases {
			if lease.RequestedPath() != path {
				continue
			}

			if err = lease.Release(ctx); err != nil {
				return err
			}
		}
	}

	fmt.Println(pterm.Green("done"))
	return nil
}
