package main

import (
	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
)

func launch() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "launch",
		Short:   shortDesc["launch"],
		Aliases: []string{"open", "start"},
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}
			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}
			session, err := cmd.Flags().GetString("session")
			if err != nil {
				return err
			}

			return cli.Launch(api, cli.LaunchArgs{
				SandboxSpec: spec,
				Path:        path,
				Session:     session,
			})
		}),
	}

	flags := cmd.Flags()

	flags.StringP("spec", "s", "", "name of the sandbox spec")
	flags.StringP("path", "p", "", "path")
	flags.StringP("session", "r", "", "session id to resume")

	return withGlobalFlags(cmd)
}
