package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func Delete(api nestor.API) error {
	ctx := context.Background()

	sandboxes, err := api.ListSandboxes(ctx)
	if err != nil {
		return nil
	}
	if len(sandboxes) == 0 {
		fmt.Println(pterm.Yellow("no sandbox to delete"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	dt := fmt.Sprintf("Select sandbox (%d available)", len(sandboxes))
	r, err := Select(sandboxes, dt, func(i int, s nestor.Sandbox) string {
		return fmt.Sprintf(
			"%d. %s - spec %s - harness %s - profile %s",
			i+1,
			s.ID(),
			pterm.Cyan(s.Spec().Name),
			pterm.Magenta(s.Harness().Name()),
			pterm.Red(s.Profile().Name),
		)
	})
	if err != nil {
		return err
	}

	msg := "Delete sandbox %s and its worktree and state?"
	confirmed, err := pterm.DefaultInteractiveConfirm.WithDefaultText(msg).Show()
	if err != nil {
		return err
	}

	if confirmed {
		if err = r.Value.Stop(ctx); err != nil {
			return err
		}
		if err = r.Value.Delete(ctx); err != nil {
			return err
		}
	}
	fmt.Println(pterm.Green(fmt.Sprintf("deleted sandbox %s. done", r.Value.ID())))
	return nil
}
