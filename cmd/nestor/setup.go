package main

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/internal/nx"
)

func setupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: shortDesc["setup"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, _ []string) error {
			dir, err := cmd.Flags().GetString("dir")
			if err != nil {
				return err
			}
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}

			displayed := dir
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}

				displayed = nx.CutHome(cwd, api.Runtime().Platform.UserHomeDir())
				question := fmt.Sprintf("Set up a nestor workspace in the current directory %s?", pterm.Blue(displayed))
				useCWD, err := pterm.DefaultInteractiveConfirm.WithDefaultText(question).WithDefaultValue(true).Show()
				if err != nil {
					return err
				}

				if !useCWD {
					fmt.Println(pterm.Yellow("nothing to do"))
					fmt.Println(pterm.Green("done"))
					return nil
				}

				dir = cwd
			}

			return cli.SetupWorkspace(api, cli.SetupWorkspaceArgs{
				Path:          dir,
				DisplayedPath: displayed,
				DryRun:        dryRun,
			})
		}),
	}
	cmd.Flags().String("dir", "", "dir want to setup")
	cmd.Flags().Bool("dry-run", false, "dry run")
	return cmd
}
