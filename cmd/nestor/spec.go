package main

import (
	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/internal/nx"
)

type specCmd struct {
	root    *cobra.Command
	build   *cobra.Command
	down    *cobra.Command
	release *cobra.Command
}

func newSpecCmd() *specCmd {
	root := &cobra.Command{
		Use:   "spec",
		Short: shortDesc["spec"],
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	down := &cobra.Command{
		Use:   "down",
		Short: shortDesc["spec-down"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			names := nx.Dedup(argv)
			return cli.Down(api, cli.DownArgs{Sandboxes: names})
		}),
	}

	release := &cobra.Command{
		Use:   "release",
		Short: shortDesc["spec-release"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}
			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}
			return cli.Release(api, cli.ReleaseArgs{
				SandboxSpec: spec,
				Path:        path,
			})
		}),
	}

	release.Flags().StringP("spec", "s", "", "name of the sandbox spec")
	release.Flags().StringP("path", "p", "", "path")

	root.AddCommand(down, release)

	return &specCmd{
		root:    root,
		down:    withGlobalFlags(down),
		release: withGlobalFlags(release),
	}
}
