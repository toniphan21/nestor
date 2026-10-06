package main

import (
	"strings"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
)

func rename() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename",
		Short: shortDesc["rename"],
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

	cmd.Flags().StringP("sandbox", "s", "", "name of the sandbox to rename")
	cmd.Flags().StringP("name", "n", "", "new name of the sandbox")
	return withGlobalFlags(cmd)
}
