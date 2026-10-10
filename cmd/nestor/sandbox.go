package main

import (
	"strings"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/internal/nx"
)

type sandboxCmd struct {
	root   *cobra.Command
	delete *cobra.Command
	down   *cobra.Command
	rename *cobra.Command
	up     *cobra.Command
}

func newSandboxCmd() *sandboxCmd {
	root := &cobra.Command{
		Use:   "sandbox",
		Short: shortDesc["sandbox"],
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	delete := &cobra.Command{
		Use:   "delete",
		Short: shortDesc["sandbox-delete"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			name, err := cmd.Flags().GetString("sandbox")
			if err != nil {
				return err
			}
			return cli.Delete(api, cli.DeleteArgs{Sandbox: strings.TrimSpace(name)})
		}),
	}
	delete.Flags().StringP("sandbox", "s", "", "name of the sandbox to delete")

	down := &cobra.Command{
		Use:   "down",
		Short: shortDesc["sandbox-down"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			names := nx.Dedup(argv)
			return cli.Down(api, cli.DownArgs{Sandboxes: names})
		}),
	}

	rename := &cobra.Command{
		Use:   "rename",
		Short: shortDesc["sandbox-rename"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			sandbox, err := cmd.Flags().GetString("sandbox")
			if err != nil {
				return err
			}

			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}

			return cli.Rename(api, cli.RenameArgs{
				Sandbox: strings.TrimSpace(sandbox),
				NewName: name,
			})
		}),
	}
	rename.Flags().StringP("sandbox", "s", "", "name of the sandbox to rename")
	rename.Flags().StringP("name", "n", "", "new name of the sandbox")

	up := &cobra.Command{
		Use:   "up",
		Short: shortDesc["sandbox-up"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			names := nx.Dedup(argv)
			return cli.Up(api, cli.UpArgs{Sandboxes: names})
		}),
	}

	root.AddCommand(delete, down, rename, up)

	return &sandboxCmd{
		root:   root,
		delete: withGlobalFlags(delete),
		down:   withGlobalFlags(down),
		rename: withGlobalFlags(rename),
		up:     withGlobalFlags(up),
	}
}
