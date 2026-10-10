package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type RenameArgs struct {
	Sandbox string
	NewName string
}

func Rename(api nestor.API, args RenameArgs) error {
	ctx := context.Background()

	sandboxes, err := api.ListSandboxes(ctx)
	if err != nil {
		return nil
	}
	if len(sandboxes) == 0 {
		fmt.Println(pterm.Yellow("no sandbox to rename"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	seen := make(map[string]bool)

	var selected nestor.Sandbox
	if args.Sandbox != "" {
		for _, v := range sandboxes {
			if v.ID() != args.Sandbox {
				continue
			}
			selected = v
			break
		}
		if selected == nil {
			return fmt.Errorf("%w: sandbox %q", nestor.ErrNotFound, args.Sandbox)
		}
	} else {
		dt := fmt.Sprintf("Select sandbox (%d available)", len(sandboxes))
		r, err := Select(sandboxes, dt, func(i int, s nestor.Sandbox) string {
			seen[s.ID()] = true
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
		selected = r.Value
	}

	var newName string
	if args.NewName != "" {
		newName = args.NewName
	} else {
		v, err := pterm.DefaultInteractiveTextInput.WithDefaultText("New name").Show()
		if err != nil {
			return nil
		}
		newName = strings.TrimSpace(v)
	}

	if newName == selected.ID() {
		fmt.Println(pterm.Yellow("new name is the same as the old one, nothing to do"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	if seen[newName] {
		fmt.Println(pterm.Red(fmt.Sprintf("name %q is already taken by another sandbox", newName)))
		return nil
	}

	if err = api.RenameSandbox(ctx, selected, newName); err != nil {
		return err
	}

	fmt.Printf("rename sandbox %q to %q\n", selected.ID(), newName)

	fmt.Println(pterm.Green("done"))
	return nil
}
