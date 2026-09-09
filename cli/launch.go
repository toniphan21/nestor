package cli

import (
	"context"
	"fmt"

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
		fmt.Println(pterm.Green("done"))
		return nil
	}

	ctx := context.Background()
	lease, err := api.Acquire(ctx, info.spec, info.path)
	if err != nil {
		return fmt.Errorf("acquire lease: %w", err)
	}
	if err = lease.Extend(ctx); err != nil {
		return fmt.Errorf("extend lease: %w", err)
	}

	var returnErr error
	defer func() {
		if err := lease.Release(ctx); err != nil {
			returnErr = fmt.Errorf("run lease: %w", err)
			return
		}

		fmt.Println(pterm.Green("done"))
		returnErr = nil
	}()

	_, returnErr = lease.Run(ctx, nestor.Interactive{})
	return returnErr
}
