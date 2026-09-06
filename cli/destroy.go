package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

func Destroy(api nestor.API) error {
	text := fmt.Sprintf(
		"Do you want to stop all containers, and delete all sandbox directories in %s?",
		api.Runtime().Platform.NestorDir(),
	)
	result, err := pterm.DefaultInteractiveConfirm.WithDefaultText(text).Show()
	if err != nil {
		return err
	}

	if !result {
		fmt.Println("ok, do nothing")
		fmt.Println(pterm.Green("done"))
		return nil
	}

	return DoDestroy(context.Background(), api)
}

func DoDestroy(ctx context.Context, api nestor.API) error {
	sandboxes, err := api.ListSandboxes(ctx)
	if err != nil {
		return err
	}

	if len(sandboxes) == 0 {
		fmt.Println(pterm.Yellow("no sandbox to destroy"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	for _, sandbox := range sandboxes {
		if sandbox.IsRunning(ctx) {
			fmt.Printf("stopping sandbox %s (container: %s)", pterm.Blue(sandbox.ID()), pterm.Cyan(sandbox.Container()))
			err = sandbox.Stop(ctx)
			if err != nil {
				return err
			}
			fmt.Printf(", stopped\n")
		}

		if err := fs.RemoveDir(sandbox.Dir()); err != nil {
			return err
		}
		fmt.Printf("deleted sandbox %s\n", pterm.Blue(sandbox.ID()))
	}
	fmt.Println(pterm.Green("done"))
	return nil
}
