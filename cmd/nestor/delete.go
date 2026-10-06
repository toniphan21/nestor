package main

import (
	"strings"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
)

func delete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: shortDesc["delete"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			name, err := cmd.Flags().GetString("sandbox")
			if err != nil {
				return err
			}

			return cli.Delete(api, cli.DeleteArgs{
				Sandbox: strings.TrimSpace(name),
			})
		}),
	}
	cmd.Flags().StringP("sandbox", "s", "", "name of the sandbox to delete")
	return withGlobalFlags(cmd)
}
