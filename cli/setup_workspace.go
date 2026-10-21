package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type SetupWorkspaceArgs struct {
	Path          string
	DisplayedPath string
	DryRun        bool
}

func SetupWorkspace(api nestor.API, args SetupWorkspaceArgs) error {
	specs := api.Runtime().Registry.SandboxSpecs()

	if len(specs) == 0 {
		return setupFreshSpecAndWorkspace(api, args)
	}

	ws, have := hasWorkspace(api, args.Path)
	if have {
		seen := make(map[string]bool)
		for _, v := range ws.Mounts() {
			seen[v.Spec().Name] = true
		}

		var availableSpecs []nestor.SandboxSpec
		for _, v := range specs {
			if !seen[v.Name] {
				availableSpecs = append(availableSpecs, v)
			}
		}

		if len(availableSpecs) == 0 {
			fmt.Println(pterm.Yellow(fmt.Sprintf("Path %s is already mounted to all sandboxes, nothing to do", args.DisplayedPath)))
			fmt.Println(pterm.Green("done"))
			return nil
		}

		fmt.Println("setup to specs", availableSpecs)
	}
	fmt.Println("setup to all specs", specs)
	return nil
}

func setupFreshSpecAndWorkspace(api nestor.API, args SetupWorkspaceArgs) error {
	return nil
}

func hasWorkspace(api nestor.API, path string) (nestor.Workspace, bool) {
	ctx := context.Background()

	workspaces := api.Workspaces(ctx)
	for _, v := range workspaces {
		if v.Path() == path {
			return v, true
		}
	}
	return nestor.Workspace{}, false
}
