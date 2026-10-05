package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type DeleteArgs struct {
	SandboxName string
}

func Delete(api nestor.API, params DeleteArgs) error {
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

	var selected nestor.Sandbox

	if params.SandboxName != "" {
		for _, v := range sandboxes {
			if v.ID() != params.SandboxName {
				continue
			}
			selected = v
		}
	} else {
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
			selected = r.Value
		}
	}

	if selected == nil {
		fmt.Println(pterm.Yellow("no sandbox to delete"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	if err = selected.Stop(ctx); err != nil {
		return err
	}
	if err = selected.Delete(ctx); err != nil {
		return err
	}
	fmt.Println(pterm.Green(fmt.Sprintf("deleted sandbox %s. done", selected.ID())))
	return nil
}
