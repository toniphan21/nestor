package cli

import (
	"context"
	"fmt"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type releaseInfo struct {
	spec string
	path string
	run  bool
}

func collectReleaseInfo(api nestor.API, args []string) (*releaseInfo, error) {
	allSpecs := api.Runtime().Registry.SandboxSpecs()
	if len(allSpecs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to release"))
		return &releaseInfo{run: false}, nil
	}
	slices.SortFunc(allSpecs, func(a nestor.SandboxSpec, b nestor.SandboxSpec) int {
		return strings.Compare(a.Name, b.Name)
	})

	var err error
	var spec nestor.SandboxSpec
	if len(allSpecs) > 1 {
		spec, err = SelectSandboxSpec(allSpecs)
		if err != nil {
			return nil, err
		}
	} else {
		spec = allSpecs[0]
		fmt.Printf("chose sandbox spec %s - harness %s - profile %s\n", pterm.Green(spec.Name), pterm.Magenta(spec.Harness), pterm.Red(spec.Profile))
	}

	var paths []string
	for _, v := range spec.Mounts {
		paths = append(paths, v.Path)
	}

	if len(paths) == 0 {
		fmt.Println(pterm.Red("no sandbox mounts found"))
		return &releaseInfo{run: false}, nil
	}

	var selectedPath string
	if len(paths) > 1 {
		r, err := Select(paths, "select path", func(i int, s string) string {
			return fmt.Sprintf("%d. %s", i+1, s)
		})
		if err != nil {
			return nil, err
		}
		selectedPath = r.Value
	} else {
		selectedPath = paths[0]
		fmt.Printf("chose path %s\n", pterm.Cyan(selectedPath))
	}

	return &releaseInfo{spec: spec.Name, path: selectedPath, run: true}, nil
}

func Release(api nestor.API, args []string) error {
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

	return DoRelease(ctx, api, info.spec, info.path)
}

func DoRelease(ctx context.Context, api nestor.API, spec, path string) error {
	sandboxes, err := api.ListSandboxes(ctx, spec)
	if err != nil {
		return err
	}

	for _, sandbox := range sandboxes {
		leases := sandbox.Leases()
		for _, lease := range leases {
			if lease.HostPath() != path {
				continue
			}

			if err = lease.Release(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
