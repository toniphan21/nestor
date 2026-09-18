package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func Rename(api nestor.API) error {
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

	selected := r.Value

	v, err := pterm.DefaultInteractiveTextInput.WithDefaultText("New name").Show()
	if err != nil {
		return nil
	}
	newName := strings.TrimSpace(v)
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

	fmt.Println(pterm.Green("done"))
	return nil
}
