package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type RenameWorkspaceArgs struct {
	WorkspacePath string
	NewName       string
}

func RenameWorkspace(api nestor.API, args RenameWorkspaceArgs) error {
	ctx := context.Background()

	workspaces := api.Workspaces(ctx)
	if len(workspaces) == 0 {
		fmt.Println(pterm.Yellow("no workspace to rename"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	seen := make(map[string]bool)

	var selected *nestor.Workspace
	if args.WorkspacePath != "" {
		for _, v := range workspaces {
			if v.Path() != args.WorkspacePath {
				continue
			}
			selected = &v
			break
		}
		if selected == nil {
			return fmt.Errorf("%w: workspace %q", nestor.ErrNotFound, args.WorkspacePath)
		}
	} else {
		dt := fmt.Sprintf("Select workspace (%d available)", len(workspaces))
		r, err := Select(workspaces, dt, func(i int, w nestor.Workspace) string {
			seen[w.Name()] = true
			return fmt.Sprintf("%d. %s - %s", i+1, w.Name(), pterm.Magenta(w.Path()))
		})
		if err != nil {
			return err
		}
		selected = &r.Value
	}

	config := LoadConfig(api.Runtime().Platform)
	currentName := selected.Name()
	for _, v := range config.Workspaces {
		if v.Path != selected.Path() {
			seen[v.Name] = true
			continue
		}
		if v.Name == "" {
			continue
		}
		currentName = v.Name
		break
	}
	fmt.Printf("Workspace path: %s\n", pterm.Magenta(selected.Path()))
	fmt.Printf("  Current name: %s\n", pterm.Green(currentName))

	var newName string
	if args.NewName != "" {
		newName = args.NewName
	} else {
		v, err := pterm.DefaultInteractiveTextInput.WithDefaultText("      New name").Show()
		if err != nil {
			return nil
		}
		newName = strings.TrimSpace(v)
	}

	if newName == selected.Name() {
		fmt.Println(pterm.Yellow("new name is the same as the old one, nothing to do"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	if seen[newName] {
		fmt.Println(pterm.Red(fmt.Sprintf("name %q is already taken by another workspace", newName)))
		return nil
	}

	for i, v := range config.Workspaces {
		if v.Path != selected.Path() {
			continue
		}

		v.Name = newName
		config.Workspaces[i] = v
		break
	}

	if err := config.Save(api.Runtime().Platform); err != nil {
		return err
	}

	fmt.Printf("\nrename workspace %q to %q\n", currentName, newName)
	fmt.Println(pterm.Green("done"))

	return nil
}
