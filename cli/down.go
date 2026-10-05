package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type DownArgs struct {
	Sandboxes []string
}

func Down(api nestor.API, args DownArgs) error {
	ctx := context.Background()

	sandboxes, err := api.ListSandboxes(ctx)
	if err != nil {
		return err
	}

	if len(sandboxes) == 0 {
		fmt.Println(pterm.Yellow("no sandbox to stop"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	allows := make(map[string]bool)
	if len(args.Sandboxes) > 0 {
		for _, v := range args.Sandboxes {
			allows[v] = true
		}
	}

	for _, sandbox := range sandboxes {
		if len(allows) > 0 && !allows[sandbox.ID()] {
			fmt.Printf("sandbox %s is not in the list, skip\n", pterm.Blue(sandbox.ID()))
			continue
		}

		if sandbox.IsRunning(ctx) {
			fmt.Printf("stopping sandbox %s (container: %s)", pterm.Blue(sandbox.ID()), pterm.Cyan(sandbox.Container()))
			err = sandbox.Stop(ctx)
			if err != nil {
				return err
			}
			fmt.Printf(", stopped\n")
			continue
		}

		fmt.Printf("sandbox %s is not running, skip\n", pterm.Blue(sandbox.ID()))
	}
	fmt.Println(pterm.Green("done"))
	return nil
}
