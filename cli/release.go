package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type releaseInfo struct {
	spec string
	path string
	run  bool
}

func collectReleaseInfo(api nestor.API, args ReleaseArgs) (*releaseInfo, error) {
	var spec nestor.SandboxSpec
	if v, err := collectSpec(api, args.SandboxSpec); err != nil || v == nil {
		return &releaseInfo{run: false}, err
	} else {
		spec = *v
	}

	var selectedPath string
	if v, err := collectPath(spec, args.Path); err != nil || v == nil {
		return &releaseInfo{run: false}, err
	} else {
		selectedPath = *v
	}
	return &releaseInfo{spec: spec.Name, path: selectedPath, run: true}, nil
}

type ReleaseArgs struct {
	SandboxSpec string
	Sandbox     string
	Path        string
}

func Release(api nestor.API, args ReleaseArgs) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	info, err := collectReleaseInfo(api, args)
	if err != nil {
		return err
	}
	if !info.run {
		fmt.Println(pterm.Green("done"))
		return nil
	}

	sandboxes, err := api.ListSandboxes(ctx, info.spec)
	if err != nil {
		return err
	}

	for _, sandbox := range sandboxes {
		leases := sandbox.Leases()
		for _, lease := range leases {
			if lease.RequestedPath() != info.path {
				continue
			}

			if err = lease.Release(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
