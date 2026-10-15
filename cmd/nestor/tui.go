package main

import (
	"strings"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
)

type tuiCmd struct {
	root *cobra.Command
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

	launchSpec := &cobra.Command{
		Use:   "launch-spec",
		Short: shortDesc["tui-launch-spec"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}

			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}
			return cli.LaunchSpec(api, cli.LaunchSpecArgs{
				Spec: strings.TrimSpace(spec),
				Path: strings.TrimSpace(path),
			})
		}),
	}
	launchSpec.Flags().StringP("spec", "s", "", "sandbox spec name")
	launchSpec.Flags().StringP("path", "p", "", "working path")

	launchSandbox := &cobra.Command{
		Use:   "launch-sandbox",
		Short: shortDesc["tui-launch-sandbox"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}

			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}

			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}

			return cli.LaunchSandbox(api, cli.LaunchSandboxArgs{
				Spec:    strings.TrimSpace(spec),
				Path:    strings.TrimSpace(path),
				Sandbox: strings.TrimSpace(name),
			})
		}),
	}
	launchSandbox.Flags().StringP("spec", "s", "", "sandbox spec name")
	launchSandbox.Flags().StringP("path", "p", "", "working path")
	launchSandbox.Flags().StringP("name", "n", "", "sandbox name")

	root.AddCommand(renameWorkspace, launchSpec, launchSandbox)
	return &tuiCmd{
		root: root,
	}
}
