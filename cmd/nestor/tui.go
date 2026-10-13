package main

import (
	"strings"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
)

type tuiCmd struct {
	root            *cobra.Command
	renameWorkspace *cobra.Command
	launch          *cobra.Command
	prompt          *cobra.Command
}

func newTUICmd() *tuiCmd {
	root := &cobra.Command{
		Use:    "tui",
		Short:  shortDesc["tui"],
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	renameWorkspace := &cobra.Command{
		Use:   "rename-workspace",
		Short: shortDesc["tui-rename-workspace"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}

			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}

			return cli.RenameWorkspace(api, cli.RenameWorkspaceArgs{
				WorkspacePath: strings.TrimSpace(path),
				NewName:       name,
			})
		}),
	}
	renameWorkspace.Flags().StringP("path", "p", "", "path of the workspace to rename")
	renameWorkspace.Flags().StringP("name", "n", "", "new name of the workspace")

	root.AddCommand(renameWorkspace)
	return &tuiCmd{
		root: root,
	}
}
